package main

import (
	"github.com/dgraph-io/badger/badger"
	"path"
	"os"
)

//Tracker tracks which hashes are published in IPLD
type Tracker struct {
	kv *badger.KV
}

func NewTracker(gitPath string) (*Tracker, error) {
	ipldDir := path.Join(gitPath, "ipld")
	err := os.MkdirAll(ipldDir, 0777)
	if err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions
	opt.Dir = ipldDir
	opt.ValueDir = ipldDir
	opt.ValueGCThreshold = 0

	kv, err := badger.NewKV(&opt)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		kv: kv,
	}, nil
}

func (t *Tracker) AddEntry(hash []byte) error {
	return t.kv.Set(hash, []byte{1})
}

func (t *Tracker) HasEntry(hash []byte) (bool, error) {
	var item badger.KVItem
	err := t.kv.Get(hash, &item)
	if err != nil {
		return false, err
	}

	return item.Value() != nil, nil
}

func (t *Tracker) Close() error {
	return t.kv.Close()
}
