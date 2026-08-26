# solitary

**Hypervisor-isolated cells for running coding agents off the leash.**

> Early. The `0.x` line can still change the shape of a cell's configuration —
> read [the limitations](website/content/docs/limitations.mdx) before trusting a
> cell with credentials.

## The problem

Most tools that sandbox a coding agent hand it a container and mount your
project directory into it. That boundary does not hold, and it does not take a
kernel exploit to break — it takes a file. An agent that can write to a mounted
directory can write to `.git/hooks/pre-commit`, a `package.json` script, a
`Makefile` or an `.envrc`. Then you run `npm test` on your machine, and the
payload runs as you. No escape was necessary; you executed it yourself.

## The approach

A cell is a Lima VM with a container inside it.

- **The VM is the boundary** — a hypervisor, not a shared kernel, so the
  isolation holds even if the agent gets root in its container.
- **The container is the toolset.** Change `image:`, run `up` again: same VM,
  same disk, same secrets, different tools.
- **Nothing is mounted from the host, ever.** Clone, build, lint and test inside
  the cell; the host only displays results.
- **Secrets are whitelisted per cell** and stay on the host, so `rm` then `up`
  gives you a clean cell that is still authenticated.
- **Ports are the way in, `network.allow` is the way out.** Set the allow list
  and the cell reaches what you list and nothing else — not the rest of the
  internet, not your machine, not your local network.

```yaml
# ~/.config/solitary/cells/claude/cell.yaml
image: ghcr.io/you/nvim-claude:latest

secrets:
  - GITHUB_TOKEN

ports:
  - 8080

network:
  allow:
    - github.com
    - api.anthropic.com
```

```sh
solitary up claude
```

## Documentation

The docs are in [`website/content/docs`](website/content/docs) and are the site
built from `website/`. Start with the [quickstart](website/content/docs/quickstart.mdx).

| | |
| --- | --- |
| [Core concepts](website/content/docs/concepts.mdx) | What a cell is made of, and which layer a change lands in |
| [Configuration](website/content/docs/configuration.mdx) | Every field in `cell.yaml` and `config.yaml` |
| [Commands](website/content/docs/commands.mdx) | The whole command surface |
| [Networking](website/content/docs/networking.mdx) | Egress control, DNS, ports and VPN routing |
| [Moving work in and out](website/content/docs/artifacts.mdx) | `fetch`, `send`, and the cell's own `artifact` |
| [Sharing cells](website/content/docs/sharing.mdx) | Publish a definition; take one out of someone's repository |
| [Example cells](website/content/docs/guides-examples.mdx) | Two complete cells in [`examples/`](examples/), ready to clone |
| [The dashboard](website/content/docs/dashboard.mdx) | Every cell and its state in one live view |
| [Security model](website/content/docs/security.mdx) | What the boundary protects, and what it leaves to you |
| [Limitations](website/content/docs/limitations.mdx) | What this does not protect against |
| [Troubleshooting](website/content/docs/troubleshooting.mdx) | The failure modes a cell actually has |

## Install

```sh
curl -fsSL https://solitary.balakin.io/install.sh | sh   # macOS and Linux
brew install balakin/solitary/solitary                   # macOS and Linux
```

Binaries for macOS and Linux are attached to every
[release](https://github.com/balakin/solitary/releases/latest) too, and
`solitary update` replaces the binary with the newest one. See the
[installation guide](website/content/docs/installation.mdx) for the rest.

Requirements:

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

## License

Apache 2.0
