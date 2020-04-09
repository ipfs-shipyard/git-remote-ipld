# Git IPLD remote helper

Push and fetch commits using IPFS!

This helper is experimental as of now

## Usage
```
Clone an example repository:
$ git clone ipld://QmdwpxCnSVRmQotUZBgNfBjuTgDdARpsq3wYqcM6mgTvNF

Pull a commit:
$ git pull ipld://2347e110c29742a1783134ef45f5bff58b29e40e

Push:
$ git push --set-upstream ipld:// master
```

Note: Some features like remote tracking are still missing, though the plugin is
quite usable. IPNS helper is WIP and doesn't yet do what it should.

## Installation
1. `git clone https://github.com/dhappy/git-remote-ipld.git`
2. `cd git-remote-ipld`
3. `make`
4. `sudo cp cmd/git-remote-ipld/git-remote-ipld /usr/lib/git-core/`

## Limitations / TODOs
* ipns remote is not implemented fully yet
* it takes _forever_ to run

# Troubleshooting
* `fetch: manifest has unsupported version: x (we support y)` on any command
  - This usually means that tracker data format has changed
  - Simply do `rm -rf .git/ipld`

## License
MIT
