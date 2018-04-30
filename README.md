# Git IPLD remote helper

Push and fetch commits using IPFS!

This helper is experimental as of now

## Usage
```
Clone an example repository:
$ git clone ipld::2347e110c29742a1783134ef45f5bff58b29e40e

Pull a commit:
$ git pull ipld::2347e110c29742a1783134ef45f5bff58b29e40e

Push:
$ git push --set-upstream ipld:: master
```

Note: Some features like remote tracking are still missing, though the plugin is
quite usable. IPNS helper is WIP and doesn't yet do what it should

## Installation
1. `go get github.com/magik6k/git-remote-ipld`
2. `make install`
3. Done
4. ???
5. You will need IPFS with Git plugin installed, see https://github.com/ipfs/go-ipfs/blob/master/docs/plugins.md

## Limitations / TODOs
* IPFS Limits single block to 2MB, this is limiting maximum file size (trying to
  pull repos with large objects will just make the command hang)
  * This can and will be worked around for ipns remote.
* ipns remote is not implemented fully yet

# Troubleshooting
* `fetch: manifest has unsupported version: 2 (we support 3)` on any command
  - This usually means that tracker data format has changed
  - Simply do `rm -rf .git/ipld`
* `command [...] EOF` or `[...] no parser for format "git" using input type "raw"`
  - You don't have git IPFS plugin properly installed, see step 5 of installation.

## License
MIT
