# solitary

**Hypervisor-isolated cells for running coding agents off the leash.**

> ⚠️ Pre-alpha. Cells build, run, persist and take secrets. Egress from a cell
> is not restricted yet — see [what this does not protect against](#what-this-does-not-protect-against).

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
- **Ports are the way in.** Run a dev server in the cell, open it in your
  browser on the host. Traffic the other way — a cell reaching your machine or
  your network — is not blocked yet.

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

command: sleep infinity   # optional; must not exit — it is the container's life

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
on the host and is never copied into the VM — they are passed to the container
as environment variables when it starts. The file may hold more than a cell
needs; only the names that cell declares are ever passed in. `up` asks for any
that are missing, and `solitary secrets <name>` sets or rotates them later.

Because the values live on the host, `rm` followed by `up` gives you a clean
cell that is still authenticated.

`~/.config/solitary/config.yaml` holds a `vm:` block used as the default for
every cell.

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
It also replaces the container when the image or the secrets changed, so
editing `cell.yaml` and running `up` again is all a change ever takes.

Work belongs in `/home/cell`, which lives on the machine's disk rather than in
the container. It survives a new image, a stop and start, and anything an editor
installs into the home directory. `rm` is what discards it.

Creating a cell takes a couple of minutes: it downloads a cloud image and
installs podman. Everything after that is container-speed.

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

## How it works

`up` renders an embedded Lima template into a machine definition, creates the
machine, and starts one rootless podman container inside it. Both are driven by
shelling out — `limactl` on the host, `podman` through `limactl shell` in the
machine — so there is nothing to install beyond Lima.

The container runs with `--network host`: the machine is the boundary, so there
is no reason to put a second one between the container and the machine it lives
in. Ports reach the host through Lima's forwarding.

The container's identity is recorded in two labels: the image as written in
`cell.yaml`, and a digest of the environment it was started with. `up` compares
both and replaces the container when either has moved, which is why changing a
secret or an image needs no separate command.

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
