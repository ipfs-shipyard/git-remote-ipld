package core

import (
	"fmt"
	"os"
	"path"
	"log"

	"github.com/dgraph-io/badger/v2"
)

//Tracker tracks which hashes are published in IPLD
type Tracker struct {
	db  *badger.DB
	txn *badger.Txn
	log *log.Logger
}

func NewTracker(gitPath string) (*Tracker, error) {
	log := log.New(os.Stderr, "\x1b[31mtracker:\x1b[39m ", 0)
	cacheDir := path.Join(gitPath, "remote-ipfs")
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions(cacheDir)
	opt.Logger = nil
	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		log: log,
		db:  db,
	}, nil
}

func (t *Tracker) AddEntry(hash string, c string) error {
	if t.txn == nil {
		t.txn = t.db.NewTransaction(true)
	}

	err := t.txn.Set([]byte(hash), []byte(c))
	if err != nil && err.Error() == badger.ErrTxnTooBig.Error() {
		if err := t.txn.Commit(); err != nil {
			return fmt.Errorf("commit: %s", err)
		}
		t.txn = t.db.NewTransaction(true)
		t.log.Printf("Tracker#AddEntry.txn.Set %s ⏩ %s\n", hash, c)
		if err := t.txn.Set([]byte(hash), []byte(c)); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("set: %s", err)
	}

	return nil
}

func (t *Tracker) Entry(hash string) (string, error) {
	if t.txn == nil {
		t.txn = t.db.NewTransaction(true)
	}

	ret, err := t.txn.Get([]byte(hash))
	if err == badger.ErrKeyNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	cBytes, err := ret.ValueCopy(nil)
	if err != nil {
		return "", err
	}

	c := string(cBytes)

	return c, nil
}

func (t *Tracker) Close() error {
	if t.txn != nil {
		if err := t.txn.Commit(); err != nil {
			return err
		}

	}
	return t.db.Close()
}
