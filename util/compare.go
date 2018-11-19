package util

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type byName []os.FileInfo

func (n byName) Len() int           { return len(n) }
func (n byName) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n byName) Less(i, j int) bool { return n[i].Name() < n[j].Name() }

func CompareDirs(srcPath, dstPath string, ignore []string) error {
	ignoreSet := map[string]bool{}
	for _, val := range ignore {
		ignoreSet[val] = true
	}

	filterEntries := func(a []os.FileInfo) []os.FileInfo {
		count := 0
		for i := 0; i < len(a)-count; i++ {
			if ignoreSet[a[i].Name()] {
				a[i] = a[len(a)-1-count]
				count++
			}
		}
		return a[:len(a)-count]
	}

	srcEntries, err := ioutil.ReadDir(srcPath)
	if err != nil {
		return err
	}
	dstEntries, err := ioutil.ReadDir(dstPath)
	if err != nil {
		return err
	}

	srcEntries = filterEntries(srcEntries)
	dstEntries = filterEntries(dstEntries)
	sort.Sort(byName(srcEntries))
	sort.Sort(byName(dstEntries))

	for i := 0; float64(i) < math.Max(float64(len(srcEntries)), float64(len(dstEntries))); i++ {
		if i >= len(srcEntries) {
			dstSubPath := filepath.Join(dstPath, dstEntries[i].Name())
			return fmt.Errorf("File %s in destination directory does not exist in source directory %s", dstSubPath, srcPath)
		}
		if i >= len(dstEntries) {
			srcSubPath := filepath.Join(srcPath, srcEntries[i].Name())
			return fmt.Errorf("File %s in source directory does not exist in destination directory %s", srcSubPath, dstPath)
		}
		if srcEntries[i].Name() < dstEntries[i].Name() {
			srcSubPath := filepath.Join(srcPath, srcEntries[i].Name())
			return fmt.Errorf("File %s in source directory does not exist in destination directory %s", srcSubPath, dstPath)
		} else if srcEntries[i].Name() > dstEntries[i].Name() {
			dstSubPath := filepath.Join(dstPath, dstEntries[i].Name())
			return fmt.Errorf("File %s in destination directory does not exist in source directory %s", dstSubPath, srcPath)
		}
	}

	for _, entry := range srcEntries {
		srcSubPath := filepath.Join(srcPath, entry.Name())
		dstSubPath := filepath.Join(dstPath, entry.Name())

		if entry.IsDir() {
			err = CompareDirs(srcSubPath, dstSubPath, ignore)
			if err != nil {
				return err
			}
			// Skip symlinks.
		} else if entry.Mode()&os.ModeSymlink == 0 {
			err = CompareFiles(srcSubPath, dstSubPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

const chunkSize = 64000

func CompareFiles(file1, file2 string) error {
	f1, err := os.Open(file1)
	if err != nil {
		return err
	}

	f2, err := os.Open(file2)
	if err != nil {
		return err
	}

	for {
		b1 := make([]byte, chunkSize)
		_, err1 := f1.Read(b1)

		b2 := make([]byte, chunkSize)
		_, err2 := f2.Read(b2)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return nil
			} else if err1 == io.EOF || err2 == io.EOF {
				f1.Close()
				f2.Close()
				return CompareZlib(file1, file2)
			} else {
				return err1
			}
		}

		if !bytes.Equal(b1, b2) {
			f1.Close()
			f2.Close()
			return CompareZlib(file1, file2)
		}
	}
}

func CompareZlib(file1, file2 string) error {
	z1, err := os.Open(file1)
	if err != nil {
		return err
	}

	z2, err := os.Open(file2)
	if err != nil {
		return err
	}

	f1, err := zlib.NewReader(z1)
	if err != nil {
		return err
	}

	f2, err := zlib.NewReader(z2)
	if err != nil {
		return err
	}

	for {
		b1 := make([]byte, chunkSize)
		_, err1 := f1.Read(b1)

		b2 := make([]byte, chunkSize)
		_, err2 := f2.Read(b2)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return nil
			} else if err1 == io.EOF || err2 == io.EOF {
				return fmt.Errorf("File %s != %s", file1, file2)
			} else {
				return err1
			}
		}

		if !bytes.Equal(b1, b2) {
			return fmt.Errorf("File %s != %s", file1, file2)
		}
	}
}
