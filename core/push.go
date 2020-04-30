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

	git "github.com/go-git/go-git/v5"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	ipfs "github.com/ipfs/go-ipfs-api"
	ipldgit "github.com/ipfs/go-ipld-git"
	mh "github.com/multiformats/go-multihash"
	sizedwaitgroup "github.com/remeh/sizedwaitgroup"
)

type Push struct {
	objectDir string
	gitDir    string

	done      uint64
	todoc     uint64
	todo      *list.List
	log       *log.Logger
	tracker   *Tracker
	repo      *git.Repository
	shuntHash string
	shunts    map[string]string

	seen map[string]bool

	errCh chan error
	wg    sizedwaitgroup.SizedWaitGroup
}

func NewPush(gitDir string, tracker *Tracker, repo *git.Repository) *Push {
	return &Push{
		gitDir: gitDir,

		todo:      list.New(),
		log:       log.New(os.Stderr, "\x1b[33mpush:\x1b[39m ", 0),
		tracker:   tracker,
		repo:      repo,
		todoc:     1,
		shunts:    make(map[string]string),
		shuntHash: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn",

		seen: make(map[string]bool),

		wg:    sizedwaitgroup.New(512),
		errCh: make(chan error),
	}
}

func (p *Push) PushHash(hash string, remote *Remote) (string, error) {
	p.todo.PushFront(hash)
	res, err := p.doWork(remote)
	return res, err
}

func (p *Push) doWork(remote *Remote) (string, error) {
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

		entry, err := p.tracker.Entry(hash)
		if err != nil {
			return "", fmt.Errorf("push/process: %v", err)
		}

		obj, err := p.repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(hash))
		if err != nil {
			return "", fmt.Errorf("push/getObject(%s): %v", hash, err)
		}

		objType := "unknown"

		switch obj.Type() {
		case plumbing.CommitObject:
			objType = "commit"
		case plumbing.TreeObject:
			objType = "tree"
		case plumbing.BlobObject:
			objType = "blob"
		case plumbing.TagObject:
			objType = "tag"
		}

		if entry == "" {
			rawReader, err := obj.Reader()
			if err != nil {
				return "", fmt.Errorf("push: %v", err)
			}

			// panics and dies if API server isn't running
			entry, err = api.Add(rawReader)
			if err != nil {
				return "", fmt.Errorf("push: %v", err)
			}

			p.tracker.AddEntry(hash, entry)
		}

		p.shunts[hash] = entry
		p.shuntHash, err = api.PatchLink(p.shuntHash, objType + "s/" + hash, entry, true)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		p.done++
		if p.done%1 == 0 || p.done == p.todoc {
			p.log.Printf("%d/%d (P:%d) %s/%s (%s)\r\x1b[A", p.done, p.todoc, len(p.seen), objType, hash, entry)
		}

		if objType != "blob" {
			rawReader, err := obj.Reader()
			if err != nil {
				return "", fmt.Errorf("push: %v", err)
			}

			raw, err := ioutil.ReadAll(rawReader)
			if err != nil {
				return "", fmt.Errorf("push: %v", err)
			}

			raw = append([]byte(fmt.Sprintf("%s %d\x00", objType, obj.Size())), raw...)

			n, err := p.processLinks(raw)
			if err != nil {
				return "", fmt.Errorf("push/processLinks: %v", err)
			}

			if n == 0 {
				p.todoc++
				p.todo.PushBack(p.doneFunc(hash))
			}
		}
	}
	p.log.Printf("\n")
	return p.shuntHash, nil
}

func (p *Push) doneFunc(hash string) func() error {
	return func() error {
		if err := p.tracker.AddEntry(hash, p.shunts[hash]); err != nil {
			return err
		}
		return nil
	}
}

func (p *Push) processLinks(object []byte) (int, error) {
	nd, err := ipldgit.ParseObjectFromBuffer(object)
	if err != nil {
		return 0, fmt.Errorf("push/process: %v", err)
	}

	var n int
	links := nd.Links()
	for _, link := range links {
		decoded, err := mh.Decode(link.Cid.Hash())
		if err != nil {
			return 0, fmt.Errorf("push/process: %v", err)
		}

		hash := hex.EncodeToString(decoded.Digest)
		if _, proc := p.seen[hash]; !!proc {
			continue
		}
		p.seen[hash] = true

		n++
		p.todoc++
		p.todo.PushBack(hash)
	}
	return n, nil
}
