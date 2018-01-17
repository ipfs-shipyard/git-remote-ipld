all: deps
	go build -o cmd/git-remote-ipld/git-remote-ipld ./cmd/git-remote-ipld/...
	go build -o cmd/git-remote-ipns/git-remote-ipns ./cmd/git-remote-ipns/...

gx:
	go get github.com/whyrusleeping/gx
	go get github.com/whyrusleeping/gx-go

deps: gx
	gx --verbose install --global
	gx-go rewrite
	go get ./...

install: all
	go install -v ./cmd/git-remote-ipld/...
	go install -v ./cmd/git-remote-ipns/...

.PHONY: all gx deps install
