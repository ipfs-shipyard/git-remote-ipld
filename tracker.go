package main

import (
	"github.com/dgraph-io/badger"
	"os"
	"path"
)

//Tracker tracks which hashes are published in IPLD
type Tracker struct {
	kv *badger.KV
}

func NewTracker(gitPath string) (*Tracker, error) {
	ipldDir := path.Join(gitPath, "ipld")
	err := os.MkdirAll(ipldDir, 0755)
	if err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions
	opt.Dir = ipldDir
	opt.ValueDir = ipldDir

	kv, err := badger.NewKV(&opt)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		kv: kv,
	}, nil
}

func (t *Tracker) GetRef(refName string) ([]byte, error) {
	var it badger.KVItem
	err := t.kv.Get([]byte(refName), &it)
	if err != nil {
		return nil, err
	}
	return syncBytes(it.Value)
}

func (t *Tracker) SetRef(refName string, hash []byte) error {
	return t.kv.Set([]byte(refName), hash, 0)
}

func (t *Tracker) AddEntry(hash []byte) error {
	return t.kv.Set(hash, []byte{1}, 0)
}

func (t *Tracker) HasEntry(hash []byte) (bool, error) {
	var item badger.KVItem
	err := t.kv.Get(hash, &item)
	if err != nil {
		return false, err
	}

	val, err := syncBytes(item.Value)
	return val != nil, err
}

func (t *Tracker) Close() error {
	return t.kv.Close()
}

func syncBytes(get func(func([]byte) error) error) ([]byte, error) {
	var out []byte
	err := get(func(data []byte) error {
		out = data
		return nil
	})

	return out, err
}
