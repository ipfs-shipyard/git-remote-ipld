package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"io/ioutil"
)

func getLocalDir() (string, error) {
	localdir := path.Join(os.Getenv("GIT_DIR"))

	if err := os.MkdirAll(localdir, 0755); err != nil {
		return "", err
	}
	return localdir, nil
}

func gitListRefs() (map[string]string, error) {
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
}

func gitSymbolicRef(name string) (string, error) {
	out, err := exec.Command("git", "symbolic-ref", name).Output()
	if err != nil {
		return "", fmt.Errorf(
			"GitSymbolicRef: git symbolic-ref %s: %v", name, out, err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func Main() error {
	l := log.New(os.Stderr, "", 0)
	l.Println(os.Args)

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

	//refspec := fmt.Sprintf("refs/heads/*:refs/ipld/%s/*", remoteName)

	/*err = os.Setenv("GIT_DIR", path.Join(*localDir, ".git"))
	if err != nil {
		return err
	}*/

	stdinReader := bufio.NewReader(os.Stdin)

	for {
		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}

		//l.Printf("< %s", command)
		switch {
		case command == "capabilities\n":
			//printf("refspec %s\n", refspec)
			printf("push\n")
			printf("fetch\n")
			printf("\n")
		case strings.HasPrefix(command, "list"):
			refs, err := gitListRefs()
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
			printf("\n")
		case strings.HasPrefix(command, "push "):
			refs := strings.Split(command[5:], ":")
			from := refs[0]
			head, err := ioutil.ReadFile(path.Join(localDir, from))
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			headHash := strings.Trim(string(head), " \t\n\r")

			push := NewPush(localDir)
			err = push.PushHash(headHash)
			if err != nil {
				return fmt.Errorf("command push: %v", err)
			}

			l.Printf("Pushed to IPFS as \x1b[32mipld::%s\x1b[39m\n", headHash)
			printf("ok refs/heads/master")
			printf("\n")
		case strings.HasPrefix(command, "fetch "):
			parts := strings.Split(command, " ")

			fetch := NewFetch(localDir)
			err := fetch.FetchHash(parts[1])
			if err != nil {
				return fmt.Errorf("command fetch: %v", err)
			}

			printf("\n")
		case command == "\n":
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
