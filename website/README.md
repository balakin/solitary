# Solitary website

The documentation site for [solitary](https://github.com/balakin/solitary), built with Fumadocs, Vite, React Router, and pnpm.

## Requirements

- Node.js 24 (`>=24 <25`); `.nvmrc` pins the exact release that CI and `nvm use` install
- Corepack

pnpm is not installed by hand. `packageManager` pins the exact version **and a
SHA-512 of its tarball**, so corepack fetches that one and refuses anything whose
contents do not hash to the pinned value. Change it with `corepack use pnpm@<version>`,
which rewrites the field with a fresh hash, rather than by editing the version by hand.

## Fonts

Every face is served from this origin. There is no request to Google Fonts or any other
third party, so reading the docs does not announce itself to anyone but the host serving
them. Both families are OFL 1.1, with their licences beside the files in `public/fonts/`.

| Family                        | Files                                                     | Used for                              |
| ----------------------------- | --------------------------------------------------------- | ------------------------------------- |
| Inter v4.1 (variable, subset) | `InterVariable.woff2`, `InterVariable-Italic.woff2`       | body text (`--font-sans`)             |
| JetBrains Mono v2.304         | `JetBrainsMono-Regular.woff2`, `JetBrainsMono-Bold.woff2` | code and terminal art (`--font-mono`) |

Inter is shipped as the variable face, so `wght` 100–900 and `opsz` 14–32 both work; the
`html` rule in `app/app.css` leaves `font-optical-sizing` on.

### Why they are not loaded from Google Fonts

The Google Fonts API serves faces subsetted by `unicode-range`, and its `latin` subset stops
well before U+2500. The home page draws terminal screenshots out of box-drawing characters,
so those glyphs would fall back to a system font with different metrics and the frames would
not join up. The default monospace stack fails the same way: on most Linux hosts it lands on
Liberation Mono or Courier New, neither of which has the block.

The terminal art also depends on a tight line-height — `│` is 1.52em tall, so any line
advance above that breaks the verticals into dashes. See `app/components/terminal.tsx`.

### Regenerating the Inter subset

Inter ships at 352KB + 388KB for the two variable faces. They are cut to the ranges this
site writes in, which is about a third of that. To rebuild from an
[upstream release](https://github.com/rsms/inter/releases):

```sh
pip install fonttools brotli
RANGES='U+0000-00FF,U+0100-017F,U+0192,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20A0-20CF,U+2100-214F,U+2190-21FF,U+2212,U+2215,U+2260,U+2264-2265,U+FEFF,U+FFFD'
pyftsubset InterVariable.woff2 --output-file=public/fonts/InterVariable.woff2 \
  --flavor=woff2 --unicodes="$RANGES" --layout-features='*' --notdef-outline
```

The ranges deliberately exclude box drawing: those characters only ever appear inside a
`<pre>` or `<code>`, where the monospace face renders them. If you add prose in a script the
subset does not cover, its characters will fall back to a system font — widen `RANGES` and
regenerate rather than letting that happen quietly.

JetBrains Mono is shipped whole, since the art depends on glyphs a subset would be likely to
drop.

## Development

```sh
corepack enable pnpm
pnpm install
pnpm dev
```

## Linting and formatting

[oxlint](https://oxc.rs/docs/guide/usage/linter) and [oxfmt](https://oxc.rs/docs/guide/usage/formatter)
do both jobs, so there is no ESLint or Prettier here and nothing to keep in step between them.

```sh
pnpm lint        # oxlint
pnpm lint:fix    # apply the fixes it is sure about
pnpm fmt         # rewrite files in place
pnpm fmt:check   # fail instead of rewriting, for CI
pnpm types:check # react-router typegen && tsc --noEmit
```

`oxfmt` formats the TypeScript under `app/` and the Markdown and MDX under `content/`, including
the JSX inside it. It sorts Tailwind class lists the way `prettier-plugin-tailwindcss` does, reading
the theme from `app/app.css`, so class order is decided rather than argued about. `proseWrap` is
left at `preserve`: prose keeps whatever line breaks it was written with.

`oxlint` runs the `correctness`, `suspicious` and `perf` categories as errors. Two rules are off in
`.oxlintrc.json`, both because of how this project is built rather than as taste: `react-in-jsx-scope`
(the automatic JSX runtime needs no `React` import) and `no-unassigned-import` (`app/app.css` is
imported for its side effect). Suppress anything else at the line that needs it, with a comment
saying why, rather than by widening the config.

## Analytics

The production site counts pageviews and three clicks with [Umami Cloud](https://umami.is)
(EU region). There is no telemetry anywhere else — not in the CLI, not in a cell — and the
[privacy policy](https://solitary.balakin.io/privacy) (`app/routes/privacy.tsx`) says so; change it
along with anything here that changes what is collected.

`app/lib/analytics.ts` builds the one script tag `app/root.tsx` puts in every page's `<head>`. It
is emitted only when the build is a production build **and** `VITE_UMAMI_WEBSITE_ID` is set, so
`pnpm dev` and a build without the ID — a fork's, a pull request's — load nothing. Even with the ID,
`data-domains` keeps the tracker silent on any host but `solitary.balakin.io`, so a production build
served from localhost sends nothing either. It also drops query strings and fragments, and honours
Do Not Track. The tracker sets no cookie and counts client-side navigations itself.

The ID is public — it is in every page — so it is a repository **variable**, not a secret:
`VITE_UMAMI_WEBSITE_ID` under Settings → Secrets and variables → Actions → Variables, which
`pages.yml` passes to the build. A deploy without it still ships, with a warning on the run. To
build with it locally:

```sh
VITE_UMAMI_WEBSITE_ID=<id> pnpm build:pages
```

| Event            | Sent when someone                                                                                           |
| ---------------- | ----------------------------------------------------------------------------------------------------------- |
| `install_click`  | copies the install command, or follows "Create your first cell" or "other ways to install" on the home page |
| `github_click`   | follows "View on GitHub" on the home page or "GitHub" in the footer                                         |
| `feedback_click` | reserved; the site has no feedback link yet                                                                 |

`install_click` measures interest, not installs. The GitHub icon in the header is fumadocs' own and
is not counted.

To track another click, name it in `AnalyticsEvent`, then:

- on a link that leaves the site, add `data-umami-event="<name>"` (keep `target="_blank"`);
- on a link that stays, call `trackEvent('<name>')` from `onClick`. The attribute is wrong there:
  Umami cancels the click and reloads the page at the link's URL, so the client-side navigation is
  lost.

Never attach data to an event. `pnpm test` checks the built pages, so run it after `pnpm build:pages`.

## Production build

```sh
pnpm build        # prerenders every route into build/client
pnpm start        # serves that with a node server

pnpm build:pages  # the same build, plus the 404.html GitHub Pages serves
```

The deployed site is static: `pnpm build` prerenders every route, search included, so nothing
runs `pnpm start` in production. `pnpm build:pages` is what CI publishes — the same build, with
the catch-all route renamed to the `404.html` Pages answers unknown paths with. The root
`CLAUDE.md` explains what a static host still needs from the app.

The site documents the problem, architecture, security model, capabilities, configuration, and trade-offs. Its installation page covers both the released binaries and a build from source.

Documentation claims are written from the Go source in the parent repository rather than from intent. When changing `internal/config`, `internal/cli`, or `internal/dashboard`, check `content/docs/configuration.mdx`, `commands.mdx`, and `dashboard.mdx` against it.
