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
which uploads the archives to it and undrafts it at the end.

Immutability is also why release-please runs as two jobs rather than one. The action's two phases —
cut the release for a merged release pull request, then write the next release pull request — ship
as a single invocation, and that arrangement cannot survive a draft: the second phase asks GitHub
what the latest release is, and the release the first phase just cut is still an unpublished draft
with no tag behind it, so it finds nothing and writes the next pull request as though the repository
had never released anything, listing its entire history. Splitting them with
`skip-github-pull-request` and `skip-github-release` puts the publish in between, so the second
phase asks the question once the answer exists. It has to run on every push, not only on a release,
which is what the `always()` in its condition is for: GoReleaser is skipped whenever nothing was
released, and a skipped dependency would otherwise skip the pull request job with it. That order is GoReleaser's own: it
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
excludes `website/`, so a commit that only touches the docs site never proposes a CLI release. That
exclusion is by path and nothing else: a commit is dropped only when *every* file it touches sits
under `website/`. A `website` scope on the subject buys nothing, and neither does a hidden
changelog type — `docs` and `chore` are kept out of the changelog but still count towards a bump.
So site-only work has to be committed on its own; the moment the same commit also edits `CLAUDE.md`
or `.github/workflows/pages.yml` it becomes a release trigger, and prose about the site belongs in
its own `docs:` commit anyway.

Pull requests are merged with a merge commit, so the changelog is composed from the commits on the
branch: every one of them has to be a well-formed Conventional Commit, and every one of them becomes
its own changelog line. The merge commit is the trap. Its title is GitHub's own `MERGE_MESSAGE`
(`Merge pull request #34 from ...`), which release-please cannot parse — but GitHub pairs that title
with the pull request's own as the *body*, and release-please reads the body as well as the header.
So a pull request titled `fix: …` lands in the changelog twice: once as the branch commit, once as
the merge commit repeating it. That is what 0.2.0 through 0.5.1 shipped, and the repository setting
is not where it is fixed — 0.5.1 was cut after the title had already been moved from `PR_TITLE` to
`MERGE_MESSAGE`, which only moved the conventional subject out of the header and into the body.

What leaves one entry is the pull request's title: write it as a sentence rather than a conventional
subject, so that the body it becomes does not parse either. `Correct a description that says the
opposite of what a cell does`, not `fix: correct a description that says the opposite of what a cell
does` — the branch commit underneath still carries the type, and it is the one that counts. This is
the one place where what goes in the GitHub UI is load-bearing for what ships.

The branch commit only counts if it is *after* the last release. release-please collects commits by
walking back from `main` and stopping at the commit the last release was cut from, so a pull request
whose branch was cut before that commit and merged after it is invisible: the walk stops at the
release commit, which is the merge commit's first parent, and the branch commit hanging off the
second parent is never reached. What is left is the merge commit alone, whose title does not parse —
by design, see above — so release-please finds nothing to bump and writes no pull request at all.
The change is on `main`, and no release will ever contain it until some later commit forces one.
`v0.9.0` shipped without the scp fix for exactly this reason.

The repository setting that prevents it is strict required status checks — "require branches to be
up to date before merging" — which is on for `main`. A pull request must have the current `main` in
it before it can merge, so its commits can never sit behind the last release. When one slips through
anyway, the way out is a new commit with a parseable subject: the release it triggers carries
everything already on `main`, and the subject should describe what is actually shipping rather than
the commit's own diff.

Squash merging is release-please's own recommendation and would collapse a pull request to a single
line instead; it is disabled here, which is also why the squash-only commit overrides are
unavailable and `release-as` is the only way to force a version.

Deliberate version overrides go through `release-as`, and it must be removed again once the release
it forced has shipped — left in place it proposes the same version forever, and that version is
already spent. `last-release-sha` is the floor the commit search stops at, for disowning commits
that should have been excluded and were not; note that it and `bootstrap-sha` are root-level options
and are ignored inside `packages`, where `release-as` and `exclude-paths` belong.

## Distribution

Three ways in, all fed by the same release archives:

- `website/public/install.sh`, served from `https://solitary.balakin.io/install.sh` because the site
  is a root domain and `public/` is copied to the root of what Pages serves. It detects the
  platform, verifies the archive against `checksums.txt` and stages the binary *inside* the install
  directory before renaming it over the old one — a rename is only atomic within one filesystem,
  and it is the one way to replace a binary that is currently running. It never uses `sudo`: a
  script piped from the network does not ask for a password, it falls back to `~/.local/bin`.
  `website/public/uninstall.sh` is its counterpart at `/uninstall.sh`, and undoes only what the
  install script did: the binary. A cell is a machine with a disk and a definition holding secrets,
  so the script stops while any `solitary-*` machine is left rather than choosing for you — the
  binary is the tool that removes them — and asks for `SOLITARY_REMOVE_CELLS` or
  `SOLITARY_KEEP_CELLS`; `SOLITARY_PURGE` is what takes the definitions and secrets too. It leaves a Homebrew install and Lima itself alone.
- `balakin/homebrew-solitary`, a tap holding one formula that GoReleaser regenerates and pushes on
  every release. A formula rather than a cask although GoReleaser deprecated `brews` for
  `homebrew_casks`: casks install on macOS only, and Homebrew quarantines what a cask downloads,
  which for an unsigned binary means a `postflight` hook stripping the attribute back off. A formula
  needs neither — and covers Linuxbrew, where `lima` has bottles too. `brews` warns for the rest of
  v2 and `goreleaser check` exits non-zero on it; when v3 drops it, generate the formula in the
  release workflow rather than giving up Linux — and have that generator write to `Formula/` too.
  The formula has to land there and nowhere else: a tap's formula directory is the first of
  `Formula`, `HomebrewFormula` and the repository root that exists, and only that one is ever
  searched, so a second copy elsewhere is not a fallback but a file Homebrew never reads.
  `brews.directory` is set to `Formula` for exactly that reason; GoReleaser defaults it to the
  repository root, where `homebrew_casks` defaults to `Casks`, and the tap served a stale version
  for a release because of it. The push is a write to another repository, which `GITHUB_TOKEN`
  cannot do — it needs `HOMEBREW_TAP_TOKEN`, a fine-grained token with contents write on the tap
  and nothing else. It is an environment secret on `homebrew-tap`, not a repository
  secret: a repository secret is readable by any job in any workflow on any branch that names it,
  where an environment secret reaches only the job declaring that environment, and only from the
  branches its policy allows — `main`. The environment carries no reviewers, so it gates nothing;
  it is there to scope the secret.
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
container settings (`image`, `build`, `command`, `secrets`, `devices`, `git`) are applied by `up`; machine
settings (`vm`, `ports`, `network`) only take effect at the next boot — `up` warns on a running
machine and applies them when the machine is stopped; anything `vm.provision` changed lives on the
disk and only `rm` undoes it.

## Conventions

- Conventional Commits. No AI attribution trailers.
- Comments explain why a thing is the way it is, not what the code does — match that register.
- Errors wrap with `%w` and are lower-case sentences naming the operation (`"reading answer: %w"`).
- Docs are MDX under `website/content/docs/`; the README links to them. Update the relevant page when
  changing behaviour that a page describes, and add new pages to `meta.json`.
