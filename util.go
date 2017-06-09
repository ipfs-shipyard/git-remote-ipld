package main

import (
	"compress/zlib"
	"bytes"
)

func compressObject(in []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(in)
	w.Close()
	return b.Bytes()
}

