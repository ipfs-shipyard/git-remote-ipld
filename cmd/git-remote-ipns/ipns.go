package main

import (
	"log"

	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/magik6k/git-remote-ipld/core"
)

type IpnsHandler struct {
	remoteName string
}

func (h *IpnsHandler) List(remote *core.Remote, forPush bool) ([]string, error) {
	h.paths(h.remoteName)

	return nil, nil
}

func (h *IpnsHandler) Push(remote *core.Remote, local string, remoteRef string) ([]string, error) {
	return nil, nil
}

func (h *IpnsHandler) paths(path string) ([]string, error) {
	api := ipfs.NewLocalShell()

	links, err := api.List(path)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		log.Println(link.Name)
		log.Println(link.Type)
	}
	return nil, nil
}
