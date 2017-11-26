package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/magik6k/git-remote-ipld/core"
	"gopkg.in/src-d/go-git.v4/plumbing"
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

const (
	IPLD_PREFIX = "ipld://"
	IPFS_PREFIX = "ipfs://"
)

func Main() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipld remote-name url")
	}

	stdinReader := bufio.NewReader(os.Stdin)

	hashArg := os.Args[2]
	if strings.HasPrefix(hashArg, IPLD_PREFIX) || strings.HasPrefix(hashArg, IPFS_PREFIX) {
		hashArg = hashArg[len(IPLD_PREFIX):]
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

		remote.Logger.Printf("< %s", command)
		switch {
		case command == "capabilities":
			remote.Printf("push\n")
			remote.Printf("fetch\n")
			remote.Printf("\n")
		case strings.HasPrefix(command, "list"):
			headRef, err := remote.Repo.Reference(plumbing.HEAD, false)
			if err != nil {
				return err
			}

			it, err := remote.Repo.Branches()
			if err != nil {
				return err
			}

			var n int
			err = it.ForEach(func(ref *plumbing.Reference) error {
				n++
				r, err := remote.Tracker.GetRef(ref.Name().String())
				if err != nil {
					return err
				}
				if r == nil {
					r = make([]byte, 20)
				}

				if !strings.HasPrefix(command, "list for-push") && headRef.Target() == ref.Name() && headRef.Type() == plumbing.SymbolicReference && len(os.Args) >= 3 {
					sha, err := hex.DecodeString(hashArg)
					if err != nil {
						return err
					}
					if len(sha) != 20 {
						return errors.New("invalid hash length")
					}

					remote.Printf("%s %s\n", hashArg, headRef.Target().String())
				} else {
					remote.Printf("%s %s\n", hex.EncodeToString(r), ref.Name())
				}

				return nil
			})
			it.Close()
			if err != nil {
				return err
			}

			if n == 0 && !strings.HasPrefix(command, "list for-push") && len(os.Args) >= 3 {
				sha, err := hex.DecodeString(hashArg)
				if err != nil {
					return err
				}
				if len(sha) != 20 {
					return errors.New("invalid hash length")
				}

				remote.Printf("%s %s\n", hashArg, "refs/heads/master")
			}

			switch headRef.Type() {
			case plumbing.HashReference:
				remote.Printf("%s %s\n", headRef.Hash(), headRef.Name())
			case plumbing.SymbolicReference:
				remote.Printf("@%s %s\n", headRef.Target().String(), headRef.Name())
			}

			remote.Printf("\n")
		case strings.HasPrefix(command, "push "):
			refs := strings.Split(command[5:], ":")

			localRef, err := remote.Repo.Reference(plumbing.ReferenceName(refs[0]), true)
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			headHash := localRef.Hash().String()

			push := remote.NewPush()
			err = push.PushHash(headHash)
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			hash := localRef.Hash()
			remote.Tracker.SetRef(refs[1], (&hash)[:])

			mhash, err := mh.FromHexString("1114" + headHash)
			if err != nil {
				return fmt.Errorf("fetch: %v", err)
			}

			c := cid.NewCidV1(cid.GitRaw, mhash)

			remote.Logger.Printf("Pushed to IPFS as \x1b[32mipld::%s\x1b[39m\n", headHash)
			remote.Logger.Printf("Head CID is %s\n", c.String())
			remote.Printf("ok %s\n", refs[0])
			remote.Printf("\n")
		case strings.HasPrefix(command, "fetch "):
			parts := strings.Split(command, " ")

			fetch := remote.NewFetch()
			err := fetch.FetchHash(parts[1])
			if err != nil {
				return fmt.Errorf("command fetch: %v", err)
			}

			sha, err := hex.DecodeString(parts[1])
			if err != nil {
				return fmt.Errorf("push: %v", err)
			}

			remote.Tracker.SetRef(parts[2], sha)

			remote.Printf("\n")
		case command == "\n":
			return nil
		case command == "":
			return nil
		default:
			return fmt.Errorf("Received unknown command %q", command)
		}
	}
	return nil
}

func main() {
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[K")
		log.Fatal(err)
	}
}
