package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/magik6k/git-remote-ipld/core"
)

const (
	IPLD_PREFIX = "ipld://"
	IPFS_PREFIX = "ipfs://"
)

func Main() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipld remote-name url")
	}

	hashArg := os.Args[2]
	if strings.HasPrefix(hashArg, IPLD_PREFIX) || strings.HasPrefix(hashArg, IPFS_PREFIX) {
		hashArg = hashArg[len(IPLD_PREFIX):]
	}

	remote, err := core.NewRemote(&IpldHandler{remoteHash: hashArg})
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
