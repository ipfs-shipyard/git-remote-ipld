# Git IPLD remote helper

Push and fetch commits using IPFS!

This helper is experimental as of now

## Usage
```
Clone an example repository:
$ git clone ipld://2347e110c29742a1783134ef45f5bff58b29e40e

Pull a commit:
$ git pull ipld://2347e110c29742a1783134ef45f5bff58b29e40e

Push:
$ git push --set-upstream ipld:// master
```

Note: Some features like remote tracking are still missing, though the plugin is
quite usable. IPNS helper is WIP and doesn't yet do what it should

## Custom API URL

It is possible to pass the API URL of the running ipfs daemon for cases where the
daemon is running as another user, in a container, in another machine, or
behind a proxy. One can either specify the API URL through the `IPFS_API_URL`
environment variable:

```
$ export IPFS_API_URL=http://localhost:5001

other git commands...
```

Or through the remote URL itself:

```
Clone an example repository:
$ git clone ipld://localhost:5001/2347e110c29742a1783134ef45f5bff58b29e40e

Pull a commit:
$ git pull ipld://localhost:5001/2347e110c29742a1783134ef45f5bff58b29e40e

Push (remember to end with slash if there is no previous hash):
$ git push --set-upstream ipld://localhost:5001/ master
```

If the API URL is **behind an https proxy**, then one can set `IPFS_API_URL` with the proper https URL. It is also possible to use the `ipld+https://` schema on the git command itself:

```
Clone an example repository:
$ git clone ipld+https://my.ipfs.domain/my_api/2347e110c29742a1783134ef45f5bff58b29e40e

Pull a commit:
$ git pull ipld+https://my.ipfs.domain/my_api/2347e110c29742a1783134ef45f5bff58b29e40e

Push (remember to end with slash if there is no previous hash):
$ git push --set-upstream ipld+https://my.ipfs.domain/my_api/ master
```

**Note:** The link `git-remote-ipld+https` must be created manually for this to work.

## Installation
1. `go get github.com/ipfs-shipyard/git-remote-ipld`
2. `make install`
3. Done
5. Make sure you run go-ipfs 0.4.17 or newer as you need git support

## Limitations / TODOs
* ipns remote is not implemented fully yet

# Troubleshooting
* `fetch: manifest has unsupported version: 2 (we support 3)` on any command
  - This usually means that tracker data format has changed
  - Simply do `rm -rf .git/ipld`
* Invalid hash when pushing to a new remote while specifying a custom API URL
  - The remote URL **must** end with a forward slash if there is no hash specified:
  - `git push --set-upstream ipld://127.0.0.1:5001/ master`

## License
MIT
