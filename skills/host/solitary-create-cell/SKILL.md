---
name: solitary-create-cell
description: Write a solitary cell definition and bring the cell up — a Lima VM with one container in it, for running a coding agent or any tool isolated from this machine. For the host, where the solitary CLI is installed, not for an agent inside a cell. Use when the user wants a new cell, wants an agent or a project sandboxed, asks to write or change a cell.yaml, or wants to take an example cell.
---

# Create a solitary cell

A cell is a virtual machine with one rootless container inside it. The machine is the
boundary; the container is the toolset, and can be replaced without losing the cell's home.
Nothing from the host is ever mounted into a cell, and secrets stay on the host until the
container starts.

Every command below is the `solitary` CLI, run on the host. Run `solitary <command> --help`
when a flag is in doubt rather than guessing one.

## 1. Check the ground

```sh
solitary doctor   # Lima, the hypervisor, memory and disk; run it before a first cell
solitary ls       # the cells that already exist
```

A cell is named by its directory, so a name that `ls` already shows — even as `orphaned` —
reuses that machine and its disk. Pick a new name unless reusing it is the point. Names are
lowercase letters, digits and dashes.

## 2. Start from an example when one fits

```sh
solitary clone balakin/solitary/examples/claude            # Claude Code, neovim, tmux, gh, node
solitary clone balakin/solitary/examples/vscode            # code-server in the browser, plus the same
solitary clone balakin/solitary/examples/claude --as work  # under a name of your own
```

`clone` shows what the definition asks for and asks before installing it. Don't pass
`--yes` on the user's behalf: that question is theirs to answer. Then edit the copy under
`~/.config/solitary/cells/<name>/` like any other cell.

## 3. Otherwise scaffold one

```sh
solitary init <name>   # writes ~/.config/solitary/cells/<name>/cell.yaml, commented
```

Edit that file. Find out what the user wants in the cell before writing it — the tools, the
credentials, what it must reach — rather than filling it in from guesses.

```yaml
description: >- # for someone who has not met the cell yet; at most 350 characters
  Go and Claude Code, reaching github and the Go module proxy.

build: ./Containerfile # or image: ghcr.io/owner/name:tag — exactly one of the two

secrets: # names only; values never go in this file
  ANTHROPIC_API_KEY:
  GITHUB_TOKEN:
    required: false
    description: pushing branches and opening pull requests

ports: # omit and every port the container listens on reaches host localhost
  - 8080

vm: # optional; defaults are 2 cpus, 4GiB, 20GiB
  cpus: 4
  memory: 8GiB
```

- **`image` or `build`.** Use `image` for a published image that already has the tools. Use
  `build` for a `Containerfile` beside `cell.yaml`: the cell's directory is the build context,
  and the build runs inside the machine, never on the host.
- **`command`** defaults to `sleep infinity`. If you set it, it must never exit: it is the
  container's life, and the shells people open are separate from it.
- **`user`.** Leave it out. Everything runs as root in the container, and that root is an
  unprivileged user on the machine. Set it only when the image serves a login of its own (an
  `sshd`, an editor server) or a tool refuses to run as root.
- **`secrets`.** A name on its own is required, and the cell won't start without it. Mark
  credentials the cell can work without `required: false`.
- **`network`.** Leave it out on the first `up`. A missing name in an allow list fails the
  image build itself, so get the cell working first and then restrict it. The
  `solitary-restrict-cell-network` skill does that part.
- **`vm.memory`.** On Linux it has to fit in `/dev/shm`, which is usually half of RAM.
  `doctor` says whether it fits.
- **`devices`** and **`gpu`** hand the cell more authority. Add them only when asked for.

Settings the user wants in every cell, such as `git: {name, email}` and `vm` defaults, belong
in `~/.config/solitary/config.yaml` rather than in each cell.

## 4. Secrets are the user's to type

Never ask for a secret's value in the conversation, and never write the cell's `.env`
yourself. Ask the user to run this in their own terminal:

```sh
solitary secrets <name>
```

It prompts for each declared name without echoing what is typed. `up` asks for missing values
too, but only on a terminal; run without one, it fails with `no terminal to prompt on`.

## 5. Bring it up

```sh
solitary up --detach <name>
```

Use `--detach`. Without it, `up` attaches an interactive shell when it is done. The first
`up` creates the machine, patches it and builds or pulls the image, which takes several
minutes. That's normal, so let it run rather than interrupting it.

Check it:

```sh
solitary ls                      # the cell should be running
solitary exec <name> uname -a    # run one command inside it
```

The user then works in it with `solitary shell <name>`, or with `solitary up <name>`, which
attaches.

## Changing a cell later

Edit `cell.yaml` and run `up` again. What that costs depends on the layer the setting is in:

| Settings                                                    | Applied by                                                  |
| ----------------------------------------------------------- | ----------------------------------------------------------- |
| `image`, `build`, `command`, `secrets`, `user`, `devices`, `git` | `up`, which replaces the container and keeps the home  |
| `vm`, `ports`, `network`, `gpu`                             | the next boot: `solitary down <name> && solitary up <name>` |
| whatever `vm.provision` already did                         | `solitary up --new-machine <name>`, which carries the home  |

`up --rebuild <name>` rebuilds the image without a cache, or pulls it again. That is how the
tools in a cell are updated.

## Don't

- Don't run `solitary rm` or `up --new-machine` without the user asking. `rm` destroys the
  machine and the cell's home, and anything not pushed or fetched out is gone.
- Don't edit anything Lima keeps, or drive the machine with `limactl`. Everything a cell
  needs belongs in `cell.yaml`, and `up` puts the machine back to what it describes.
- Don't try to mount or sync a host directory into a cell. There is no way to, by design.
  Files go in with `solitary send <name> <file>` and come out with `solitary fetch <name>`.

The full reference is at https://solitary.balakin.io/docs/configuration.
