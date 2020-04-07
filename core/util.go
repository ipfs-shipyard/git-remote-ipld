package core

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path"
	"strings"

	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func compressObject(in []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(in)
	w.Close()
	return b.Bytes()
}

func isNoLink(err error) bool {
	return strings.Contains(err.Error(), "no link named") || strings.Contains(err.Error(), 
"no link by that name")
}

func GetLocalDir() (string, error) {
	fmt.Printf("GIT_DIR: %s\n", os.Getenv("GIT_DIR"))
	localdir := path.Join(os.Getenv("GIT_DIR"))

	fmt.Printf("Creating: %s\n", localdir)

	if err := os.MkdirAll(localdir, 0755); err != nil {
		return "", err
	}
	return localdir, nil
}

func CidFromHex(sha string) (cid.Cid, error) {
	mhash, err := mh.FromHexString("1114" + sha)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(0x78, mhash), nil
}

func HexFromCid(cid cid.Cid) (string, error) {
	if cid.Type() != 0x78 {
		return "", fmt.Errorf("unexpected cid type %d", cid.Type())
	}

	hash := cid.Hash()
	// TODO: validate length
	return hash.HexString()[4:], nil
}
