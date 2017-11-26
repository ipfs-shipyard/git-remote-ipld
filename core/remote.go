package core

import (
	"fmt"
	"log"
	"os"

	git "gopkg.in/src-d/go-git.v4"
	"path"
)

type Remote struct {
	Logger   *log.Logger
	localDir string

	Repo    *git.Repository
	Tracker *Tracker
}

func NewRemote() (*Remote, error) {
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
	defer tracker.Close()

	return &Remote{
		Logger:   log.New(os.Stderr, "", 0),
		localDir: localDir,

		Repo:    repo,
		Tracker: tracker,
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
