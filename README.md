# solitary

**Hypervisor-isolated cells for running coding agents off the leash.**

> ⚠️ Pre-alpha. This repository is a scaffold: the command tree exists, none of
> it is implemented. Nothing here works yet.

## The problem

Most tools that sandbox a coding agent hand it a container and mount your
project directory into it. That boundary does not hold, and it does not take a
kernel exploit to break — it takes a file.

An agent that can write to a mounted directory can write to `.git/hooks/pre-commit`,
a `package.json` script, a `Makefile`, an `.envrc`, a `.vscode/tasks.json`, or a
lint plugin resolved from the local tree. Then you run `npm test` or `git commit`
on your machine, and the payload runs as you. No escape was necessary; you
executed it yourself.

## The approach

A cell is a Lima VM with a container inside it.

- **The VM is the boundary.** A hypervisor, not a shared kernel — the isolation
  is real even if the agent gets root in its container.
- **The container is the toolset.** Change `image:`, run `up` again: same VM,
  same disk, same secrets, different tools, in seconds.
- **Nothing is mounted from the host.** Clone, build, lint, test and review
  inside the cell. The host only ever displays results.
- **Secrets are whitelisted per cell.** This cell sees a GitHub token; that one
  does not.
- **Only ports cross the boundary**, and only inbound: run a dev server in the
  cell, open it in your browser on the host.

Cells are meant to be thrown away. `rm` then `up` gets you a clean one, still
authenticated, because secrets live on the host and not in the VM.

### What this does not protect against

Isolation stops a compromise of your machine. It does not stop an agent
misusing the authority you granted it. It can still push to any repository the
token you whitelisted can reach, and until egress filtering lands it can still
send data anywhere. The image you run is trusted code.

## Configuration

`~/.config/solitary/cells/<name>/cell.yaml` — the cell's name is the directory
name, never a field inside the file.

```yaml
image: ghcr.io/you/nvim-claude:latest

secrets:            # only these names are passed into the cell
  - CLAUDE_API_KEY
  - GITHUB_TOKEN

ports:              # omit and every listening port reaches host localhost;
  - 8080            # set and only these do

vm:                 # optional; falls back to config.yaml, then to built-ins
  cpus: 4
  memory: 8GiB
```

Values for the names under `secrets:` live in `cells/<name>/.env`, which stays
on the host and is never copied into the VM. `~/.config/solitary/config.yaml`
holds a `vm:` block used as the default for every cell.

## Commands

```
solitary init <name>            scaffold a cell definition
solitary up <name|image-ref>    start the cell and attach
solitary shell <name>           shell into a running cell
solitary down <name>            stop the cell, keep the disk
solitary rm <name>              destroy the VM; definition and secrets stay
solitary ls                     list cells and their state
solitary secrets <name>         set the values a cell is allowed to see
```

`up` is the only command that changes state, and it is idempotent: it creates
the cell if absent, boots it if stopped, and attaches if it is already running.
Passing an image reference instead of a known name creates a cell from it with
default settings — useful for demos.

## Requirements

- [Lima](https://lima-vm.io) 2.0 or newer
- macOS or Linux

## Development

Go is not vendored; fetch dependencies first.

```sh
go mod tidy
make build
make lint
```

Formatting is `gofumpt` plus `goimports` via `golangci-lint fmt`. Commit
messages follow Conventional Commits.

## Roadmap

- `network: allowlist` — default-deny egress with a domain allowlist, modelled
  on Anthropic's [`init-firewall.sh`](https://github.com/anthropics/claude-code/blob/main/.devcontainer/init-firewall.sh),
  later replaced by an SNI-filtering proxy so IP churn stops mattering. The
  firewall runs in the VM, outside the rootless container, so the agent cannot
  disable it.
- Blocking VM→host and VM→LAN traffic while keeping host-initiated connections.
- Building a cell image from a local `Containerfile`.
- Distributing cell definitions as OCI artifacts.
- VM snapshots, so a new cell does not reinstall podman from scratch.

## License

Apache 2.0
