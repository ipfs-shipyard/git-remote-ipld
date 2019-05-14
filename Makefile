all:
	go build -o cmd/git-remote-ipld/git-remote-ipld ./cmd/git-remote-ipld/...

test:
	go test -v ./...

.PHONY: all test
