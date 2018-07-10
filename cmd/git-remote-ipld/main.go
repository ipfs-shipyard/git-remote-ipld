package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ipfs-shipyard/git-remote-ipld/core"
)

const (
	IPLD_PREFIX = "ipld://"
	IPFS_PREFIX = "ipfs://"
)

func Main(args []string, reader io.Reader, writer io.Writer, logger *log.Logger) error {
	if len(args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipns remote-name url")
	}

	remoteName := args[2]
	if strings.HasPrefix(remoteName, IPLD_PREFIX) || strings.HasPrefix(remoteName, IPFS_PREFIX) {
		remoteName = remoteName[len(IPLD_PREFIX):]
	}

	remote, err := core.NewRemote(&IpnsHandler{remoteName: remoteName}, reader, writer, logger)
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
	if err := Main(os.Args, os.Stdin, os.Stdout, nil); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "Done\n")
}
