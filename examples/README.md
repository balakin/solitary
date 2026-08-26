# Example cells

Two complete cell definitions, meant to be taken rather than read. Each is a
`cell.yaml`, the `Containerfile` it builds from, and a `skel/` directory that is
seeded into the cell's home the first time it starts.

```sh
solitary clone balakin/solitary/examples/claude   # claude, neovim, tmux, workmux, node
solitary clone balakin/solitary/examples/vscode   # openvscode-server in the browser, node, claude
```

`clone` shows you what a definition asks for before installing it, and installs
a copy: the cell is yours from then on, with no link back here. `--as <name>`
takes one under a different name.

Neither declares a secret and neither restricts the network, so both run as
they stand. [Example cells](https://solitary.balakin.io/docs/guides-examples)
covers what is inside them, the allow list to paste in when you want one, and
what to change first.
