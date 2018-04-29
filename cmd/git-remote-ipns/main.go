package main

import (
	"fmt"
	"github.com/magik6k/git-remote-ipld/core"
	"io"
	"log"
	"os"
	"strings"
)

const (
	IPNS_PREFIX = "ipns://"
)

func Main(reader io.Reader, writer io.Writer, logger *log.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipns remote-name url")
	}

	remoteName := os.Args[2]
	if strings.HasPrefix(remoteName, IPNS_PREFIX) {
		remoteName = remoteName[len(IPNS_PREFIX):]
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
	if err := Main(os.Stdin, os.Stdout, nil); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "Done\n")
}
