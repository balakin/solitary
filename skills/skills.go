// Package skills holds the agent skills solitary ships. They live at the root of
// the repository rather than under internal/ because two things read them from
// here: the binary, which embeds them so the skills it installs always describe
// the binary that installed them, and `npx skills add balakin/solitary`, which
// finds them by walking skills/<category>/<name>/SKILL.md.
package skills

import (
	"embed"
	"io/fs"
)

//go:embed host
var host embed.FS

// Host is the skills for an agent on the host, the one that drives solitary. Each
// top-level directory is one skill.
func Host() fs.FS {
	sub, err := fs.Sub(host, "host")
	if err != nil {
		panic(err) // the directory is embedded above; it cannot be missing
	}
	return sub
}

//go:embed cell
var cell embed.FS

// Cell is the skills for an agent inside a cell, which up installs there. They
// describe the artifact tool, the inbox and the firewall that the same up sets
// up, so they never travel on their own: each carries metadata.internal, which
// keeps `npx skills add` from offering them to a host that has none of those.
func Cell() fs.FS {
	sub, err := fs.Sub(cell, "cell")
	if err != nil {
		panic(err) // the directory is embedded above; it cannot be missing
	}
	return sub
}
