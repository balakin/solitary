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
make fmt                        # golangci-lint fmt (gofumpt + goimports, local prefix github.com/balakin/solitary)

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

## Deploying the site

`.github/workflows/pages.yml` publishes `website/` to GitHub Pages on every push to `main` that
touches it. The site tracks `main` rather than the last release: release-please already keeps
`website/` out of the CLI version, so the two ship independently and a documentation fix never
waits for a release. Nothing in the docs names a version — the install page points at
`releases/latest` — so the only skew the site can carry is prose describing behaviour that is
written but not yet released.

The site is served from the root of `solitary.balakin.io`, a custom domain set in the repository's
Pages settings; the DNS record is a `CNAME` from that subdomain to `balakin.github.io`. It used to
be a project site under `/solitary`, and that one path prefix cost a `basename`, a
`renderBuiltUrl` in `vite.config.ts` (Vite's `base` cannot be used: it also prefixes the URLs
react-router asks itself for while prerendering, so the prerender fails, silently) and a build step
that re-prefixed every URL react-router took from its own build manifest. Serving from a root
removed all of it — so keep the site at a root, and reach for a second host rather than a
subdirectory of this one.

What a static host still asks of the app is two things:

- `routeDiscovery: { mode: 'initial' }` in `react-router.config.ts`. Under `ssr: true`
  react-router otherwise discovers routes lazily, asking `/__manifest` for the routes a link
  points at before it navigates there; nothing answers that on Pages, so the first client-side
  navigation lands on the error boundary while the prerendered pages all look fine.
- `website/scripts/pages-artifact.mjs` (`pnpm build:pages`), which renames the prerendered
  catch-all route to `404.html`. Pages answers an unknown path with that file from the root of
  what it serves, and a prerendered route is a directory with an `index.html` in it.

Everything else is prerendering: every route is a file, `/api/search` included.

Search has no server behind it. `app/routes/search.ts` prerenders the whole index to `/api/search`
and the dialog downloads it and searches in the browser.

## Releases

`.github/workflows/release.yml` runs release-please on every push to `main`. It keeps one release
pull request open, accumulating `CHANGELOG.md` from the Conventional Commit subjects; merging that
pull request is what cuts a release. Releases here are immutable — publishing seals the tag and the
assets — so release-please creates the release as a draft and a second job hands it to GoReleaser,
which uploads the archives to it and undrafts it at the end. That order is GoReleaser's own: it
always uploads to a draft, and `release.use_existing_draft` only points it at the one already there
rather than a release of its own, matched by name against the tag. A draft has no tag until it is
published, which is why that job checks out the release commit by SHA and tags locally. Nothing is
released by pushing a tag by hand either: a tag pushed with `GITHUB_TOKEN` starts no workflow.

Never delete a published release. Its tag name is reserved for good — GitHub refuses to reuse a
name that belonged to an immutable release, even after the release and the tag are gone, and even
across a deleted repository. A release that went out wrong is fixed by shipping the next version,
not by cutting the same one again. `v0.1.0` is already spent this way, which is why the first
release is `0.1.1`.

The version lives in `.release-please-manifest.json` and nowhere else — `make build` and GoReleaser
bake it in through `-ldflags`, so no source file names a version. `release-please-config.json`
excludes `website/`, so a commit that only touches the docs site never proposes a CLI release; give
site-only work a `website` scope so that stays true. Deliberate overrides go through `release-as` in
that config, and it must be removed again once the release it forced has shipped.

## Distribution

Three ways in, all fed by the same release archives:

- `website/public/install.sh`, served from `https://solitary.balakin.io/install.sh` because the site
  is a root domain and `public/` is copied to the root of what Pages serves. It detects the
  platform, verifies the archive against `checksums.txt` and stages the binary *inside* the install
  directory before renaming it over the old one — a rename is only atomic within one filesystem,
  and it is the one way to replace a binary that is currently running. It never uses `sudo`: a
  script piped from the network does not ask for a password, it falls back to `~/.local/bin`.
- `balakin/homebrew-solitary`, a tap holding one formula that GoReleaser regenerates and pushes on
  every release. A formula rather than a cask although GoReleaser deprecated `brews` for
  `homebrew_casks`: casks install on macOS only, and Homebrew quarantines what a cask downloads,
  which for an unsigned binary means a `postflight` hook stripping the attribute back off. A formula
  needs neither — and covers Linuxbrew, where `lima` has bottles too. `brews` warns for the rest of
  v2 and `goreleaser check` exits non-zero on it; when v3 drops it, generate the formula in the
  release workflow rather than giving up Linux. The push is a write to another repository, which
  `GITHUB_TOKEN` cannot do — it needs `HOMEBREW_TAP_TOKEN`, a PAT with contents write on the tap and
  nothing else.
- `solitary update` (`internal/update`), which does what the install script does to the binary it is
  running: resolve the latest tag, download, verify, rename over itself. Every other command asks
  github once a day and mentions a newer release at most three times, only to a terminal, recorded
  in `update.json` under `XDG_STATE_HOME` — `SOLITARY_NO_UPDATE_CHECK` turns it off.

Only a bare dotted version is treated as a release. `make build` bakes in what `git describe` says
(`v0.1.1-10-g619fd0f-dirty`), which is *ahead* of the tag it names, not a pre-release of it, so a
source build is never notified and never replaced.

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
