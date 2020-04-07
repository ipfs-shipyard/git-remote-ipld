package core

import (
	"container/list"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"

	cid "github.com/ipfs/go-cid"
	ipfs "github.com/ipfs/go-ipfs-api"
	ipldgit "github.com/ipfs/go-ipld-git"
	mh "github.com/multiformats/go-multihash"
	sizedwaitgroup "github.com/remeh/sizedwaitgroup"
	git "gopkg.in/src-d/go-git.v4"
	plumbing "gopkg.in/src-d/go-git.v4/plumbing"
)

type Push struct {
	objectDir string
	gitDir    string

	done    uint64
	todoc   uint64
	todo    *list.List
	log     *log.Logger
	tracker *Tracker
	repo    *git.Repository
	shuntHash string

	processing map[string]int
	subs       map[string][][]byte

	errCh chan error
	wg    sizedwaitgroup.SizedWaitGroup

	NewNode func(hash cid.Cid, data []byte) error
}

func NewPush(gitDir string, tracker *Tracker, repo *git.Repository) *Push {
	return &Push{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,

		todo:    list.New(),
		log:     log.New(os.Stderr, "push: ", 0),
		tracker: tracker,
		repo:    repo,
		todoc:   1,
		shuntHash: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn",

		processing: map[string]int{},
		subs:       map[string][][]byte{},

		wg:    sizedwaitgroup.New(512),
		errCh: make(chan error),
	}
}

func (p *Push) PushHash(hash string) (string, error) {
	p.todo.PushFront(hash)
	return p.doWork()
}

func (p *Push) doWork() (string, error) {
	defer p.wg.Wait()

	api := ipfs.NewLocalShell()

	intch := make(chan os.Signal, 1)
	signal.Notify(intch, os.Interrupt)
	go func() {
		<-intch
		p.errCh <- errors.New("interrupted")
	}()
	defer signal.Stop(intch)

	for e := p.todo.Front(); e != nil; e = e.Next() {
		if df, ok := e.Value.(func() error); ok {
			if err := df(); err != nil {
				return "", err
			}
			p.todoc--
			continue
		}

		hash := e.Value.(string)

		sha, err := hex.DecodeString(hash)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		_, processing := p.processing[string(sha)]
		if processing {
			p.todoc--
			continue
		}

		has, err := p.tracker.HasEntry(sha)
		if err != nil {
			return "", fmt.Errorf("push/process: %v", err)
		}

		if has {
			p.todoc--
			continue
		}

		expectedCid, err := CidFromHex(hash)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		obj, err := p.repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(hash))
		if err != nil {
			return "", fmt.Errorf("push/getObject(%s): %v", hash, err)
		}

		rawReader, err := obj.Reader()
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		raw, err := ioutil.ReadAll(rawReader)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		isBlob := false

		switch obj.Type() {
		case plumbing.CommitObject:
			raw = append([]byte(fmt.Sprintf("commit %d\x00", obj.Size())), raw...)
		case plumbing.TreeObject:
			raw = append([]byte(fmt.Sprintf("tree %d\x00", obj.Size())), raw...)
		case plumbing.BlobObject:
			rawReader, err := obj.Reader()
			if err != nil {
				return "", fmt.Errorf("push: %v", err)
			}
			contentid, _ := api.Add(rawReader)
			p.log.Printf("Adding ID: %s (%s)\n", contentid, expectedCid)
			p.shuntHash, _ = api.PatchLink(p.shuntHash, expectedCid.String(), contentid, true)
			raw = append([]byte(fmt.Sprintf("blob %d\x00", obj.Size())), raw...)
			isBlob = true
		case plumbing.TagObject:
			raw = append([]byte(fmt.Sprintf("tag %d\x00", obj.Size())), raw...)
		}

		p.done++
		if p.done%1 == 0 || p.done == p.todoc {
			//p.log.Printf("%d/%d (P:%d) %s %s\r\x1b[A", p.done, p.todoc, len(p.processing), hash, expectedCid.String())
			p.log.Printf("%d/%d (P:%d) %s %s\n", p.done, p.todoc, len(p.processing), hash, expectedCid.String())
		}

		p.wg.Add()
		go func() {
			defer p.wg.Done()

			if !isBlob {
				res, err := api.BlockPut(raw, "git-raw", "sha1", -1)
				if err != nil {
					p.errCh <- fmt.Errorf("push/put: %v", err)
					return
				}

				p.log.Printf("Finished Block Put: %s ==? %s\n", res, expectedCid)

				if expectedCid.String() != res {
					p.errCh <- fmt.Errorf("CIDs don't match: expected %s, got %s", expectedCid, res)
					return
				}
			}

			if p.NewNode != nil {
				if err := p.NewNode(expectedCid, raw); err != nil {
					p.errCh <- fmt.Errorf("newNode: %s", err)
					return
				}
			}
		}()

		n, err := p.processLinks(raw, sha)
		if err != nil {
			return "", fmt.Errorf("push/processLinks: %v", err)
		}

		if n == 0 {
			p.todoc++
			p.todo.PushBack(p.doneFunc(sha))
		} else {
			p.processing[string(sha)] = n
		}

		select {
		case e := <-p.errCh:
			return "", e
		default:
			return p.shuntHash, nil
		}
	}
	p.log.Printf("\n")
	return p.shuntHash, nil
}

func (p *Push) doneFunc(sha []byte) func() error {
	return func() error {
		if err := p.tracker.AddEntry(sha); err != nil {
			return err
		}
		delete(p.processing, string(sha))

		for _, sub := range p.subs[string(sha)] {
			p.processing[string(sub)]--
			if p.processing[string(sub)] <= 0 {
				p.todoc++
				p.todo.PushBack(p.doneFunc(sub))
			}
		}
		delete(p.subs, string(sha))
		return nil
	}
}

func (p *Push) processLinks(object []byte, selfSha []byte) (int, error) {
	nd, err := ipldgit.ParseObjectFromBuffer(object)
	if err != nil {
		return 0, fmt.Errorf("push/process: %v", err)
	}

	var n int
	links := nd.Links()
	for _, link := range links {
		mhash := link.Cid.Hash()
		decoded, err := mh.Decode(mhash)
		if err != nil {
			return 0, fmt.Errorf("push/process: %v", err)
		}

		if _, proc := p.processing[string(decoded.Digest)]; !proc {
			has, err := p.tracker.HasEntry(decoded.Digest)
			if err != nil {
				return 0, fmt.Errorf("push/process: %v", err)
			}

			if has {
				continue
			}
		}

		p.subs[string(decoded.Digest)] = append(p.subs[string(decoded.Digest)], selfSha)

		n++
		p.todoc++
		p.todo.PushBack(hex.EncodeToString(decoded.Digest))
	}
	return n, nil
}
