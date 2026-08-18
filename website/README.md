# Solitary website

The documentation site for [solitary](https://github.com/dm-balakin/solitary), built with Fumadocs, Vite, React Router, and pnpm.

## Requirements

- Node.js 24 (`>=24 <25`)
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

## Production build

```sh
pnpm build
pnpm start
```

The site documents the problem, architecture, security model, capabilities, configuration, and trade-offs. Solitary itself is pre-alpha and has no released binaries, so its installation page documents a build from source.

Documentation claims are written from the Go source in the parent repository rather than from intent. When changing `internal/config`, `internal/cli`, or `internal/dashboard`, check `content/docs/configuration.mdx`, `commands.mdx`, and `dashboard.mdx` against it.
