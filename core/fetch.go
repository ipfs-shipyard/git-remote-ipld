package core

import (
	"container/list"

	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"encoding/hex"
	ipfs "github.com/ipfs/go-ipfs-api"
	ipldgit "github.com/ipfs/go-ipld-git"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
)

type Fetch struct {
	objectDir string
	gitDir    string

	todo    *list.List
	log     *log.Logger
	tracker *Tracker
}

func NewFetch(gitDir string, tracker *Tracker) *Fetch {
	return &Fetch{
		objectDir: path.Join(gitDir, "objects"),
		gitDir:    gitDir,
		todo:      list.New(),
		log:       log.New(os.Stderr, "fetch: ", 0),
		tracker:   tracker,
	}
}

func (f *Fetch) FetchHash(base string) error {
	f.todo.PushFront(base)
	return f.doWork()
}

func (f *Fetch) doWork() error {
	api := ipfs.NewLocalShell()

	for e := f.todo.Front(); e != nil; e = e.Next() {
		hash := e.Value.(string)

		mhash, err := mh.FromHexString("1114" + hash)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		c := cid.NewCidV1(cid.GitRaw, mhash)

		f.log.Printf("%s %s\r\x1b[A", hash, c.String())

		objectPath, err := prepHashPath(f.objectDir, hash)
		if err != nil {
			return err
		}

		if _, err := os.Stat(*objectPath); !os.IsNotExist(err) {
			continue
		}

		object, err := api.BlockGet(c.String())
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		f.processLinks(object)

		object = compressObject(object)

		/////////////////

		err = ioutil.WriteFile(*objectPath, object, 0444)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		sha, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		err = f.tracker.AddEntry(sha)
		if err != nil {
			return fmt.Errorf("fetch: %v", err)
		}
	}
	f.log.Printf("\n")
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
		objectPath, err := prepHashPath(f.objectDir, hash)
		if err != nil {
			return err
		}

		if _, err := os.Stat(*objectPath); !os.IsNotExist(err) {
			continue
		}

		f.todo.PushBack(hash)
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
