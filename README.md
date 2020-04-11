# Git IPFS Remote Helper

Push and fetch commits to IPFS!

__This software is in an alpha state. The base repository structure is relatively stable and repositories should be backward compatible.__

## Installation
1. `git clone https://github.com/dhappy/git-remote-ipfs.git`
2. `cd git-remote-ipfs`
3. `make`
4. `sudo cp cmd/git-remote-ipfs/git-remote-* /usr/lib/git-core/`

## Usage

Push:

`git push ipvfs:// master`

Clone an example repository:

`git clone ipfs://QmR6Yt8MgjXKPzKQhxuBAw8yM8qScxHXABh9RAuBEosuvj git-remote-ipfs`

Pull a commit:

`git pull ipfs://QmawUDKvygz3kxVusjLVf3AVGuqcPBAneCjbTTeHjbarfH`

Push without the `/vfs` directory:

`git push ipfs:// master`

## Generated File Structure

* `content/`: the contents of the branch that was pushed
* `vfs/commits/`: all the trees linked by commit message and sorted by date
* `vfs/commits/rev/`: the commits as before, but prefaced with a count to reverse the order
* `vfs/authors/#{name}/`: commits sorted by author
* `vfs/authors/#{name}/rev/`: commits sorted by author reversed chronologically
* `blobs/`, `trees/`, `commits/`, `tags/`: various Git objects stored by their SHA1 hash as filename
* `refs/heads/*`: files containing the root hash of various Git branches
* `HEAD`: the name of the branch contained in this repo

## Overview

Git is at its core an object database. There are four types of objects and they are stored by the SHA1 hash of the serialized form in `.git/objects`.

When a remote helper is asked to push, it receives the key of a Commit object. That commit has an associated Tree and zero or more Commit parents.

Trees are lists of umasks, names, and keys for either nested Trees or Blobs.

Tags are named links to specific commits. They are frequently used to mark versions.

The helper traverses the tree and parents of the root Commit and stores them all on the remote.

Integrating Git and IPFS has been on ongoing work with several solutions over the years. This one is based on [git-remote-ipld](https://github.com/ipfs-shipyard/git-remote-ipld) which `ipfs block put` to store the Commit tree using SHA1. Fetching converts the SHA1 keys in the raw git blocks to IPFS content ids and retrieves them directly.

The SHA1 keys used by Git aren't exactly the hash of the object. Each serialized form is prefaced with a header of the format `"#{type} #{size}\x00". So a Blob in Git is this header plus the file contents.

Because this system stores the raw Git blocks, the file data is fully present, but unreadable because of the header.

There were also technical issues because the contents of a `block put` aren't sharded and there are reliability problems with large blocks.

This project mitigates that issue by leaving off the header and storing the simple serialized form in a named directory. The filename is the SHA1 key for the file contents.

When fetching, a map is created between the SHA1 keys and their CID along with the type. A traditional headered block can then be generated.

# Troubleshooting
* `fetch: manifest has unsupported version: x (we support y)` on any command
  - This usually means that cache tracker data format has changed
  - Remove the cache with: `rm -rf .git/remote-ipfs`

# License
MIT
