package core

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"
	"bytes"

	"github.com/ipfs/go-cid"
	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/ipfs/go-ipld-git"
	mh "github.com/multiformats/go-multihash"
	"github.com/remeh/sizedwaitgroup"
)

var ErrNotProvided = errors.New("block not provided")

type ObjectProvider func(cid string, tracker *Tracker) ([]byte, error)

type Fetch struct {
	objectDir string
	gitDir    string

	done  uint64
	todoc uint64

	todo    chan string
	log     *log.Logger
	tracker *Tracker

	errCh  chan error
	wg     sizedwaitgroup.SizedWaitGroup
	doneCh chan []byte

	shunt map[string]string
	fsLk *sync.Mutex

	provider ObjectProvider
	api      *ipfs.Shell
}

func NewFetch(gitDir string, tracker *Tracker, provider ObjectProvider) *Fetch {
	return &Fetch{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,

		log:     log.New(os.Stderr, "fetch: ", 0),
		tracker: tracker,

		fsLk: &sync.Mutex{},

		wg: sizedwaitgroup.New(512),

		//Note: logic below somewhat relies on these channels staying unbuffered
		todo:   make(chan string),
		errCh:  make(chan error),
		doneCh: make(chan []byte),

		provider: provider,
		api:      ipfs.NewLocalShell(),
	}
}

func (f *Fetch) FetchHash(base string, remote *Remote) error {
	go func() {
		f.todo <- base
	}()
	shunt, _ := f.api.List(remote.Handler.GetRemoteName() + "/blobs")
	f.shunt = make(map[string]string)
	for _, s := range shunt {
		f.shunt[s.Name] = s.Hash
	}
	f.log.Printf("Shunt! %s (%s)\n", f.shunt, remote.Handler.GetRemoteName() + "/blobs")
	return f.doWork()
}

func (f *Fetch) doWork() error {
	f.log.Println("Fetch#doWork")
	for {
		select {
		case err := <-f.errCh:
			return err
		case hash := <-f.todo:
			f.todoc++
			if err := f.processSingle(hash); err != nil {
				return err
			}
		case <-f.doneCh:
			f.done++
		}

		//f.log.Printf("%d/%d\r\x1b[A", f.done, f.todoc)
		f.log.Printf("%d/%d\n", f.done, f.todoc)

		if f.done == f.todoc {
			f.wg.Wait()
			f.log.Printf("\n")
			return nil
		}
	}
}

func (f *Fetch) processSingle(hash string) error {
	f.log.Println("Fetch#processSingle.hash == ", hash)
	mhash, err := mh.FromHexString("1114" + hash)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	c := cid.NewCidV1(cid.GitRaw, mhash).String()

	f.log.Println("Fetch#processSingle.cid == ", c)

	sha, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	has, err := f.tracker.HasEntry(sha)
	if err != nil {
		return err
	}
	if has {
		f.todoc--
		return nil
	}

	// Need to do this early
	if err := f.tracker.AddEntry(sha); err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	go func() {
		f.wg.Add()
		defer f.wg.Done()

		f.fsLk.Lock()
		objectPath, err := prepHashPath(f.objectDir, hash)
		f.fsLk.Unlock()
		if err != nil {
			f.errCh <- err
			return
		}

		f.log.Println("Fetch#prepHashPath == ", hash)

		object, err := f.provider(c, f.tracker)
		if err != nil {
			if err != ErrNotProvided {
				f.errCh <- err
				return
			}

			shunt, ok := f.shunt[c]
			f.log.Printf("Fetch#BlockGet == %s (%s)\n", c, shunt)
			if ok {
				f.log.Println("Fetch#BlockGet// special == ", c)
				r, _ := f.api.Cat(shunt)
				defer r.Close()
				buff := new(bytes.Buffer)
				buff.ReadFrom(r)
				out := buff.String()
				object = append([]byte(fmt.Sprintf("blob %d\x00", len(out))), out...)
			} else {
				object, err = f.api.BlockGet(c)
				if err != nil {
					f.errCh <- fmt.Errorf("fetch: %v", err)
					return
				}
			}
		}

		f.log.Println("Fetch#processLinks")

		f.processLinks(object)

		f.log.Println("Fetch#processedLinks")

		object = compressObject(object)

		f.log.Println("Fetch#writing: ", *objectPath)

		err = ioutil.WriteFile(*objectPath, object, 0444)
		if err != nil {
			f.errCh <- fmt.Errorf("fetch: %v", err)
			return
		}

		f.log.Println("Fetch#wrote: ", *objectPath)

		//TODO: see if moving this higher would help
		f.doneCh <- sha
	}()

	return nil
}

func (f *Fetch) processLinks(object []byte) error {
	f.log.Println("len(Fetch#processLinks.object) == ", len(object))
	nd, err := ipldgit.ParseObjectFromBuffer(object)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	links := nd.Links()
	for _, link := range links {
		mhash := link.Cid.Hash()
		hash := mhash.HexString()[4:]

		f.log.Println("Fetch#processLinks.hash == ", hash)

		f.todo <- hash
	}
	f.log.Println("End Fetch#processLinks")
	return nil
}

func prepHashPath(localDir string, hash string) (*string, error) {
	base := path.Join(localDir, hash[:2])
	err := os.MkdirAll(base, 0777)

	if err != nil {
		return nil, err
	}

	objectPath := path.Join(base, hash[2:])
	return &objectPath, nil
}
