<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/assets/logo-dark.svg">
    <img src=".github/assets/logo.svg" alt="" width="72" height="72">
  </picture>
  <br>
  Solitary
</h1>

<p align="center">
  <a href="https://solitary.balakin.io/docs"><img alt="Documentation" src="https://img.shields.io/badge/docs-solitary.balakin.io-blue"></a>
  <a href="https://github.com/balakin/solitary/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/balakin/solitary?sort=semver"></a>
  <a href="https://github.com/balakin/homebrew-solitary"><img alt="Homebrew" src="https://img.shields.io/badge/brew-balakin%2Fsolitary-orange"></a>
  <br>
  <a href="https://github.com/balakin/solitary/actions/workflows/verify.yml"><img alt="Verify" src="https://img.shields.io/github/actions/workflow/status/balakin/solitary/verify.yml?branch=main&label=verify"></a>
  <a href="https://github.com/balakin/solitary/actions/workflows/pages.yml"><img alt="Docs deploy" src="https://img.shields.io/github/actions/workflow/status/balakin/solitary/pages.yml?branch=main&label=docs%20deploy"></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/balakin/solitary"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/balakin/solitary"></a>
</p>

**Run coding agents on a VM, not on your machine.**

> Early. The `0.x` line can still change the shape of a cell's configuration —
> read [the limitations](https://solitary.balakin.io/docs/limitations) before
> trusting a cell with credentials.

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
  GITHUB_TOKEN:

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

## Install

Either one, on macOS or Linux:

```sh
curl -fsSL https://solitary.balakin.io/install.sh | sh
```

```sh
brew install balakin/solitary/solitary
```

Binaries for macOS and Linux are attached to every
[release](https://github.com/balakin/solitary/releases/latest) too, and
`solitary update` replaces the binary with the newest one.

To remove it again, `curl -fsSL https://solitary.balakin.io/uninstall.sh | sh`
(or `brew uninstall solitary`); it keeps the cells unless told otherwise. See
the [installation guide](https://solitary.balakin.io/docs/installation) for the
rest.

Requirements:

- [Lima](https://lima-vm.io) 2.0 or newer
- macOS or Linux
- QEMU 11 or newer with Venus and OpenGL support for `gpu: true` on Linux

## Quick start

An editor in a cell, from nothing:

```sh
solitary clone balakin/solitary/examples/vscode
solitary up vscode --detach
open http://localhost:9797
```

That is VS Code with Claude Code beside it, running in the cell and reached
through a port forwarded to your host's localhost only. `clone` shows you what
the definition asks for before installing it; `up` builds the image **inside**
the machine, so nothing in that `Containerfile` runs on your host. Then
`solitary shell vscode` for a shell in the same cell, `solitary rm vscode` when
you are done.

Write your own with `solitary init <name>` — see the
[quickstart](https://solitary.balakin.io/docs/quickstart).

## Documentation

Everything else — the configuration reference, the command surface, networking,
secrets, the security model — is at
**[solitary.balakin.io](https://solitary.balakin.io/)**.

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
