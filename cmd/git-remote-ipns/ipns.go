package main

import (
	"fmt"
	"path"
	"strings"

	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/magik6k/git-remote-ipld/core"

	"bytes"
	"encoding/hex"
	"gopkg.in/src-d/go-git.v4/plumbing"
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

const (
	REFPATH_HEAD = iota
	REFPATH_REF
)

type refPath struct {
	path  string
	rType int

	hash string
}

type IpnsHandler struct {
	remoteName  string
	currentHash string
}

func (h *IpnsHandler) Initialize(remote *core.Remote) error {
	h.currentHash = h.remoteName
	return nil
}

func (h *IpnsHandler) Finish(remote *core.Remote) error {
	//TODO: publish

	remote.Logger.Printf("Pushed to IPFS as \x1b[32mipns::%s\x1b[39m\n", h.currentHash)
	return nil
}

func (h *IpnsHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	api := ipfs.NewLocalShell()

	out := make([]string, 0)
	if !forPush {
		refs, err := h.paths(api, h.remoteName)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			switch ref.rType {
			case REFPATH_HEAD:
				r := path.Join(strings.Split(ref.path, "/")[1:]...)
				c, err := cid.Parse(ref.hash)
				if err != nil {
					return nil, err
				}

				hash, err := core.HexFromCid(c)
				if err != nil {
					return nil, err
				}

				out = append(out, fmt.Sprintf("%s %s", hash, r))
			case REFPATH_REF:
				r := path.Join(strings.Split(ref.path, "/")[1:]...)
				dest, err := h.getRef(api, r)
				if err != nil {
					return nil, err
				}
				out = append(out, fmt.Sprintf("@%s %s", dest, r))
			}

		}
	} else {
		it, err := remote.Repo.Branches()
		if err != nil {
			return nil, err
		}

		err = it.ForEach(func(ref *plumbing.Reference) error {
			// resolve ref.Name().String()
			// return that

			remoteRef := make([]byte, 20)

			out = append(out, fmt.Sprintf("%s %s", hex.EncodeToString(remoteRef), ref.Name()))

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (h *IpnsHandler) Push(remote *core.Remote, local string, remoteRef string) (string, error) {
	api := ipfs.NewLocalShell()

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
	remote.Tracker.SetRef(remoteRef, (&hash)[:])

	c, err := core.CidFromHex(headHash)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	//patch object
	res, err := api.PatchLink(h.currentHash, remoteRef, c.String(), true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	head, err := h.getRef(api, "HEAD")
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	if head == "" {
		headRef, err := api.Add(strings.NewReader("refs/heads/master")) //TODO: Make this smarter?
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		res, err = api.PatchLink(res, "HEAD", headRef, true)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}
	}

	h.currentHash = res

	return local, nil
}

func (h *IpnsHandler) getRef(api *ipfs.Shell, name string) (string, error) {
	r, err := api.Cat(path.Join(h.remoteName, name))
	if err != nil {
		if strings.Contains(err.Error(), "cat: no link named") {
			return "", nil
		}
		return "", err
	}
	defer r.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(r)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (h *IpnsHandler) paths(api *ipfs.Shell, p string) ([]refPath, error) {
	links, err := api.List(p)
	if err != nil {
		return nil, err
	}

	out := make([]refPath, 0)
	for _, link := range links {
		switch link.Type {
		case ipfs.TDirectory:
			sub, err := h.paths(api, path.Join(p, link.Name))
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		case ipfs.TFile:
			out = append(out, refPath{path.Join(p, link.Name), REFPATH_REF, link.Hash})
		case -1: //unknown, git node
			out = append(out, refPath{path.Join(p, link.Name), REFPATH_HEAD, link.Hash})
		default:
			return nil, fmt.Errorf("unexpected link type %d", link.Type)
		}
	}
	return out, nil
}
