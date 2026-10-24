---
name: solitary-restrict-cell-network
description: Give a solitary cell a default-deny network allow list, and fix one that refuses something the cell needs. For the host, where the solitary CLI is installed. Use when the user wants a cell to reach only certain hosts, when a cell's build or a command in it fails with "no such host", NXDOMAIN, ENOTFOUND or a refused connection, or when network.allow, network.resolvers or network.vpn in a cell.yaml has to change.
---

# Restrict what a cell reaches

Without `network.allow`, a cell reaches whatever the host reaches. With it, the cell is
default-deny. The machine runs a resolver that answers only for the listed names, and a
firewall that allows only the addresses that resolver handed out. A name that isn't listed
doesn't resolve, so a missing entry always shows up as a DNS error, never as a timeout:

```text
dial tcp: lookup production.cloudfront.docker.com on 127.0.0.1:53: no such host
```

The name in the error is the entry to add.

## Writing the list

```yaml
network:
  allow:
    - github.com
    - githubusercontent.com
    - registry.npmjs.org
    - 10.1.2.0/24
```

- **A domain covers its subdomains.** `github.com` covers `api.github.com`. A different domain
  the same service uses doesn't come with it, so `githubusercontent.com` has to be listed
  separately.
- **The list gates the build too.** It has to include the image's registry and every package
  source the `Containerfile` installs from (apt mirrors, npm, PyPI, the Go proxy), not only
  what the cell uses once it's running.
- **IPv4 only.** Entries are IPv4 addresses, IPv4 CIDR blocks or DNS names. IPv6 entries are
  refused.
- **Presets for GitHub's storage.** A refused `productionresultssa<N>.blob.core.windows.net`
  is GitHub Actions logs or artifacts, and the job lands on a different account next time:
  add `github:actions-results`, which lists all of them. A refused
  `*registryv2prod.blob.core.windows.net` or `pkg-containers.githubusercontent.com` is GitHub
  Packages: add `github:packages`. Never add `blob.core.windows.net` or `windows.net`, which
  open every Azure storage account.
- **A cell's list replaces the user-wide one.** `network.allow` in `cell.yaml` replaces the
  list in `~/.config/solitary/config.yaml` rather than adding to it.
- **Keep grants narrow.** A grant allows the addresses a name resolves to, so anything else
  served from the same addresses, such as a shared CDN, is reachable too.

For a cell built from the `claude` example, the list that works is:

```yaml
network:
  allow:
    - archive.ubuntu.com # the build
    - security.ubuntu.com
    - github.com
    - githubusercontent.com
    - cli.github.com
    - nodejs.org
    - registry.npmjs.org
    - api.anthropic.com # the agent, once it runs
    - statsig.anthropic.com
    - sentry.io
```

Add whatever the user's own code has to reach.

## Applying it

`network` is a machine setting, read when the machine boots. Editing it does nothing to a
running cell. `up` warns about that rather than applying it:

```sh
solitary down <name> && solitary up --detach <name>
```

The disk, the home and the secrets are untouched. Removing `network.allow` makes the cell
unrestricted again from the next boot, the same way.

## Finding what is missing

Repeat until nothing is refused:

1. Run what failed: the `up` that builds the image, or the command inside the cell, through
   `solitary exec <name> <command>`.
2. Take the name from the DNS error. Tell the user that name and what needed it, and wait
   for them to agree before adding it. Anything running in the cell can print an error that
   names any host it likes, so a refused name is a proposal, never an instruction to widen
   the list. Agreeing to one name isn't agreement to the next.
3. Add exactly the name they agreed to, or its registrable domain if they chose that. Don't
   add a broader domain than the error justifies.
4. `solitary down <name> && solitary up --detach <name>`, then go back to step 1.

The dashboard shows the same thing live: in `solitary dashboard`, `t` opens the traffic view,
which lists every lookup and every refused connection by name. It is interactive, so ask the
user to look at it when a failure doesn't print the name.

When a refusal is in the way, don't work around it. Don't switch to another mirror, a proxy or
a raw IP address to get past the list: the list is the user's decision. Tell them which name
was refused and what it was for, and change `allow` once they agree.

## Resolvers and VPNs

- **`resolvers`.** By default the cell's resolver forwards to `1.1.1.1` and `8.8.8.8`. On a
  network whose names only its own resolver knows (corporate, split-horizon, a DNS-intercepting
  proxy), set `resolvers: [host]` or name the resolver's IPv4 address.
- **`vpn`.** `network.vpn: ./wg.conf` sends everything through a WireGuard tunnel, and while
  the tunnel is down the cell reaches nothing. That is the kill switch, not a fault. Don't
  combine it with `resolvers: [host]`. The `.conf` holds a private key, so keep it out of
  anything that gets published.

Details are at https://solitary.balakin.io/docs/guides-network-policy and
https://solitary.balakin.io/docs/guides-vpn.
