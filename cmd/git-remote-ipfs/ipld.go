package main

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"log"
	"os"
	"time"
	"os/exec"
	"regexp"

	core "github.com/dhappy/git-remote-ipfs/core"
	ipfs "github.com/ipfs/go-ipfs-api"

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
	if h.didPush {
		if !strings.HasPrefix(h.remoteName, "key:") {
			remote.Log.Printf("Pushed to IPFS as \x1b[32mipfs://%s\x1b[39m\n", h.currentHash)
		} else {
			key := h.remoteName[4:]
			cmd := exec.Command("ipfs", "name", "publish", "--key=" + key, h.currentHash)

			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			remote.Log.Printf("Publishing \x1b[35m%s\x1b[39m to \x1b[36m%s\x1b[39m\n", h.currentHash, key)
			err := cmd.Run()
			if err != nil {
				return err
			}

			re := regexp.MustCompile(`Published to (.+):`)
			ipnsAddr := re.FindStringSubmatch(out.String())[1]
			remote.Log.Printf("Pushed to IPNS as \x1b[32mipns://%s\x1b[39m\n", ipnsAddr)
		}
	}
	return nil
}

func (h *IPFSHandler) GetRemoteName() string {
	return h.remoteName
}

func (h *IPFSHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	out := make([]string, 0)
	if !forPush {
		_, _, err := h.api.BlockStat(fmt.Sprintf("%s/.git/", h.remoteName))
		if err == nil {
			h.log.Printf("Setting GIT DIR: %s → %s\n", h.remoteName, ".git")
			h.remoteName += "/.git"
		}

		h.log.Printf("Getting HEAD Ref:", h.remoteName)
		head, err := h.getCid(fmt.Sprintf("%s/HEAD", h.remoteName))
		out = append(out, fmt.Sprintf("@%s HEAD", head))

		heads, err := h.api.List(fmt.Sprintf("%s/refs/heads/", h.remoteName))
		for _, head := range heads {
			hash, _ := h.getCid(head.Hash)
			out = append(out, fmt.Sprintf("%s refs/heads/%s", hash, head.Name))
		}
	} else {
		it, err := remote.Repo.Branches()
		if err != nil {
			return nil, err
		}

		err = it.ForEach(func(ref *plumbing.Reference) error {
			remoteRef := "0000000000000000000000000000000000000000"
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
	gitRef, err := push.PushHash(root.String(), remote)
	if err != nil {
		return "", fmt.Errorf("command push: %v", err)
	}

	commit, _ := remote.Repo.CommitObject(root)
	c, _ := h.cidForCommit(commit, remote)

	h.currentHash, err = h.api.PatchLink(c, ".git", gitRef, true)

	exe, _ := os.Executable()
	if os.Getenv("GIT_IPFS_VFS") != "" || exe[(len(exe) - 6):] == "-ipvfs" {
		depth := 0
		object.NewCommitPreorderIter(commit, nil, nil).ForEach(func(commit *object.Commit) error {
			c, _ := h.cidForCommit(commit, remote)
			depth += 1
			_, err := h.placeCommitCID(commit, c, depth)
			return err
		})
	}
	h.log.Println()

	hashHolder, err := h.api.Add(strings.NewReader(root.String()))
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	h.currentHash, err = h.api.PatchLink(h.currentHash, ".git/" + remoteRef, hashHolder, true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	headRef, err := h.api.Add(strings.NewReader(remoteRef))
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}
	h.currentHash, err = h.api.PatchLink(h.currentHash, ".git/HEAD", headRef, true)
	if err != nil {
		return "", fmt.Errorf("push: %v", err)
	}

	return local, nil
}

func (h *IPFSHandler) fileSafeName(name string) string {
	name = strings.ReplaceAll(name, "%", "%25")
	name = strings.ReplaceAll(name, "\x00", "%00")
	return strings.ReplaceAll(name, "/", "%2F")
}

func (h *IPFSHandler) placeCommitCID(commit *object.Commit, c string, commitNum int) (string, error) {
	// Pretty much the only disallowed character is / which will create a subdirectory
	// %s are encoded so strings can be reliably percent decoded
	when := commit.Author.When.Format(time.RFC3339)
	message := strings.Split(commit.Message, "\n")[0]
	entry := h.fileSafeName(fmt.Sprintf("%s: %s – %s", when, commit.Author.Name, message))

	h.log.Printf("Adding: %s → %s\r\x1b[A", entry, c)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, ".git/vfs/commits/" + entry, c, true)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, fmt.Sprintf(".git/vfs/rev/commits/%020d: %s", commitNum, entry), c, true)

	entry = h.fileSafeName(fmt.Sprintf("%s: %s", when, message))
	name := h.fileSafeName(commit.Author.Name)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, fmt.Sprintf(".git/vfs/authors/%s/%s", name, entry), c, true)
	h.currentHash, _ = h.api.PatchLink(h.currentHash, fmt.Sprintf(".git/vfs/rev/authors/%s/%020d: %s", name, commitNum, entry), c, true)

	h.currentHash, _ = h.api.PatchLink(h.currentHash, fmt.Sprintf(".git/vfs/trees/%s", commit.Hash.String()), c, true)

	return h.currentHash, nil
}

func (h *IPFSHandler) cidForCommit(commit *object.Commit, remote *core.Remote) (string, error) {
	tree, _ := commit.Tree()
	files := tree.Files()
	c := "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
	for leaf, _ := files.Next(); leaf != nil; leaf, _ = files.Next() {
		refId, err := remote.Tracker.Entry(leaf.Hash.String())
		if err == nil && refId != "" {
			c, err = h.api.PatchLink(c, leaf.Name, refId, true)
			if err != nil {
				return "", fmt.Errorf("cidForCommit: %v", err)
			}
		} else {
			remote.Log.Println("Couldn't Find Blob: ", leaf.Hash)
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

func isNoLink(err error) bool {
	return strings.Contains(err.Error(), "no link named") || strings.Contains(err.Error(),
"no link by that name")
}
