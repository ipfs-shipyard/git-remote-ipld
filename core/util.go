package core

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path"

	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
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
