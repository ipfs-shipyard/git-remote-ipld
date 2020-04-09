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

	done      uint64
	todoc     uint64
	todo      *list.List
	log       *log.Logger
	tracker   *Tracker
	repo      *git.Repository
	shuntHash string
	shunts    map[string]string

	processing map[string]int
	subs       map[string][]string

	errCh chan error
	wg    sizedwaitgroup.SizedWaitGroup
}

func NewPush(gitDir string, tracker *Tracker, repo *git.Repository) *Push {
	return &Push{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,

		todo:    list.New(),
		log:     log.New(os.Stderr, "\x1b[33mpush:\x1b[39m ", 0),
		tracker: tracker,
		repo:    repo,
		todoc:   1,
		shunts:   make(map[string]string),
		shuntHash: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn",

		processing: map[string]int{},
		subs:       make(map[string][]string),

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

		sha, err := hex.DecodeString(hash)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		_, processing := p.processing[hash]
		if processing {
			p.log.Println("Currently Processing: ", hash)
			p.todoc--
			continue
		}

		entry, err := p.tracker.Entry(hash)
		if err != nil {
			return "", fmt.Errorf("push/process: %v", err)
		}

		if entry != "" {
			p.log.Printf("Cache Hit: %s (%s)\n", hash, entry)
			//p.todoc--
			//continue
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
			p.shunts[hash] = contentid
			p.shuntHash, _ = api.PatchLink(p.shuntHash, hash, contentid, true)
			p.log.Printf("Adding ID: %s (%s)\n", hash, p.shuntHash)
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

				if expectedCid.String() != res {
					p.errCh <- fmt.Errorf("CIDs don't match: expected %s, got %s", expectedCid, res)
					return
				}

				if err := remote.Tracker.AddEntry(hash, res); err != nil {
					p.errCh <- fmt.Errorf("push/put: %v", err)
				}
			}

			p.log.Printf("Finished Block Put: %s\n", hash)
		}()

		p.log.Printf("Push#doWork.processLinks(): %s\n", hash)
		n, err := p.processLinks(raw, sha)
		if err != nil {
			return "", fmt.Errorf("push/processLinks: %v", err)
		}

		if n == 0 {
			p.todoc++
			p.todo.PushBack(p.doneFunc(hash, p.shunts[hash]))
		} else {
			p.processing[string(sha)] = n
		}

		select {
		case e := <-p.errCh:
			return "", e
		default:
		}
	}
	p.log.Printf("\n")
	return p.shuntHash, nil
}

func (p *Push) doneFunc(hash string, c string) func() error {
	p.log.Printf("Push#doneFunc.sha == %s (%s)\n", hash, p.shunts[hash])
	return func() error {
		if err := p.tracker.AddEntry(hash, p.shunts[hash]); err != nil {
			return err
		}
		delete(p.processing, hash)

		for _, sub := range p.subs[hash] {
			p.processing[sub]--
			if p.processing[sub] <= 0 {
				p.todoc++
				p.todo.PushBack(p.doneFunc(sub, p.shunts[sub]))
			}
		}
		delete(p.subs, hash)
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

		hash := hex.EncodeToString(decoded.Digest)
		p.log.Println("Push#Links.hash == ", hash)

		if _, proc := p.processing[hash]; !proc {
			entry, err := p.tracker.Entry(hash)
			if err != nil {
				return 0, fmt.Errorf("push/process: %v", err)
			}

			if entry != "" {
				p.log.Println("Push#Links Cache Hit: ", hex.EncodeToString(decoded.Digest))
				//continue
			}
		}

		p.log.Println("Push#processLinks.selfSha == ", hex.EncodeToString(selfSha))

		//p.subs[string(decoded.Digest)] = append(p.subs[string(decoded.Digest)], selfSha)
		p.subs[hash] = append(p.subs[hash], hex.EncodeToString(selfSha))

		n++
		p.todoc++
		p.todo.PushBack(hex.EncodeToString(decoded.Digest))
	}
	return n, nil
}
