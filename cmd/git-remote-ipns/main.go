package main

import (
	"bufio"
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
		return fmt.Errorf("Usage: git-remote-ipld remote-name url")
	}

	stdinReader := bufio.NewReader(os.Stdin)

	hashArg := os.Args[2]
	if strings.HasPrefix(hashArg, IPNS_PREFIX) {
		hashArg = hashArg[len(IPNS_PREFIX):]
	}

	remote, err := core.NewRemote()
	if err != nil {
		return err
	}
	defer remote.Close()

	for {
		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}

		command = strings.Trim(command, "\n")

	}
	return nil
}

func main() {
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
}
