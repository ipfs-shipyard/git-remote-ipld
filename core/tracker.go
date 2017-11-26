package core

import (
	"github.com/dgraph-io/badger"
	"os"
	"path"
)

//Tracker tracks which hashes are published in IPLD
type Tracker struct {
	db *badger.DB
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

	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		db: db,
	}, nil
}

func (t *Tracker) GetRef(refName string) ([]byte, error) {
	txn := t.db.NewTransaction(false)
	defer txn.Discard()

	it, err := txn.Get([]byte(refName))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return it.Value()
}

func (t *Tracker) SetRef(refName string, hash []byte) error {
	txn := t.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Set([]byte(refName), hash, 0)
	if err != nil {
		return err
	}

	return txn.Commit(nil)
}

func (t *Tracker) AddEntry(hash []byte) error {
	txn := t.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Set([]byte(hash), []byte{}, 1)
	if err != nil {
		return err
	}

	return txn.Commit(nil)
}

func (t *Tracker) HasEntry(hash []byte) (bool, error) {
	txn := t.db.NewTransaction(false)
	defer txn.Discard()

	it, err := txn.Get(hash)
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	it.UserMeta()

	return true, nil
}

func (t *Tracker) Close() error {
	return t.db.Close()
}
