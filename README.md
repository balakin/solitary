# solitary

**Hypervisor-isolated cells for running coding agents off the leash.**

> ⚠️ Pre-alpha. Cells build, run, persist, take secrets and can be held to an
> egress allow list — see [what this does not protect against](#what-this-does-not-protect-against).

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
- **Nothing is mounted from the host, ever.** Clone, build, lint, test and
  review inside the cell. The host only ever displays results. A cell is a
  server: its state lives on its own disk and is destroyed with it. The one
  thing that outlives a cell is its secrets, because those were never in it.
- **No browser.** A cell has no browser and no launcher configured. Authenticate
  on the host and pass the credential in as a secret.
- **Secrets are whitelisted per cell.** This cell sees a GitHub token; that one
  does not.
- **Ports are the way in.** Run a dev server in the cell, open it in your
  browser on the host. Set `ports:` and only those reach it.
- **Egress is yours to allow.** Set `network.allow` and the cell reaches the
  domains you list and nothing else — not the rest of the internet, not your
  machine, not your local network.

Cells are meant to be thrown away. `rm` then `up` gets you a clean one, still
authenticated, because secrets live on the host and not in the VM.

### What this does not protect against

Isolation stops a compromise of your machine. It does not stop an agent
misusing the authority you granted it: it can still push to any repository the
token you whitelisted can reach, and reach anything you allowed. A cell with no
`network.allow` reaches the whole internet. The image you run is trusted code.

A secret passed to a cell lives inside that cell. The `.env` file never leaves
the host, but the values reach the container as environment variables, and
podman records a container's environment in its own metadata on the machine's
disk. `rm` discards that along with everything else. Give a cell only the
credentials it needs.

## Configuration

`~/.config/solitary/cells/<name>/cell.yaml` — the cell's name is the directory
name, never a field inside the file.

```yaml
image: ghcr.io/you/nvim-claude:latest
# build: ./Containerfile  # or build it instead — set one, not both

command: sleep infinity   # optional; must not exit — it is the container's life

secrets:            # only these names are passed into the cell
  - CLAUDE_API_KEY
  - GITHUB_TOKEN

ports:              # omit and every listening port reaches host localhost;
  - 8080            # set and only these do

network:            # omit and the cell reaches whatever the host reaches;
  allow:            # set and it reaches these and nothing else
    - github.com
    - api.anthropic.com
  vpn: ./vpn.conf   # optional; send all of it through this WireGuard tunnel

git:                # optional; usually set once in config.yaml instead
  name: Ada Lovelace
  email: ada@example.com

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

`~/.config/solitary/config.yaml` holds `vm:`, `git:` and `network:` blocks used
as the defaults for every cell. A cell that sets `network.allow` replaces the
user-wide list rather than adding to it: what a cell may reach should be
readable in one place.

A cell has nowhere of its own to keep a git identity — nothing is mounted from
the host, and anything configured by hand inside a cell is gone when it is
rebuilt. So `git:` is passed in as environment variables, which git reads ahead
of any config file. git wants an author and a committer, each with a name and
an email, and has no single setting for both; solitary fills in all four from
these two fields. Write it once in `config.yaml` and every cell commits as you.

### Restricting what a cell can reach

Without `network.allow`, a cell reaches whatever your host reaches. With it, the
cell is default-deny: the listed domains resolve and are reachable, and nothing
else is — not the rest of the internet, not your machine, not your local
network. Connections the host starts *into* the cell keep working, so forwarded
ports are unaffected.

```yaml
network:
  allow:
    - github.com                 # covers api.github.com
    - objects.githubusercontent.com   # a different domain: list it too
    - registry.npmjs.org
    - 10.1.2.0/24                # addresses and CIDR blocks work as given
```

By default the cell's resolver forwards to `1.1.1.1` and `8.8.8.8`. That is the
wrong answer on a network whose names only its own resolver knows — a corporate
one, a split horizon, anything behind a proxy that intercepts DNS — and on one
that refuses to carry DNS to a public resolver at all. Name the resolvers
instead:

```yaml
network:
  resolvers:
    - host          # the resolver this machine is given, which is the host's
    - 10.0.0.53     # or name them outright
```

`host` is discovered at boot from the machine's DHCP lease, so it follows the
network you are on. It is the one hole in VM→host isolation and a narrow one:
the cell's own resolver, port 53, and nothing else — a process in the cell still
cannot reach the resolver directly, only ask the one in front of it, which
answers for the allowed names only.

Two pieces enforce it, both in the machine and outside the container. A resolver
answers for the listed names only — anything else gets NXDOMAIN, so a query
cannot carry data out to a resolver of an agent's choosing — and it records the
addresses it hands out in a set that the firewall allows. So a name resolves and
is reachable, or does neither, and a site that changes its addresses keeps
working without anyone editing a rule. The container is rootless: nothing inside
it can load a firewall rule, stop the resolver, or edit either one's config.

The catch is that an allow list has to be complete. A restricted cell cannot
pull its own image unless the registry is listed, and a build cannot install
packages unless its package sources are. When something is missing you see it
immediately, as a name that does not resolve:

```
dial tcp: lookup production.cloudfront.docker.com on 127.0.0.1:53: no such host
```

Changing the list takes effect when the machine next starts: `solitary down
<name> && solitary up <name>`.

### Sending a cell out through a VPN

`network.vpn` points at a WireGuard configuration — the `.conf` your provider
gives you, saved beside `cell.yaml` and used unchanged:

```yaml
network:
  vpn: ./vpn.conf
  allow:
    - github.com
```

Everything the cell reaches then leaves through that tunnel, so it has its own
exit address rather than your host's. The tunnel is brought up in the machine,
where the container cannot touch it; the container needs no configuration at all,
because it runs on the machine's own network.

The allow list is enforced exactly as before, with one difference: what it allows
is reachable **through the tunnel only**. If the tunnel is down, nothing leaves —
rather than the same traffic quietly going out the way it came. The one exception
is the cell's resolver asking the servers under `resolvers:`, because the tunnel's
own peer has to be resolved before there is a tunnel to resolve it through.

Two things a configuration must not have: a `DNS =` line — a cell resolves through
its own resolver, and if you want the tunnel's, name its address under
`resolvers:` — and a missing `Endpoint`. Both are refused when the cell is read,
rather than leaving you with a machine whose tunnel never comes up.

**The `.conf` is not part of the cell.** It holds a private key, so solitary never
puts it in the machine's definition; it is placed into the running machine
separately, and read from your disk each time. That is what makes a cell
definition shareable: publish `cell.yaml` and your `Containerfile`, keep `.env`
and `vpn.conf` out of it, and whoever copies the cell supplies their own — from
any provider, with no edit to `cell.yaml`, since the peer to allow is read out of
whichever configuration is present.

```gitignore
# in a repo of cell definitions
.env
vpn.conf
```

### Building the image

Instead of `image:`, a cell can name a `Containerfile` to build:

```yaml
build: ./Containerfile
```

The path is relative to the cell's directory, and that directory is the build
context. It is copied into the machine and built there, so nothing in a
`Containerfile` ever runs on the host — the same reason the rest of this tool
exists. `.git` and `node_modules` are left out of the copy, and the copy is
deleted afterwards so a context that carried build-time secrets does not linger.

`up` rebuilds when anything in the context changes and reuses the image when
nothing has, so iterating on a `Containerfile` is just `up` again.

## Commands

```
solitary init <name>            scaffold a cell definition
solitary up <name|image-ref>    start the cell and attach
solitary shell <name>           shell into a running cell
solitary exec <name> <cmd...>   run one command in a running cell
solitary down <name>            stop the cell, keep the disk
solitary rm <name>              destroy the VM; definition and secrets stay
solitary ls                     list cells and their state
solitary fetch <name>           collect what a cell published
solitary send <name> <file...>  put files into a cell's inbox
solitary dashboard              manage cells in a live view
solitary secrets <name>         set the values a cell is allowed to see
```

`up` is the only command that changes state, and it is idempotent: it creates
the cell if absent, boots it if stopped, and attaches if it is already running.
It also replaces the container when the image or the secrets changed, so
editing `cell.yaml` and running `up` again is all a change ever takes.

### Handing files in and out

Nothing is mounted from the host, so a cell has no path to your files and its own
work has no way out. Two folders and two commands close that gap without opening
one: the host is always the party that moves the bytes.

Inside the cell:

```
artifact report.pdf dist/app   publish these, for the host to collect
artifact --list                what is published, and what is waiting to come in
```

On the host:

```
solitary fetch claude          copy everything published into the current directory
solitary fetch claude --list   see what is there without copying it
solitary fetch claude a.pdf    or name what you want
solitary send claude notes.md  copy a file into the cell's inbox ($HOME/inbox)
```

`fetch` copies rather than moves — the cell keeps its copy, so fetching twice is
not a mistake and an interrupted fetch loses nothing. Both folders live in the
cell's home, so they survive the container being replaced and go when the cell
is destroyed.

Only the machine has to be running, not the container: what a cell published can
still be collected after whatever produced it has died.

**The outbox is untrusted input, and is treated that way.** Its contents are
named by whatever runs in the cell, and those names become paths on your machine:

- solitary does the listing itself — regular files only, one level deep. A
  symlink is not followed and a directory is not descended into (pack it into an
  archive and publish that).
- a name that is not a plain file name — `../escape`, `-rf`, an empty one — is
  refused *by name* rather than silently skipped, and the rest still come out.
- nothing is written over something already there without `--force`, and the
  check happens before the first file is copied.
- nothing arrives executable. What a cell produces is data on the host, never a
  program.

### The dashboard

`solitary dashboard` is the same things in one live view: every cell and its
state, refreshed as it changes, with the actions that apply to the selected one.

```
 solitary
 ╭─────────────────────╮╭──────────────────────────────────╮
 │ cells               ││ claude                           │
 │ › ● claude  running ││ image   build:./Containerfile    │
 │   ○ demo    stopped ││ machine 4 cpus · 4GiB · 40GiB    │
 ╰─────────────────────╯│ ports   all reach host localhost │
                        │ network 2 allowed                │
                        │         github.com               │
                        │         api.anthropic.com        │
                        │ secrets 2 of 2 set               │
                        ╰──────────────────────────────────╯
 ↑↓ move · ⏎ shell · u up · s stop · e secrets · d rm · r refresh · q quit
```

It does nothing the commands cannot, and the slow ones it runs *as* those
commands: pressing `u` runs `solitary up --detach`, so a build prints what a
build prints and a cell missing a secret asks for it the way it always does.
The dashboard steps out of the way and comes back rather than owning a second,
worse version of each.

`t` follows what the selected cell's network is doing, as it happens: every name
it asks about, what it resolved to, and every connection the firewall refused.

```
 ╭─────────────────────╮╭─────────────────────────────────────────────────╮
 │ cells               ││ traffic · claude                                │
 │ › ● claude  running ││ 12:07:53 query    api.github.com ×2             │
 ╰─────────────────────╯│ 12:07:53 resolved api.github.com → 140.82.121.6 │
                        │ 12:07:53 refused  example.com                   │
                        │ 12:07:53 denied   1.1.1.1:443                   │
                        ╰─────────────────────────────────────────────────╯
 live · c clear · any other key back
```

It reads the machine's log, where both halves of the allow list record what they
did — so a cell cannot see, let alone edit, what is recorded about it. Repeats
fold into a count, since one lookup answers with every address a name has. This
is also the fastest way to find what an allow list is missing: a `refused` line
names it.

`n` shows the whole allow list, and `e` manages the selected cell's secrets: which are set, which are not, and a
masked field to set or rotate one. Values are never displayed — the dashboard
reads only whether each name has one.

`exec` is `shell` for one command: it exits with the command's status and
leaves its streams alone, so a cell can be scripted from the host without a
terminal in the middle.

```
solitary exec claude git status
solitary exec claude bash -lc 'npm test | tail -20'
solitary exec claude cat notes.md > notes.md   # redirection is the host's
```

The command is run directly rather than through a shell, so flags after it are
its own and quoting survives. Ask for a shell explicitly when you want one.

A session you are watching also carries your terminal into the cell: its name,
whether it does true colour, and its description, compiled into the cell the
first time you attach. Terminals that ship their own terminfo — ghostty, kitty,
wezterm — work without the cell having heard of them, and a theme arrives in the
colours it was written in.

Work belongs in `/home/cell`, which lives on the machine's disk rather than in
the container. It survives a new image, a stop and start, and anything an editor
installs into the home directory — so a tool you authenticate or configure by
hand inside a cell stays that way for the life of the cell. `rm` is what
discards it, and `rm` means destroy the machine.

Nothing is synced back to the host, because a cell has no path to the host.
Anything you would be upset to lose either belongs in a git remote or belongs in
`secrets:`, which lives on the host and is passed in on every start.

Creating a cell takes a couple of minutes: it downloads a cloud image and
installs podman. Everything after that is container-speed.

### A machine has to fit the host

On Linux a guest's entire memory is a file on `/dev/shm`, which is usually half
of RAM. A machine asking for more than that filesystem holds boots, reports
itself running, and then dies the moment it touches enough pages — the process
stays alive and Lima still calls it running, so every command hangs instead of
failing. `up` refuses to create such a machine and says what to lower `vm.memory`
to, and warns when a machine fits the filesystem but not the space free on it.

A cell that stops answering for any other reason is reported as `unreachable`
rather than `running`, and commands against it fail in seconds instead of
blocking. `solitary down` then `up` recovers it without touching the disk.

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

- An SNI-filtering proxy as a second way to allow a domain, for sites whose
  addresses are shared with everything else behind the same CDN.
- Distributing cell definitions as OCI artifacts.
- VM snapshots, so a new cell does not reinstall podman from scratch.

## License

Apache 2.0
