# Git IPLD remote helper

Push and fetch commits using IPFS!

This helper is experimental as of now

## Usage
```
Clone:
$ git clone ipld::20dae521ef399bcf95d4ddb3cefc0eeb49658d2a

Pull:
$ git pull ipld::20dae521ef399bcf95d4ddb3cefc0eeb49658d2a

Push:
$ git push ipld::
```

Note: Some features like remote tracking are still missing, though the plugin is
quite usable.

## Installation
1. goget/clone/download the repo
2. `make install`
3. Done
4. ???
5. You will need IPFS with Git plugin installed, see https://github.com/ipfs/go-ipfs/blob/master/docs/plugins.md

## License
MIT
