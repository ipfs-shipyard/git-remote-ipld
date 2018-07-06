package main

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ipfs-shipyard/git-remote-ipld/core"

	"gx/ipfs/QmSVCWSGNwq9Lr1t4uLSMnytyJe4uL7NW7jZ3uas5BPpbX/go-git.v4/plumbing"
)

type IpldHandler struct {
	// remoteHash is hash from remote name
	remoteHash string
	// command line arguments
	osArgs []string
}

func (h *IpldHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	headRef, err := remote.Repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return nil, err
	}

	it, err := remote.Repo.Branches()
	if err != nil {
		return nil, err
	}

	out := make([]string, 0)
	var n int
	err = it.ForEach(func(ref *plumbing.Reference) error {
		n++
		trackedRef, err := remote.Tracker.Get(ref.Name().String())
		if err != nil {
			return err
		}
		if trackedRef == nil {
			trackedRef = make([]byte, 20)
		}

		// pull ipld::hash, we only want to update HEAD
		if !forPush && headRef.Target() == ref.Name() && headRef.Type() == plumbing.SymbolicReference && len(h.osArgs) >= 3 {
			sha, err := hex.DecodeString(h.remoteHash)
			if err != nil {
				return err
			}
			if len(sha) != 20 {
				return errors.New("invalid hash length")
			}

			out = append(out, fmt.Sprintf("%s %s", h.remoteHash, headRef.Target().String()))
		} else {
			// For other branches, or if pushing assume value from tracker
			out = append(out, fmt.Sprintf("%s %s", hex.EncodeToString(trackedRef), ref.Name()))
		}

		return nil
	})
	it.Close()
	if err != nil {
		return nil, err
	}

	// For clone
	if n == 0 && !forPush && len(h.osArgs) >= 3 {
		sha, err := hex.DecodeString(h.remoteHash)
		if err != nil {
			return nil, err
		}
		if len(sha) != 20 {
			return nil, errors.New("invalid hash length")
		}

		out = append(out, fmt.Sprintf("%s %s", h.remoteHash, "refs/heads/master"))
	}

	switch headRef.Type() {
	case plumbing.HashReference:
		out = append(out, fmt.Sprintf("%s %s", headRef.Hash(), headRef.Name()))
	case plumbing.SymbolicReference:
		out = append(out, fmt.Sprintf("@%s %s", headRef.Target().String(), headRef.Name()))
	}

	return out, nil
}

func (h *IpldHandler) Push(remote *core.Remote, local string, remoteRef string) (string, error) {
	localRef, err := remote.Repo.Reference(plumbing.ReferenceName(local), true)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	headHash := localRef.Hash().String()

	push := remote.NewPush()
	err = push.PushHash(headHash)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	hash := localRef.Hash()
	remote.Tracker.Set(remoteRef, (&hash)[:])

	c, err := core.CidFromHex(headHash)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	remote.Logger.Printf("Pushed to IPFS as \x1b[32mipld://%s\x1b[39m\n", headHash)
	remote.Logger.Printf("Head CID is %s\n", c.String())
	return local, nil
}

func (h *IpldHandler) Initialize(remote *core.Remote) error {
	return nil
}

func (h *IpldHandler) Finish(remote *core.Remote) error {
	return nil
}

func (h *IpldHandler) ProvideBlock(cid string, tracker *core.Tracker) ([]byte, error) {
	return nil, core.ErrNotProvided
}
