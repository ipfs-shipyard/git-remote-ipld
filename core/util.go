package core

import (
	"bytes"
	"compress/zlib"
	"os"
	"path"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	"fmt"
)

func compressObject(in []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(in)
	w.Close()
	return b.Bytes()
}

func GetLocalDir() (string, error) {
	localdir := path.Join(os.Getenv("GIT_DIR"))

	if err := os.MkdirAll(localdir, 0755); err != nil {
		return "", err
	}
	return localdir, nil
}

func CidFromHex(sha string) (*cid.Cid, error) {
	mhash, err := mh.FromHexString("1114" + sha)
	if err != nil {
		return nil, err
	}

	return cid.NewCidV1(0x78, mhash), nil
}

func HexFromCid(cid *cid.Cid) (string, error) {
	if cid.Type() != 0x78 {
		return "", fmt.Errorf("unexpected cid type %d", cid.Type())
	}

	hash := cid.Hash()
	// TODO: validate length
	return hash.HexString()[4:], nil
}
