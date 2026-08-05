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
	PREFIX_SEP = "://"

	IPLD_PREFIX = "ipld" + PREFIX_SEP
	IPFS_PREFIX = "ipfs" + PREFIX_SEP

	EMPTY_REPO = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
)

func Main(args []string, reader io.Reader, writer io.Writer, logger *log.Logger) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: git-remote-ipns remote-name url")
	}

	remoteName := args[2]

	sepIdx := strings.Index(remoteName, PREFIX_SEP)
	prefix := ""
	if sepIdx > 0 {
		prefix = remoteName[:sepIdx + len(PREFIX_SEP)]
		remoteName = remoteName[len(prefix):]
	} else {
		return fmt.Errorf("Invalid URL: %s", remoteName)
	}

	apiPrefixIdx := strings.Index(prefix, "+")
	apiPrefix := ""
	if apiPrefixIdx == 0 {
		return fmt.Errorf("Invalid URL prefix: %s", prefix)
	} else if apiPrefixIdx > 0 {
		apiPrefix = prefix[apiPrefixIdx+1:]
		prefix = prefix[:apiPrefixIdx] + PREFIX_SEP
	}

	if prefix != IPLD_PREFIX && prefix != IPFS_PREFIX {
		return fmt.Errorf("Unknown URL prefix: %s", prefix)
	}

	slashIdx := strings.LastIndex(remoteName, "/")

	apiURL := os.Getenv("IPFS_API_URL")
	if slashIdx < 0 {
		if len(apiPrefix) != 0 {
			return fmt.Errorf("Prefix has been provided for the API, but not an API URL")
		}
	} else {
		apiURL = apiPrefix + remoteName[:slashIdx+1]
	}

	// Quirk: The go-ipfs-api throws unexpected redirect if the API URL ends with /
	if strings.HasSuffix(apiURL, "/") {
		apiURL = apiURL[:len(apiURL)-1]
	}

	if len(apiURL) > 0 {
		fmt.Fprintf(os.Stderr, "Using API URL: %s\n", apiURL)
	}

	remoteName = remoteName[slashIdx+1:]
	if remoteName == "" {
		remoteName = EMPTY_REPO
	}

	remote, err := core.NewRemote(&IpnsHandler{apiURL: apiURL, remoteName: remoteName}, reader, writer, logger, apiURL)
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
