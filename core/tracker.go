package core

import (
	"fmt"
	"os"
	"path"

	"github.com/dgraph-io/badger"
)

//Tracker tracks which hashes are published in IPLD
type Tracker struct {
	db *badger.DB

	txn *badger.Txn
}

func NewTracker(gitPath string) (*Tracker, error) {
	ipldDir := path.Join(gitPath, "ipld")
	err := os.MkdirAll(ipldDir, 0755)
	if err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions(ipldDir)

	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		db: db,
	}, nil
}

func (t *Tracker) Get(refName string) ([]byte, error) {
	txn := t.db.NewTransaction(false)
	defer txn.Discard()

	it, err := txn.Get([]byte(refName))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return it.ValueCopy(nil)
}

func (t *Tracker) Set(refName string, hash []byte) error {
	txn := t.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Set([]byte(refName), hash)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (t *Tracker) ListPrefixed(prefix string) (map[string]string, error) {
	out := map[string]string{}

	txn := t.db.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
		item := it.Item()
		k := item.Key()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		out[string(k)] = string(v)
	}

	return out, nil
}

func (t *Tracker) AddEntry(hash []byte) error {
	if t.txn == nil {
		t.txn = t.db.NewTransaction(true)
	}

	err := t.txn.Set([]byte(hash), []byte{})
	if err != nil && err.Error() == badger.ErrTxnTooBig.Error() {
		if err := t.txn.Commit(); err != nil {
			return fmt.Errorf("commit: %s", err)
		}
		t.txn = t.db.NewTransaction(true)
		if err := t.txn.Set([]byte(hash), []byte{}); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("set: %s", err)
	}

	return nil
}

func (t *Tracker) HasEntry(hash []byte) (bool, error) {
	if t.txn == nil {
		t.txn = t.db.NewTransaction(true)
	}

	_, err := t.txn.Get(hash)
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func (t *Tracker) Close() error {
	if t.txn != nil {
		if err := t.txn.Commit(); err != nil {
			return err
		}

	}
	return t.db.Close()
}
