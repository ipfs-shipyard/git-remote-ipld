package main

import (
	"compress/zlib"
	"container/list"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	ipfs "github.com/ipfs/go-ipfs-api"
	ipldgit "github.com/ipfs/go-ipld-git"

	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

type Push struct {
	objectDir string
	gitDir    string

	todo *list.List
	log  *log.Logger
}

func NewPush(gitDir string) *Push {
	return &Push{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,
		todo:      list.New(),
		log:       log.New(os.Stderr, "push: ", 0),
	}
}

func (p *Push) PushHash(hash string) error {
	p.todo.PushFront(hash)
	return p.doWork()
}

func (p *Push) doWork() error {
	tracker, err := NewTracker(p.gitDir)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}
	defer tracker.Close()

	api := ipfs.NewShell("localhost:5001") //todo: config

	for e := p.todo.Front(); e != nil; e = e.Next() {
		hash := e.Value.(string)

		mhash, err := mh.FromHexString("1114" + hash)
		if err != nil {
			return fmt.Errorf("push: %v", err)
		}

		expectedCid := cid.NewCidV1(0x78, mhash)

		objectPath, err := getHashPath(p.objectDir, hash)

		if _, err := os.Stat(objectPath); os.IsNotExist(err) {
			return fmt.Errorf("object doesn't exist: %s", objectPath)
		}

		f, err := os.Open(objectPath)
		if err != nil {
			return err
		}

		rawReader, err := zlib.NewReader(f)
		if err != nil {
			return err
		}

		raw, err := ioutil.ReadAll(rawReader)

		p.log.Printf("%s %s\r\x1b[A", hash, expectedCid.String())

		res, err := api.DagPut(raw, "raw", "git")
		if err != nil {
			return err
		}

		sha, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		err = tracker.AddEntry(sha)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		if expectedCid.String() != res {
			return fmt.Errorf("CIDs don't match: expected %s, got %s", expectedCid.String(), res)
		}

		p.processLinks(raw, tracker)
	}
	p.log.Printf("\n")
	return nil
}

func (p *Push) processLinks(object []byte, tracker *Tracker) error {
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

		has, err := tracker.HasEntry(decoded.Digest)
		if err != nil {
			return fmt.Errorf("push/process: %v", err)
		}

		if has {
			continue
		}

		p.todo.PushBack(hex.EncodeToString(decoded.Digest))
	}
	return nil
}

func getHashPath(localDir string, hash string) (string, error) {
	base := path.Join(localDir, hash[:2])
	objectPath := path.Join(base, hash[2:])
	return objectPath, nil
}
