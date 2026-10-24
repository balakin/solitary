---
name: solitary-cell-environment
description: How a solitary cell works, for an agent running inside one (SOLITARY_CELL is set) — what survives a restart, where to keep work, how credentials arrive, and how the user reaches a server the cell runs. Use when installing tools, starting a dev server or web app, handling credentials or sign-in links, or when the user asks where something went after a restart.
metadata:
  internal: true
---

# Working inside a solitary cell

`$SOLITARY_CELL` is set, so you're in a solitary cell named by its value: a virtual
machine on the user's computer, with one container in it, and you're in the container.
The user drives it from their machine with the `solitary` command.

## What lasts

- **The home, `/home/cell`, lasts.** It's on the machine's disk, and it survives a
  restart, a new image and a rebuild. Keep work, clones and anything you install for
  yourself (`~/.local`, a language's per-user directory) in it.
- **Everything outside the home is reset at every restart.** The container is started
  over from its image each time the machine boots. A package installed with `apt`, or a
  tool updated outside the home, is gone afterwards.
- **To keep a tool,** the user adds it to the cell's image (the `Containerfile` beside its
  `cell.yaml`) and rebuilds with `solitary up --rebuild <cell>`. Say that rather than
  reinstalling the tool at every start.
- **`solitary rm` destroys the home too.** Anything the user wants to keep should be
  pushed to a git remote or published with `artifact` (see the `solitary-publish-artifact`
  skill).

## Nothing is shared with the user's machine

Nothing from the user's computer is mounted here: no repository, no `~/.ssh`, no
clipboard. Files come in through `~/inbox` and go out through `artifact`. If something
the user mentions isn't here, ask them to send it rather than searching for it.

## Credentials

Secrets the cell is allowed reach it as environment variables, set when the container
starts. Use them from the environment. Never write their values into files, logs, commit
messages or anything you publish. A credential that isn't set wasn't granted to this
cell: say which one is missing, and tell the user they add it to `secrets:` in
`cell.yaml` and set it with `solitary secrets <cell>`.

The git identity comes from the environment too, so commits are attributed correctly
without configuring git.

## Servers and sign-in

- **A server you start can usually be reached on the user's machine.** The ports the
  container listens on are forwarded to `localhost` there, unless the cell's definition
  limits them under `ports:`. Give the user the URL, such as `http://localhost:3000`, and
  bind to `0.0.0.0` or `127.0.0.1` as the tool expects.
- **A cell can't open a browser.** For a sign-in that prints a URL, show the URL and ask
  the user to open it on their machine.

## The network may be restricted

A DNS error for an ordinary public name usually means the cell's allow list refused it.
See the `solitary-cell-network` skill.
