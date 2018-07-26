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

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	git "gx/ipfs/QmSVCWSGNwq9Lr1t4uLSMnytyJe4uL7NW7jZ3uas5BPpbX/go-git.v4"
	plumbing "gx/ipfs/QmSVCWSGNwq9Lr1t4uLSMnytyJe4uL7NW7jZ3uas5BPpbX/go-git.v4/plumbing"
	sizedwaitgroup "gx/ipfs/QmTRr4W3zT41CJvnoFSmWu9PL9okw99na5XQG1t8JwSWP6/sizedwaitgroup"
	ipfs "gx/ipfs/QmZBfwm4fRhnk6L5qFKonmJTgVmv7D7LT93ky3WfgKVgxj/go-ipfs-api"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	ipldgit "gx/ipfs/QmazMphtdhiL37KJjfahQXjKvSNt6gjWqGpTs6KXQcAQ8w/go-ipld-git"
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

	errCh chan error
	wg    sizedwaitgroup.SizedWaitGroup

	NewNode func(hash *cid.Cid, data []byte) error
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

		wg:    sizedwaitgroup.New(512),
		errCh: make(chan error),
	}
}

func (p *Push) PushHash(hash string) error {
	p.todo.PushFront(hash)
	return p.doWork()
}

func (p *Push) doWork() error {
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
		hash := e.Value.(string)

		sha, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("push: %v", err)
		}

		has, err := p.tracker.HasEntry(sha)
		if err != nil {
			return fmt.Errorf("push/process: %v", err)
		}

		if has {
			p.todoc--
			continue
		}

		expectedCid, err := CidFromHex(hash)
		if err != nil {
			return fmt.Errorf("push: %v", err)
		}

		obj, err := p.repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(hash))
		if err != nil {
			return fmt.Errorf("push/getObject(%s): %v", hash, err)
		}

		rawReader, err := obj.Reader()
		if err != nil {
			return fmt.Errorf("push: %v", err)
		}

		raw, err := ioutil.ReadAll(rawReader)
		if err != nil {
			return fmt.Errorf("push: %v", err)
		}

		switch obj.Type() {
		case plumbing.CommitObject:
			raw = append([]byte(fmt.Sprintf("commit %d\x00", obj.Size())), raw...)
		case plumbing.TreeObject:
			raw = append([]byte(fmt.Sprintf("tree %d\x00", obj.Size())), raw...)
		case plumbing.BlobObject:
			raw = append([]byte(fmt.Sprintf("blob %d\x00", obj.Size())), raw...)
		case plumbing.TagObject:
			raw = append([]byte(fmt.Sprintf("tag %d\x00", obj.Size())), raw...)
		}

		p.done++
		if p.done%100 == 0 || p.done == p.todoc {
			p.log.Printf("%d/%d %s %s\r\x1b[A", p.done, p.todoc, hash, expectedCid.String())
		}

		p.wg.Add()
		go func() {
			defer p.wg.Done()

			res, err := api.BlockPut(raw, "git-raw", "sha1", -1)
			if err != nil {
				p.errCh <- fmt.Errorf("push/put: %v", err)
				return
			}

			if expectedCid.String() != res {
				p.errCh <- fmt.Errorf("CIDs don't match: expected %s, got %s", expectedCid.String(), res)
				return
			}

			if p.NewNode != nil {
				if err := p.NewNode(expectedCid, raw); err != nil {
					p.errCh <- fmt.Errorf("newNode: %s", err)
					return
				}
			}
		}()

		err = p.tracker.AddEntry(sha)
		if err != nil {
			return fmt.Errorf("push/addentry: %v", err)
		}

		p.processLinks(raw)

		select {
		case e := <-p.errCh:
			return e
		default:
		}
	}
	p.log.Printf("\n")
	return nil
}

func (p *Push) processLinks(object []byte) error {
	nd, err := ipldgit.ParseObjectFromBuffer(object)
	if err != nil {
		return fmt.Errorf("push/process: %v", err)
	}

	links := nd.Links()
	for _, link := range links {
		mhash := link.Cid.Hash()
		decoded, err := mh.Decode(mhash)
		if err != nil {
			return fmt.Errorf("push/process: %v", err)
		}

		has, err := p.tracker.HasEntry(decoded.Digest)
		if err != nil {
			return fmt.Errorf("push/process: %v", err)
		}

		if has {
			continue
		}

		p.todoc++
		p.todo.PushBack(hex.EncodeToString(decoded.Digest))
	}
	return nil
}
