# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`solitary` is a Go CLI that runs coding agents inside hypervisor-isolated cells. A cell is a Lima VM
with one rootless podman container inside it. Nothing from the host is ever mounted into a cell;
secrets stay on the host and are injected as environment variables at container start.

## Commands

```sh
go mod tidy                     # dependencies are not vendored; run this first in a fresh checkout
make build                      # -> ./solitary, with version baked in via -ldflags
make test                       # go test ./...
make lint                       # golangci-lint run ./...
make fmt                        # golangci-lint fmt (gofumpt + goimports, local prefix github.com/dm-balakin/solitary)

go test ./internal/cell/                        # one package
go test ./internal/lima/ -run TestRender        # one test
UPDATE_GOLDEN=1 go test ./internal/lima/        # rewrite internal/lima/testdata/*.yaml golden files
```

Tests that need a real environment skip rather than fail: `limactl` (Lima schema validation), `git`,
symlink support. Tests that touch the config tree set `HOME`, `XDG_CONFIG_HOME` and `XDG_STATE_HOME`
to a temp dir — follow that pattern, never write to the real config.

Website (docs site, separate toolchain, see `website/README.md`):

```sh
cd website && pnpm install && pnpm dev      # Node 24, pnpm pinned via corepack
pnpm build && pnpm types:check
pnpm lint && pnpm fmt:check                 # oxlint and oxfmt; `pnpm fmt` rewrites
```

oxfmt owns the formatting of everything under `website/` — MDX docs included — so leave it to
rewrite files rather than matching its output by hand.

## CI

`.github/workflows/verify.yml` runs on every push and pull request to `main`, as two jobs that
mirror the commands above: the Go CLI (tidy check, `golangci-lint run`, `golangci-lint fmt --diff`,
`make test`, `make build`) and the website (`pnpm lint`, `fmt:check`, `types:check`, `build`).
Run the same commands locally before pushing rather than using CI to find out.

Actions are pinned to a commit SHA with the tag in a trailing comment; Dependabot moves them, so
change the SHA and the comment together. Dependency updates arrive as one grouped PR per ecosystem
per week (`gomod`, `npm` under `/website`, `github-actions`).

## Architecture

The layering runs host → machine → container, and each package owns one layer:

- `internal/cli` — cobra command tree (`up`, `shell`, `exec`, `down`, `rm`, `ls`, `fetch`, `send`,
  `clone`, `secrets`, `dashboard`, `init`). Commands are thin; the work lives in `internal/cell`.
  `Main` unwraps a `cell.ExitError` so `exec` passes a guest command's exit status through silently.
- `internal/config` — the on-disk formats under `~/.config/solitary`: `config.yaml` (user-wide
  defaults) and `cells/<name>/cell.yaml` plus `cells/<name>/.env`. A cell's name is its directory
  name, never a field. `Resolve*` merges cell → user → built-in defaults. The `Applied` record
  (digest of the rendered Lima definition and of `vm.provision`) lives under `XDG_STATE_HOME`, not in
  the cell directory, and is what drift detection compares against.
- `internal/lima` — renders the embedded `templates/cell.yaml.tmpl` into a Lima machine definition and
  drives `limactl`. Users never write Lima YAML. Rendering is covered by golden files in `testdata/`.
- `internal/podman` — drives the single container (named `solitary`) through `limactl shell`, so
  podman never has to exist on the host. Container identity is two labels: the image as written in
  `cell.yaml`, and a digest of the environment it started with. `up` compares both and replaces the
  container when either moved — that is why changing a secret or image needs no separate command.
- `internal/cell` — the orchestrator that ties the three together. `Up` renders, creates or starts the
  machine, installs the VPN/firewall tunnel, resolves secrets, ensures the image and container, then
  installs the artifact tool. `Describe`, `List`, `Down`, `Remove`, traffic parsing and the
  `fetch`/`send` hand-off (`artifacts.go`, embedded `artifact.sh`) are here too.
- `internal/secrets` — `.env` parsing and prompting. The file may hold more than a cell declares; only
  declared names are ever passed in.
- `internal/clone` — stages a cell definition taken out of a repository so it can be shown before
  anything is installed.
- `internal/dashboard` — bubbletea TUI over `cell.List`/`Describe` plus a live traffic stream.
- `internal/host` — host memory checks, so `up` refuses a `vm.memory` the host cannot back.

Which layer a setting belongs to decides what applying it costs, and code should preserve that:
container settings (`image`, `build`, `command`, `secrets`, `git`) are applied by `up`; machine
settings (`vm`, `ports`, `network`) only take effect at the next boot — `up` warns on a running
machine and applies them when the machine is stopped; anything `vm.provision` changed lives on the
disk and only `rm` undoes it.

## Conventions

- Conventional Commits. No AI attribution trailers.
- Comments explain why a thing is the way it is, not what the code does — match that register.
- Errors wrap with `%w` and are lower-case sentences naming the operation (`"reading answer: %w"`).
- Docs are MDX under `website/content/docs/`; the README links to them. Update the relevant page when
  changing behaviour that a page describes, and add new pages to `meta.json`.
