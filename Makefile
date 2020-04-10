all:
	go build -o cmd/git-remote-ipfs/git-remote-ipfs ./cmd/git-remote-ipfs/...

test:
	go test -v ./...

.PHONY: all test
