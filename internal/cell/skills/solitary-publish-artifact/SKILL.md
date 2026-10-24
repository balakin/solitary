---
name: solitary-publish-artifact
description: Hand a file out of a solitary cell to the user, the only way anything leaves one. For an agent running inside a cell, where SOLITARY_CELL is set. Use when the user asks for a file, a report, a patch, a build, an export or a screenshot to keep, or when work produced something they will want on their own machine.
---

# Publish a file from this cell

This is a solitary cell: a virtual machine with nothing from the user's machine mounted
in it. A file you write here stays here. To hand one over, publish it to the outbox:

```sh
artifact report.pdf          # publish a copy
artifact dist.tar.gz notes.md
artifact --list              # what is published, and what is waiting in the inbox
```

Publishing copies the file into `~/outbox` and sends nothing anywhere. The user collects
it on their machine whenever they choose. Tell them the exact command, using the cell's
name from `$SOLITARY_CELL`:

```sh
solitary fetch <cell> report.pdf   # e.g. solitary fetch claude report.pdf
solitary fetch <cell>              # everything published
```

Run `echo "$SOLITARY_CELL"` to get the name rather than guessing it.

## Rules the host enforces

- **Only regular files, one level deep.** A directory isn't published and a symlink
  isn't followed. Archive a directory first: `tar czf site.tar.gz site/`, then
  `artifact site.tar.gz`.
- **Plain file names only.** A name starting with `-`, or `.` and `..`, is refused. Rename
  the file before publishing it.
- **Nothing arrives executable.** Fetched files land as `0644` data. If the user needs to
  run a script, say so; they'll make it executable themselves.
- **Nothing is overwritten without asking.** A fetch refuses to replace a file the user
  already has unless they pass `--force`, so publishing under the same name again is
  safe.

## Don't

- Don't publish anything holding a secret. The cell's credentials are environment
  variables; never write their values into a file you publish.
- Don't try to copy files out any other way (scp, curl to a paste service, a git push
  the user didn't ask for). The outbox is the channel the user chose.
- Don't empty the outbox unless the user asks. Old files are harmless, and `rm` in
  `~/outbox` is how it is cleared.
