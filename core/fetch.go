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

	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/ipfs/go-ipld-git"
	"github.com/remeh/sizedwaitgroup"
)

var ErrNotProvided = errors.New("block not provided")

type ObjectProvider func(cid string, tracker *Tracker) ([]byte, error)

type Object struct{
	objType string
	cid string
}

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

	seen   map[string]bool
	shunts map[string]Object
	fsLk   *sync.Mutex
	snLk   *sync.RWMutex
	api    *ipfs.Shell
}

func NewFetch(gitDir string, tracker *Tracker) *Fetch {
	return &Fetch{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,

		log:     log.New(os.Stderr, "\x1b[34mfetch:\x1b[39m ", 0),
		tracker: tracker,

		fsLk: &sync.Mutex{},
		snLk: &sync.RWMutex{},

		wg: sizedwaitgroup.New(512),

		//Note: logic below somewhat relies on these channels staying unbuffered
		todo:   make(chan string),
		errCh:  make(chan error),
		doneCh: make(chan []byte),
		
		seen:   make(map[string]bool),
		shunts: make(map[string]Object),
		api:    ipfs.NewLocalShell(),
	}
}

func (f *Fetch) FetchHash(base string, remote *Remote) error {
	go func() {
		f.todo <- base
	}()

	f.getGitObjects("blob", remote.Handler.GetRemoteName())
	f.getGitObjects("commit", remote.Handler.GetRemoteName())
	f.getGitObjects("tag", remote.Handler.GetRemoteName())
	f.getGitObjects("tree", remote.Handler.GetRemoteName())

	return f.doWork()
}

func (f *Fetch) getGitObjects(objType string, remoteName string) error {
	shunts, err := f.api.List(fmt.Sprintf("%s/.git/%ss", remoteName, objType))
	if err != nil {
		return err
	}
	for _, s := range shunts {
		f.shunts[s.Name] = Object{ objType, s.Hash }
	}
	return nil
}

func (f *Fetch) doWork() error {
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

		if f.done == f.todoc {
			f.log.Println()
			f.wg.Wait()
			return nil
		}
	}
}

func (f *Fetch) processSingle(hash string) error {
	sha, err := hex.DecodeString(hash)
	if err != nil {
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

		object := []byte(nil)

		shunt, ok := f.shunts[hash]
		if !ok {
			f.errCh <- fmt.Errorf("Fetch Missing Block!: %v", err)
			return
		}

		f.log.Printf("(%d/%d) %s as %s\r\x1b[A", f.done, f.todoc, hash, shunt.cid)

		r, _ := f.api.Cat(shunt.cid)
		defer r.Close()
		buff := new(bytes.Buffer)
		buff.ReadFrom(r)
		out := buff.String()
		object = append([]byte(fmt.Sprintf("%s %d\x00", shunt.objType, len(out))), out...)
		
		if shunt.objType != "blob" {
			f.processLinks(object)
		}

		object = compressObject(object)

		_, err = os.Stat(*objectPath)
		if os.IsNotExist(err) {
			err = ioutil.WriteFile(*objectPath, object, 0444)
			if err != nil {
				f.errCh <- fmt.Errorf("fetch: %v", err)
				return
			}
		}

		//TODO: see if moving this higher would help
		f.doneCh <- sha
	}()

	return nil
}

func (f *Fetch) processLinks(object []byte) error {
	nd, err := ipldgit.ParseObjectFromBuffer(object)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	links := nd.Links()
	for _, link := range links {
		mhash := link.Cid.Hash()
		hash := mhash.HexString()[4:]

		f.snLk.RLock()
		_, proc := f.seen[hash]
		f.snLk.RUnlock()
		
		if !!proc {
			continue
		}

		f.snLk.Lock()
		f.seen[hash] = true
		f.snLk.Unlock()

		f.todo <- hash
	}
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
