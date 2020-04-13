package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"os/exec"
	"bytes"
	"regexp"

	"github.com/dhappy/git-remote-ipfs/core"
)

const (
	IPNS_PREFIX = "ipns://"
	IPFS_PREFIX = "ipfs://"
	EMPTY_REPO = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
)

func Main(args []string, reader io.Reader, writer io.Writer) error {
	log := log.New(os.Stderr, "\x1b[34mmain:\x1b[39m ", 0)

	if len(args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipfs remote-name url")
	}

	remoteName := args[2]
	if strings.HasPrefix(remoteName, IPNS_PREFIX) {
		remoteName = remoteName[len(IPFS_PREFIX):]

		cmd := exec.Command("ipfs", "name", "resolve", remoteName)

		var out bytes.Buffer; cmd.Stdout = &out; cmd.Stderr = &out

		log.Printf("IPNS Resolving \x1b[35m%s\x1b[39m:", remoteName)
		err := cmd.Run()
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`/ipfs/(.+)`)
		remoteName = re.FindStringSubmatch(out.String())[1]
	}
	if strings.HasPrefix(remoteName, IPFS_PREFIX) {
		remoteName = remoteName[len(IPFS_PREFIX):]
	}
	if remoteName == "" {
		remoteName = EMPTY_REPO
	}

	remote, err := core.NewRemote(&IPFSHandler{remoteName: remoteName}, reader, writer)
	if err != nil {
		return err
	}

	if err := remote.ProcessCommands(); err != nil {
		err2 := remote.Close()
		if err2 != nil {
			return fmt.Errorf("%s; close error: %s", err, err2)
		}
		return err
	}

	return remote.Close()
}

func main() {
	if err := Main(os.Args, os.Stdin, os.Stdout); err != nil {
		//fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
}
