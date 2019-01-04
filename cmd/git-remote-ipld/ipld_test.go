package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs-shipyard/git-remote-ipld/util"
)

func TestCapabilities(t *testing.T) {
	tmpdir := setupTest(t)
	defer os.RemoveAll(tmpdir)

	// git clone ipld::d5b0d08c180fd7a9bf4f684a37e60ceeb4d25ec8
	args := []string{"git-remote-ipld", "origin", "ipld://QmXUXGmpAJVKs1YWraqtZSbRKX7WamPKy5BqUY5cWEYaDV"}

	listExp := []string{
		"@refs/heads/master HEAD",
		"162429cc0dac923dff140ec29247f42a8e362419 refs/heads/french",
		"78a77abd233c24d8e6a0d0d040c79ae569fc7a19 refs/heads/italian",
		"8f329544bb70a739a8a65eda1c0aa949a971dede refs/heads/lobj",
		"d5b0d08c180fd7a9bf4f684a37e60ceeb4d25ec8 refs/heads/master",
	}
	listForPushExp := []string{
		"162429cc0dac923dff140ec29247f42a8e362419 refs/heads/french",
		"78a77abd233c24d8e6a0d0d040c79ae569fc7a19 refs/heads/italian",
		"8f329544bb70a739a8a65eda1c0aa949a971dede refs/heads/lobj",
		"d5b0d08c180fd7a9bf4f684a37e60ceeb4d25ec8 refs/heads/master",
	}

	testCase(t, args, "capabilities", []string{"push", "fetch"})
	testCase(t, args, "list", listExp)
	testCase(t, args, "list for-push", listForPushExp)

	// mock/git> git push --set-upstream ipld:: master
	testCase(t, args, "push refs/heads/master:refs/heads/master\n", []string{"ok refs/heads/master"})
	testCase(t, args, "push refs/heads/lobj:refs/heads/lobj\n", []string{"ok refs/heads/lobj"})

	testCase(t, args, "fetch d5b0d08c180fd7a9bf4f684a37e60ceeb4d25ec8 refs/heads/master\n", []string{""})
	comparePullToMock(t, tmpdir, "git")
}

func testCase(t *testing.T, args []string, input string, expected []string) {
	reader := strings.NewReader(input + "\n")
	var writer bytes.Buffer
	logger := log.New(os.Stderr, "", 0)
	err := Main(args, reader, &writer, logger)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	response := writer.String()
	exp := strings.Join(expected, "\n")
	if strings.TrimSpace(response) != exp {
		t.Fatalf("Args: %s\nInput:\n%s\nExpected:\n%s\nActual:\n'%s'\n", args, input, exp, response)
	}
}

func comparePullToMock(t *testing.T, tmpdir, mock string) {
	wd, _ := os.Getwd()
	mockdir := filepath.Join(wd, "..", "..", "mock", mock)
	compareContents(t, filepath.Join(tmpdir, ".git"), mockdir)
}

func compareContents(t *testing.T, src, dst string) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	err := util.CompareDirs(src, dst, []string{"ipld"})
	if err != nil {
		t.Fatal(err)
	}
}

func setupTest(t *testing.T) string {
	wd, _ := os.Getwd()
	src := filepath.Join(wd, "..", "..", "mock", "git")
	si, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if !si.IsDir() {
		t.Fatal("source is not a directory")
	}

	tmpdir, err := ioutil.TempDir("", "git-test")
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpdir, ".git")
	err = util.CopyDir(src, dst)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("GIT_DIR", dst)
	return tmpdir
}
