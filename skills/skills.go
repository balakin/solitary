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
