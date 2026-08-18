import type { Route } from './+types/home';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { DynamicCodeBlock } from 'fumadocs-ui/components/dynamic-codeblock';
import { Link } from 'react-router';
import {
  ArrowRightLeft,
  DoorOpen,
  KeyRound,
  ShieldBan,
  Trash2,
  Waypoints,
  type LucideIcon,
} from 'lucide-react';
import { homeOptions } from '@/lib/layout.shared';
import { Terminal } from '@/components/terminal';
import { Footer } from '@/components/footer';
import { appName, asset, homeImagePath, siteDescription, siteTagline } from '@/lib/shared';

export function meta(_args: Route.MetaArgs) {
  const title = `${appName} — ${siteTagline}`;

  return [
    { title },
    { name: 'description', content: siteDescription },
    { property: 'og:type', content: 'website' },
    { property: 'og:title', content: title },
    { property: 'og:description', content: siteDescription },
    { property: 'og:image', content: asset(homeImagePath) },
    { property: 'og:site_name', content: appName },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: title },
    { name: 'twitter:description', content: siteDescription },
    { name: 'twitter:image', content: asset(homeImagePath) },
  ];
}

const controls: { icon: LucideIcon; title: string; text: string; href: string }[] = [
  {
    icon: ShieldBan,
    title: 'Default-deny egress',
    text: 'List the domains a cell may reach and it reaches nothing else — not the rest of the internet, not your machine, not your local network. One list drives both the resolver and the firewall.',
    href: '/docs/guides-network-policy',
  },
  {
    icon: Waypoints,
    title: 'A VPN with a kill switch',
    text: 'Point a cell at a WireGuard config and everything it reaches leaves through that tunnel. With the tunnel down, nothing leaves at all — no quiet fallback to the route it came in on.',
    href: '/docs/guides-vpn',
  },
  {
    icon: KeyRound,
    title: 'Secrets whitelisted per cell',
    text: 'This cell sees a GitHub token; that one does not. Values live on the host and are passed in by name, so a cell you destroy is a cell you can rebuild already authenticated.',
    href: '/docs/guides-secrets',
  },
  {
    icon: DoorOpen,
    title: 'Ports are the way in',
    text: 'Run a dev server in the cell and open it in the browser on your host. Name the ports you want and only those are forwarded.',
    href: '/docs/networking',
  },
  {
    icon: ArrowRightLeft,
    title: 'An explicit hand-off',
    text: 'The cell publishes; the host collects. Files come out by name, never executable, and never over something already there — because a name chosen inside a cell is untrusted input.',
    href: '/docs/artifacts',
  },
  {
    icon: Trash2,
    title: 'Cells are disposable',
    text: 'Destroy one and make a clean one from the same definition. The disk goes; the definition and the credentials, which were never inside it, stay.',
    href: '/docs/commands',
  },
];

const cellYaml = `image: ghcr.io/you/agent:latest

secrets:            # only these names are passed in
  - GITHUB_TOKEN

ports:              # only these reach the host
  - 8080

network:            # and it reaches nothing else
  allow:
    - github.com
    - registry.npmjs.org
  vpn: ./vpn.conf   # optional: all of it, through this tunnel`;

const install = `curl -fsSL -o solitary.tar.gz \\
  https://github.com/balakin/solitary/releases/latest/download/solitary_darwin_arm64.tar.gz
tar -xzf solitary.tar.gz solitary
install -m 755 solitary /usr/local/bin/solitary`;

const dashboard = `╭─────────────────────╮╭──────────────────────────────────╮
│ cells               ││ claude                           │
│ › ● claude  running ││ image   build:./Containerfile    │
│   ○ demo    stopped ││ machine 4 cpus · 4GiB · 40GiB    │
╰─────────────────────╯│ ports   all reach host localhost │
                       │ network 2 allowed                │
                       │ vpn     up · handshake 12s ago   │
                       │ secrets 2 of 2 set               │
                       ╰──────────────────────────────────╯
 ↑↓ move · ⏎ shell · u up · s stop · e secrets · d rm · q quit`;

const traffic = ` ╭─────────────────────╮╭─────────────────────────────────────────────────╮
 │ cells               ││ traffic · claude                                │
 │ › ● claude  running ││ 12:07:53 query    api.github.com ×2             │
 ╰─────────────────────╯│ 12:07:53 resolved api.github.com → 140.82.121.6 │
                        │ 12:07:53 refused  example.com                   │
                        │ 12:07:53 denied   1.1.1.1:443                   │
                        ╰─────────────────────────────────────────────────╯
 ↑↓ scroll · G live · / filter · b refused only · c clear · esc back`;

const layers = [
  {
    name: 'Your host',
    role: 'Starts machines, holds the secrets, displays results. Never mounted into a cell.',
  },
  {
    name: 'The VM',
    role: 'The boundary. A hypervisor, not a shared kernel — root in the container is not root on your machine.',
    boundary: true,
  },
  {
    name: 'The container',
    role: 'The toolset, and replaceable. Change the image, run up again: same disk, same secrets, different tools.',
  },
];

export default function Home() {
  return (
    <HomeLayout {...homeOptions()}>
      <main>
        {/* Hero */}
        <section className="mx-auto flex max-w-6xl flex-col items-center px-6 pt-24 pb-24 text-center md:pt-36">
          <p className="mb-6 rounded-full border border-fd-border bg-fd-card px-4 py-1.5 text-sm text-fd-muted-foreground">
            Early · macOS and Linux
          </p>
          <h1 className="max-w-4xl text-5xl font-semibold tracking-tight text-balance md:text-7xl">
            Let coding agents run free.
            <br />
            <span className="text-fd-primary">Keep your machine out of reach.</span>
          </h1>
          <p className="mt-8 max-w-2xl text-lg leading-8 text-balance text-fd-muted-foreground md:text-xl">
            Solitary runs coding agents in hypervisor-isolated cells: disposable virtual machines
            with no host mounts, narrowly scoped secrets, controlled network access, and a
            deliberate way to move work in and out.
          </p>
          <div className="mt-10 flex flex-wrap justify-center gap-3">
            <Link
              className="rounded-full bg-fd-primary px-6 py-3 font-medium text-fd-primary-foreground transition-opacity hover:opacity-85"
              to="/docs/quickstart"
            >
              Create your first cell
            </Link>
            <a
              className="rounded-full border border-fd-border px-6 py-3 font-medium transition-colors hover:bg-fd-accent"
              href="https://github.com/balakin/solitary"
              target="_blank"
              rel="noreferrer"
            >
              View on GitHub
            </a>
          </div>
        </section>

        {/* The threat */}
        <section className="border-y border-fd-border bg-fd-card/50">
          <div className="mx-auto grid max-w-6xl gap-12 px-6 py-24 md:grid-cols-[1fr_1.2fr] md:items-start">
            <div>
              <p className="text-sm font-medium tracking-widest text-fd-primary uppercase">
                The problem
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
                A mounted project is not a boundary. It is a way in.
              </h2>
            </div>
            <div className="space-y-5 leading-8 text-fd-muted-foreground">
              <p>
                Most tools that sandbox a coding agent hand it a container and mount your project
                directory into it. An agent that can write to that directory can write to{' '}
                <code className="text-fd-foreground">.git/hooks/pre-commit</code>, a{' '}
                <code className="text-fd-foreground">package.json</code> script, a{' '}
                <code className="text-fd-foreground">Makefile</code>, an{' '}
                <code className="text-fd-foreground">.envrc</code>, a{' '}
                <code className="text-fd-foreground">.vscode/tasks.json</code>, or a lint plugin
                resolved from the local tree.
              </p>
              <p className="text-fd-foreground">
                Then you run <code>npm test</code> or <code>git commit</code> on your machine, and
                the payload runs as you. No escape was necessary. You executed it yourself.
              </p>
              <Link
                className="inline-block font-medium text-fd-primary hover:underline"
                to="/docs/why-solitary"
              >
                Read why Solitary exists →
              </Link>
            </div>
          </div>
        </section>

        {/* The model */}
        <section className="mx-auto max-w-6xl px-6 py-24">
          <div className="max-w-2xl">
            <p className="text-sm font-medium tracking-widest text-fd-primary uppercase">
              The model
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
              A cell is a VM with a container inside it.
            </h2>
            <p className="mt-5 leading-8 text-fd-muted-foreground">
              The boundary and the toolset are different things, so you can replace one without
              disturbing the other.
            </p>
          </div>
          <ol className="mt-12 grid gap-4 md:grid-cols-3">
            {layers.map((layer) => (
              <li
                key={layer.name}
                className={
                  layer.boundary
                    ? 'rounded-2xl border-2 border-fd-primary bg-fd-card p-6'
                    : 'rounded-2xl border border-fd-border bg-fd-card/50 p-6'
                }
              >
                <h3 className="font-semibold">{layer.name}</h3>
                {layer.boundary && (
                  <p className="mt-1 text-xs font-medium tracking-widest text-fd-primary uppercase">
                    The boundary
                  </p>
                )}
                <p className="mt-3 text-sm leading-7 text-fd-muted-foreground">{layer.role}</p>
              </li>
            ))}
          </ol>
          <p className="mt-8 leading-8 text-fd-muted-foreground">
            Nothing is mounted from the host, ever. Clone, build, lint, test and review inside the
            cell.{' '}
            <Link className="font-medium text-fd-primary hover:underline" to="/docs/concepts">
              Core concepts →
            </Link>
          </p>
        </section>

        {/* See it running */}
        <section className="border-y border-fd-border bg-fd-secondary/40">
          {/* Wider than the other sections: these are screenshots of fixed-width
              art, and this is the width at which both sit side by side. */}
          <div className="mx-auto max-w-7xl px-6 py-24">
            <div className="flex flex-col justify-between gap-8 md:flex-row md:items-end">
              <div className="max-w-2xl">
                <p className="text-sm font-medium tracking-widest text-fd-primary uppercase">
                  See it running
                </p>
                <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
                  Every cell, and what its network is doing.
                </h2>
              </div>
              <Link className="font-medium text-fd-primary hover:underline" to="/docs/dashboard">
                The dashboard →
              </Link>
            </div>
            {/* Each card is only as wide as its own art, so the two sit side by
                side when they both fit and stack, centred, when they do not —
                rather than being stretched to a column width they do not match. */}
            <div className="mt-10 flex flex-wrap justify-center gap-6">
              <Terminal
                className="w-fit max-w-full"
                command="solitary dashboard"
                label="The dashboard: a list of cells beside a detail panel showing the selected cell's image, machine resources, ports, allowed domains, VPN handshake, and how many secrets are set."
              >
                {dashboard}
              </Terminal>
              <Terminal
                className="w-fit max-w-full"
                command="solitary dashboard · t"
                label="The traffic view: the same cell list beside a live log of timestamped lines — a DNS query, what it resolved to, a refused name, and a denied connection."
              >
                {traffic}
              </Terminal>
            </div>
            <p className="mt-8 max-w-3xl leading-8 text-fd-muted-foreground">
              The traffic view reads the machine's own log, so a cell cannot see — let alone edit —
              what is recorded about it. It is also the fastest way to find what an allow list is
              missing: a <code className="text-fd-foreground">refused</code> line names it.
            </p>
          </div>
        </section>

        {/* What you control */}
        <section className="mx-auto max-w-6xl px-6 py-24">
          <div className="max-w-2xl">
            <p className="text-sm font-medium tracking-widest text-fd-primary uppercase">
              What you control
            </p>
            <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
              Authority arrives in named pieces.
            </h2>
          </div>
          <div className="mt-12 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {controls.map((control) => (
              <Link
                key={control.title}
                to={control.href}
                className="group rounded-2xl border border-fd-border bg-fd-card p-6 transition-colors hover:border-fd-primary"
              >
                <control.icon className="size-5 text-fd-primary" aria-hidden />
                <h3 className="mt-4 font-semibold group-hover:text-fd-primary">{control.title}</h3>
                <p className="mt-3 text-sm leading-7 text-fd-muted-foreground">{control.text}</p>
              </Link>
            ))}
          </div>
        </section>

        {/* The definition */}
        <section className="border-y border-fd-border bg-fd-card/50">
          <div className="mx-auto grid max-w-6xl gap-12 px-6 py-24 md:grid-cols-[1fr_1.1fr] md:items-center">
            <div>
              <p className="text-sm font-medium tracking-widest text-fd-primary uppercase">
                A cell in practice
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
                Define it once. Share it. Rebuild it clean.
              </h2>
              <p className="mt-5 leading-8 text-fd-muted-foreground">
                The definition says what a cell needs, not the values it receives. Publish it and
                whoever copies it supplies their own credentials and their own tunnel — so an
                environment can be reviewed in a pull request before anyone runs it.
              </p>
              <Link
                className="mt-6 inline-block font-medium text-fd-primary hover:underline"
                to="/docs/guides-workflows"
              >
                Shareable workflows →
              </Link>
            </div>
            <DynamicCodeBlock lang="yaml" code={cellYaml} />
          </div>
        </section>

        {/* What it does not do */}
        <section className="mx-auto max-w-6xl px-6 py-24">
          <div className="grid gap-12 md:grid-cols-[1fr_1.2fr] md:items-start">
            <div>
              <p className="text-sm font-medium tracking-widest text-fd-muted-foreground uppercase">
                Honest limits
              </p>
              <h2 className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl">
                This does not make an agent harmless.
              </h2>
            </div>
            <div className="space-y-5 leading-8 text-fd-muted-foreground">
              <p>
                Isolation stops a compromise of your machine. It does not stop an agent misusing the
                authority you granted it: it can still push to any repository the token you
                whitelisted can reach, and reach anything you allowed. A cell with no allow list
                reaches the whole internet. The image you run is trusted code.
              </p>
              <p>
                A secret passed to a cell lives inside that cell. Give one only the credentials it
                needs, and treat a cell's output as data rather than as something to run.
              </p>
              <Link
                className="inline-block font-medium text-fd-primary hover:underline"
                to="/docs/limitations"
              >
                Limitations and trade-offs →
              </Link>
            </div>
          </div>
        </section>

        {/* Get started */}
        <section className="border-t border-fd-border bg-fd-secondary/40">
          <div className="mx-auto grid max-w-6xl gap-12 px-6 py-24 md:grid-cols-[1fr_1.1fr] md:items-center">
            <div>
              <h2 className="text-3xl font-semibold tracking-tight md:text-4xl">
                Install it and take a cell for a walk.
              </h2>
              <p className="mt-5 leading-8 text-fd-muted-foreground">
                Every release ships a binary for macOS and Linux, and the source builds in one
                command. It needs{' '}
                <a
                  className="text-fd-primary hover:underline"
                  href="https://lima-vm.io"
                  target="_blank"
                  rel="noreferrer"
                >
                  Lima
                </a>{' '}
                2.0 or newer, and Go only if you build it yourself. Creating the first cell takes a
                couple of minutes while it downloads a cloud image and installs podman; everything
                after that is container-speed.
              </p>
              <div className="mt-8 flex flex-wrap gap-3">
                <Link
                  className="rounded-full bg-fd-primary px-6 py-3 font-medium text-fd-primary-foreground transition-opacity hover:opacity-85"
                  to="/docs/installation"
                >
                  Installation
                </Link>
                <Link
                  className="rounded-full border border-fd-border px-6 py-3 font-medium transition-colors hover:bg-fd-accent"
                  to="/docs"
                >
                  Read the documentation
                </Link>
              </div>
            </div>
            <DynamicCodeBlock lang="sh" code={install} />
          </div>
        </section>
      </main>
      <Footer />
    </HomeLayout>
  );
}
