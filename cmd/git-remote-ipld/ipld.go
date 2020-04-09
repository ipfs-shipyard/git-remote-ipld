package main

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"log"
	"os"

	core "github.com/dhappy/git-remote-ipld/core"
	ipfs "github.com/ipfs/go-ipfs-api"

	"github.com/ipfs/go-cid"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

const (
	LARGE_OBJECT_DIR    = "objects"
	LOBJ_TRACKER_PRIFIX = "//lobj"
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
	api *ipfs.Shell

	remoteName  string
	currentHash string

	largeObjs map[string]string

	log *log.Logger

	didPush bool
}

func (h *IpnsHandler) Initialize(remote *core.Remote) error {
	h.api = ipfs.NewLocalShell()
	h.currentHash = h.remoteName
	h.log = log.New(os.Stderr, "\x1b[32mhandler:\x1b[39m ", 0)
	h.log.Println("Starting Object Hash: ", h.currentHash)
	return nil
}

func (h *IpnsHandler) Finish(remote *core.Remote) error {
	//TODO: publish
	if h.didPush {
		remote.Logger.Printf("Pushed to IPFS as \x1b[32mipld://%s\x1b[39m\n", h.currentHash)
	}
	return nil
}

func (h *IpnsHandler) GetRemoteName() string {
	return h.remoteName
}

func (h *IpnsHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	h.log.Println("IpnsHandler.List: forPush ==", forPush)
	out := make([]string, 0)
	if !forPush {
		h.log.Println("Starting Paths with: ", h.remoteName)
		refs, err := h.paths(h.api, h.remoteName, 0)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			h.log.Println("IPNSHandler#List.ref.path == ", ref.path)
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

func (h *IpnsHandler) Push(remote *core.Remote, local string, remoteRef string) (string, error) {
	h.didPush = true

	remote.Logger.Println("IpnsHandler#Push.local == ", local)

	localRef, err := remote.Repo.Reference(plumbing.ReferenceName(local), true)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	headHash := localRef.Hash().String()

	push := remote.NewPush()
	remote.Logger.Println("IpnsHandler.Push#PushHash: ", headHash)
	shunt, err := push.PushHash(headHash, remote)
	remote.Logger.Println("IpnsHandler.Push#shunt == ", shunt)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	keyedShunts, _ := h.api.List(shunt)
	shunts := make(map[string]string)
	for _, s := range keyedShunts {
		shunts[s.Name] = s.Hash
	}

	head, err := remote.Repo.Head()
	commit, err := remote.Repo.CommitObject(head.Hash())
	tree, err := commit.Tree()
	files := tree.Files()
	for leaf, _ := files.Next(); leaf != nil; leaf, _ = files.Next() {
		refId, ok := shunts[leaf.Hash.String()]
		if refId == "" {
			refId, err = remote.Tracker.Entry(leaf.Hash.String())
			ok = err == nil
		}
		remote.Logger.Printf("Remote.repo %s => %s (%s)\n", leaf.Name, leaf.Hash, refId)
		if ok {
			h.currentHash, err = h.api.PatchLink(h.currentHash, "content/" + leaf.Name, refId, true)
		} else {
			remote.Logger.Println("Couldn't Find Blob: ", leaf.Hash)
		}
	}

	h.currentHash, err = h.api.PatchLink(h.currentHash, "blobs", shunt, true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	hash := localRef.Hash()

	remote.Logger.Println("IpnsHandler.Push#localRef.Hash() == ", hash)
	
	remote.Tracker.AddEntry(hash.String(), shunts[hash.String()])

	c, err := core.CidFromHex(headHash)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	remote.Logger.Println("IpnsHandler.Push#cid == ", c)

	//patch object
	remote.Logger.Printf("ipfs/object/patch/add-link %s %s %s %b\n", h.currentHash, remoteRef, c, true)

	h.currentHash, err = h.api.PatchLink(h.currentHash, remoteRef, c.String(), true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	remote.Logger.Println("Post Patch currentHash == ", h.currentHash)

	gotHead, err := h.getRef("HEAD")
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	if gotHead == "" {
		headRef, err := h.api.Add(strings.NewReader("refs/heads/master")) //TODO: Make this smarter?
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}

		h.currentHash, err = h.api.PatchLink(h.currentHash, "HEAD", headRef, true)
		if err != nil {
			return "", fmt.Errorf("push: %v", err)
		}
	}

	return local, nil
}

func (h *IpnsHandler) getCid(cid string) (string, error) {
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

func (h *IpnsHandler) getRef(name string) (string, error) {
	return h.getCid(path.Join(h.remoteName, name))
}

func (h *IpnsHandler) paths(api *ipfs.Shell, p string, level int) ([]refPath, error) {
	h.log.Println("IPNSHandler.paths: ", p)
	links, err := api.List(p)
	if err != nil {
		return nil, err
	}
	h.log.Println("IPNSHandler.paths.links: ", links)

	out := make([]refPath, 0)
	for _, link := range links {
		switch link.Type {
		case ipfs.TDirectory:
			if level == 0 && link.Name == LARGE_OBJECT_DIR {
				continue
			}

			h.log.Println("Recursing?: ", link.Name)
			if link.Name == "heads" || link.Name == "refs" {
				sub, err := h.paths(api, path.Join(p, link.Name), level + 1)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
			}
		case ipfs.TFile:
			out = append(out, refPath{path.Join(p, link.Name), REFPATH_REF, link.Hash})
		case -1, 0: //unknown, assume git node
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
