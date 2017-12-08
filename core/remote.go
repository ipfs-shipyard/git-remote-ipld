package core

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
)

type RemoteHandler interface {
	List(remote *Remote, forPush bool) ([]string, error)
	Push(remote *Remote, localRef string, remoteRef string) (string, error)
}

type Remote struct {
	Logger   *log.Logger
	localDir string

	Repo    *git.Repository
	Tracker *Tracker

	Handler RemoteHandler

	todo []func() (string, error)
}

func NewRemote(handler RemoteHandler) (*Remote, error) {
	localDir, err := GetLocalDir()
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(localDir)
	if err == git.ErrWorktreeNotProvided {
		repoRoot, _ := path.Split(localDir)

		repo, err = git.PlainOpen(repoRoot)
		if err != nil {
			return nil, err
		}
	}

	tracker, err := NewTracker(localDir)
	if err != nil {
		return nil, fmt.Errorf("fetch: %v", err)
	}

	return &Remote{
		Logger:   log.New(os.Stderr, "", 0),
		localDir: localDir,

		Repo:    repo,
		Tracker: tracker,

		Handler: handler,
		}, nil
}

func (r *Remote) Printf(format string, a ...interface{}) (n int, err error) {
	r.Logger.Printf("> "+format, a...)
	return fmt.Printf(format, a...)
}

func (r *Remote) NewPush() *Push {
	return NewPush(r.localDir, r.Tracker, r.Repo)
}

func (r *Remote) NewFetch() *Fetch {
	return NewFetch(r.localDir, r.Tracker)
}

func (r *Remote) Close() error {
	return r.Tracker.Close()
}

func (r *Remote) push(src, dst string, force bool) {
	r.todo = append(r.todo, func() (string, error) {
		done, err := r.Handler.Push(r, src, dst)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("ok %s\n", done), nil
	})
}

func (r *Remote) fetch(sha, ref string) {
	r.todo = append(r.todo, func() (string, error) {
		fetch := r.NewFetch()
		err := fetch.FetchHash(sha)
		if err != nil {
			return "", fmt.Errorf("command fetch: %v", err)
		}

		sha, err := hex.DecodeString(sha)
		if err != nil {
			return "", fmt.Errorf("fetch: %v", err)
		}

		r.Tracker.SetRef(ref, sha)
		return "", nil
	})
}

func (r *Remote) ProcessCommands() error {
	stdinReader := bufio.NewReader(os.Stdin)
	for {
		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}

		command = strings.Trim(command, "\n")

		r.Logger.Printf("< %s", command)
		switch {
		case command == "capabilities":
			r.Printf("push\n")
			r.Printf("fetch\n")
			r.Printf("\n")
		case strings.HasPrefix(command, "list"):
			list, err := r.Handler.List(r, strings.HasPrefix(command, "list for-push"))
			if err != nil {
				return err
			}
			for _, e := range list {
				r.Printf("%s\n", e)
			}
			r.Printf("\n")
		case strings.HasPrefix(command, "push "):
			refs := strings.Split(command[5:], ":")
			r.push(refs[0], refs[1], false) //TODO: parse force
		case strings.HasPrefix(command, "fetch "):
			parts := strings.Split(command, " ")
			r.fetch(parts[1], parts[2])
		case command == "":
			fallthrough
		case command == "\n":
			r.Logger.Println("Processsing tasks")
			for _, task := range r.todo {
				resp, err := task()
				if err != nil {
					return err
				}
				r.Printf("%s", resp)
			}
			r.Printf("\n")
			r.todo = nil
			return nil
		default:
			return fmt.Errorf("received unknown command %q", command)
		}
	}
}
