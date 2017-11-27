package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"github.com/magik6k/git-remote-ipld/core"
)

const (
	IPNS_PREFIX = "ipns://"
)

func Main() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipns remote-name url")
	}

	remoteName := os.Args[2]
	if strings.HasPrefix(remoteName, IPNS_PREFIX) {
		remoteName = remoteName[len(IPNS_PREFIX):]
	}

	remote, err := core.NewRemote(&IpnsHandler{remoteName:remoteName})
	if err != nil {
		return err
	}
	defer remote.Close()
	return remote.ProcessCommands()
}

func main() {
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
}
