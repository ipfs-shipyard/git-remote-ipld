package main

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"log"
	"os"
	"time"

	core "github.com/dhappy/git-remote-ipfs/core"
	ipfs "github.com/ipfs/go-ipfs-api"

	"github.com/ipfs/go-cid"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

type IPFSHandler struct {
	api *ipfs.Shell

	remoteName  string
	currentHash string

	largeObjs map[string]string

	log *log.Logger

	didPush bool
}

func (h *IPFSHandler) Initialize(remote *core.Remote) error {
	h.api = ipfs.NewLocalShell()
	h.currentHash = h.remoteName
	h.log = log.New(os.Stderr, "\x1b[32mhandler:\x1b[39m ", 0)
	return nil
}

func (h *IPFSHandler) Finish(remote *core.Remote) error {
	//TODO: publish
	if h.didPush {
		remote.Logger.Printf("Pushed to IPFS as \x1b[32mipld://%s\x1b[39m\n", h.currentHash)
	}
	return nil
}

func (h *IPFSHandler) GetRemoteName() string {
	return h.remoteName
}

func (h *IPFSHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	out := make([]string, 0)
	if !forPush {
		refs, err := h.paths(h.api, h.remoteName, 0)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			switch ref.rType {
			case REFPATH_HEAD:
				r := path.Join(strings.Split(ref.path, "/")[1:]...)
				out = append(out, fmt.Sprintf("%s %s", ref.hash, r))
			case REFPATH_REF:
				r := path.Join(strings.Split(ref.path, "/")[1:]...)
				dest, err := h.getRef(r)
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
			remoteRef := "0000000000000000000000000000000000000000"

			localRef, err := h.api.ResolvePath(path.Join(h.currentHash, ref.Name().String()))
			if err != nil && !isNoLink(err) {
				return err
			}
			if err == nil {
				refCid, err := cid.Parse(localRef)
				if err != nil {
					return err
				}

				remoteRef, err = core.HexFromCid(refCid)
				if err != nil {
					return err
				}
			}

			out = append(out, fmt.Sprintf("%s %s", remoteRef, ref.Name()))

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (h *IPFSHandler) Push(remote *core.Remote, local string, remoteRef string) (string, error) {
	h.didPush = true

	localRef, err := remote.Repo.Reference(plumbing.ReferenceName(local), true)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	root := localRef.Hash()

	push := remote.NewPush()
	h.currentHash, err = push.PushHash(root.String(), remote)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	exe, _ := os.Executable()
	if os.Getenv("GIT_IPFS_VFS") != "" || exe[(len(exe) - 6):] == "-ipvfs" {
		commit, _ := remote.Repo.CommitObject(root)
		c, _ := h.cidForCommit(commit, remote)
		remote.Logger.Printf("Adding: %s/content => %s\n", h.currentHash, c)
		h.currentHash, err = h.api.PatchLink(h.currentHash, "content", c, true)

		depth := 0
		object.NewCommitPreorderIter(commit, nil, nil).ForEach(func(commit *object.Commit) error {
			c, _ := h.cidForCommit(commit, remote)
			depth += 1
			_, err := h.placeCommitCID(commit, c, depth)
			return err
		})
	}

	hashHolder, err := h.api.Add(strings.NewReader(root.String()))
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	h.currentHash, err = h.api.PatchLink(h.currentHash, remoteRef, hashHolder, true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	headRef, err := h.api.Add(strings.NewReader(remoteRef))
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	h.currentHash, err = h.api.PatchLink(h.currentHash, "HEAD", headRef, true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	return local, nil
}

func (h *IPFSHandler) placeCommitCID(commit *object.Commit, c string, commitNum int) (string, error) {
	// Pretty much the only disallowed character is / which will create a subdirectory
	// %s are encoded so strings can be reliably percent decoded
	message := strings.Split(commit.Message, "\n")[0]
	messageEnc := strings.ReplaceAll(message, "%", "%25")
	messageEnc = strings.ReplaceAll(messageEnc, "\x00", "%00")
	messageEnc = strings.ReplaceAll(messageEnc, "/", "%2F")


	when := commit.Author.When.Format(time.RFC3339)
	entry := fmt.Sprintf("%s: %s – %s", when, commit.Author.Name, messageEnc)
	path := fmt.Sprintf("%s", entry)
	h.log.Printf("Adding: %s => %s\n", path, c)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, "vfs/commits/" + entry, c, true)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, fmt.Sprintf("vfs/commits/fwd/%020d: %s", commitNum, entry), c, true)

	return h.currentHash, nil
}

func (h *IPFSHandler) cidForCommit(commit *object.Commit, remote *core.Remote) (string, error) {
	tree, _ := commit.Tree()
	files := tree.Files()
	c := "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
	for leaf, _ := files.Next(); leaf != nil; leaf, _ = files.Next() {
		refId, err := remote.Tracker.Entry(leaf.Hash.String())
		remote.Logger.Printf("Adding: %s => %s (%s)\n", leaf.Name, leaf.Hash, refId)
		if err == nil && refId != "" {
			c, err = h.api.PatchLink(c, leaf.Name, refId, true)
			if err != nil {
				return "", fmt.Errorf("cidForCommit: %v", err)
			}
		} else {
			remote.Logger.Println("Couldn't Find Blob: ", leaf.Hash)
		}
	}
	return c, nil
}

func (h *IPFSHandler) getCid(cid string) (string, error) {
	r, err := h.api.Cat(cid)
	if err != nil {
		if isNoLink(err) {
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

func (h *IPFSHandler) getRef(name string) (string, error) {
	return h.getCid(path.Join(h.remoteName, name))
}

func (h *IPFSHandler) paths(api *ipfs.Shell, p string, level int) ([]refPath, error) {
	h.log.Println("IPFSHandler.paths: ", p)
	links, err := api.List(p)
	if err != nil {
		return nil, err
	}
	h.log.Println("IPFSHandler.paths.links:", len(links))

	out := make([]refPath, 0)
	for _, link := range links {
		switch link.Type {
		case ipfs.TDirectory:
			// read files in /refs/heads/
			if link.Name == "heads" || link.Name == "refs" {
				sub, err := h.paths(api, path.Join(p, link.Name), level + 1)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
			}
		case ipfs.TFile:
			h.log.Printf("Found File: %s\n", path.Join(p, link.Name))
			if link.Name == "HEAD" {
				out = append(out, refPath{path.Join(p, link.Name), REFPATH_REF, link.Hash})
			} else {
				hashVal, err := h.getCid(link.Hash)
				if err != nil {
					return nil, err
				}
				out = append(out, refPath{path.Join(p, link.Name), REFPATH_HEAD, hashVal})
			}
		case -1, 0: //unknown, assume git node
			h.log.Printf("Found Unknown: %s (%s)\n", path.Join(p, link.Name), link.Hash)
			out = append(out, refPath{path.Join(p, link.Name), REFPATH_HEAD, link.Hash})
		default:
			return nil, fmt.Errorf("unexpected link type %d", link.Type)
		}
	}
	return out, nil
}

func isNoLink(err error) bool {
	return strings.Contains(err.Error(), "no link named") || strings.Contains(err.Error(),
"no link by that name")
}
