package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"encoding/hex"
)

func getLocalDir() (string, error) {
	localdir := path.Join(os.Getenv("GIT_DIR"))

	if err := os.MkdirAll(localdir, 0755); err != nil {
		return "", err
	}
	return localdir, nil
}

/*func gitListRefs() (map[string]string, error) {
	out, err := exec.Command(
		"git", "for-each-ref", "--format=%(objectname) %(refname)",
		"refs/heads/",
	).Output()
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(out, []byte{'\n'})
	refs := make(map[string]string, len(lines))

	for _, line := range lines {
		fields := bytes.Split(line, []byte{' '})

		if len(fields) < 2 {
			break
		}

		refs[string(fields[1])] = string(fields[0])
	}

	return refs, nil
}*/

/*func gitSymbolicRef(name string) (string, error) {
	out, err := exec.Command("git", "symbolic-ref", name).Output()
	if err != nil {
		return "", fmt.Errorf(
			"GitSymbolicRef: git symbolic-ref %s: %v", name, out, err)
	}

	return string(bytes.TrimSpace(out)), nil
}*/

func Main() error {
	l := log.New(os.Stderr, "", 0)

	printf := func(format string, a ...interface{}) (n int, err error) {
		//l.Printf("> "+format, a...)
		return fmt.Printf(format, a...)
	}

	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: git-remote-ipld remote-name url")
	}

	//remoteName := os.Args[1]

	localDir, err := getLocalDir()
	if err != nil {
		return err
	}

	repo, err := git.PlainOpen(localDir)
	if err == git.ErrWorktreeNotProvided {
		repoRoot, _ := path.Split(localDir)

		repo, err = git.PlainOpen(repoRoot)
		if err != nil {
			return err
		}
	}

	tracker, err := NewTracker(localDir)
	if err != nil {
		return fmt.Errorf("fetch: %v", err)
	}
	defer tracker.Close()

	stdinReader := bufio.NewReader(os.Stdin)

	for {
		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}

		command = strings.Trim(command, "\n")

		//l.Printf("< %s", command)
		switch {
		case command == "capabilities":
			//printf("refspec %s\n", refspec)
			printf("push\n")
			printf("fetch\n")
			printf("\n")
		case strings.HasPrefix(command, "list"):
			/*refs, err := gitListRefs()
			if err != nil {
				printf("%s refs/heads/master\n", os.Args[2]) //todo: check hash
				printf("@%s HEAD\n", "refs/heads/master")
			} else {
				head, err := gitSymbolicRef("HEAD")
				if err != nil {
					return fmt.Errorf("command list: %v", err)
				}

				if len(refs) == 0 {
					printf("%s refs/heads/master\n", os.Args[2]) //todo: check hash
				}
				for refname := range refs {
					printf("%s %s\n", os.Args[2], refname) //todo: check hash
				}

				printf("@%s HEAD\n", head)
			}
			printf("\n")*/

			it, err := repo.Branches()
			if err != nil {
				return err
			}

			it.ForEach(func(ref *plumbing.Reference) error {
				r, err := tracker.GetRef(ref.Name().String())
				if err != nil {
					return err
				}
				if r == nil {
					r = make([]byte, 20)
				}

				printf("%s %s\n", hex.EncodeToString(r), ref.Name())

				return nil
			})

			headRef, err := repo.Reference(plumbing.HEAD, false)
			if err != nil {
				return err
			}

			switch headRef.Type() {
			case plumbing.HashReference:
				printf("%s %s\n", headRef.Hash(), headRef.Name())
			case plumbing.SymbolicReference:
				printf("@%s %s\n", headRef.Target().String(), headRef.Name())
			}

			printf("\n")
		case strings.HasPrefix(command, "push "):
			refs := strings.Split(command[5:], ":")

			localRef, err := repo.Reference(plumbing.ReferenceName(refs[0]), true)
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			headHash := localRef.Hash().String()

			push := NewPush(localDir, tracker, repo)
			err = push.PushHash(headHash)
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			hash := localRef.Hash()
			l.Printf("setref '%s'\n", refs[1])
			tracker.SetRef(refs[1], (&hash)[:])

			l.Printf("Pushed to IPFS as \x1b[32mipld::%s\x1b[39m\n", headHash)
			printf("ok %s\n", refs[0])
			printf("\n")
		case strings.HasPrefix(command, "fetch "):
			parts := strings.Split(command, " ")

			fetch := NewFetch(localDir, tracker)
			err := fetch.FetchHash(parts[1])
			if err != nil {
				return fmt.Errorf("command fetch: %v", err)
			}

			printf("\n")
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
		log.Fatal(err)
	}
}
