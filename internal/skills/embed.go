package skills

import (
	"embed"
	"io/fs"
)

// host holds the skills for an agent on the host, one directory each. They are
// compiled in rather than fetched, so the skills install writes always describe
// the binary that wrote them.
//
//go:embed host
var hostSkills embed.FS

// Host is the skills install puts in the directories host agents read.
func Host() fs.FS {
	sub, err := fs.Sub(hostSkills, "host")
	if err != nil {
		panic(err) // the directory is embedded above; it cannot be missing
	}
	return sub
}
