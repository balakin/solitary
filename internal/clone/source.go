// Package clone takes a cell definition out of a git repository and installs it
// as a cell of your own.
//
// A repository can be one cell — a cell.yaml at its root, the way a dotfiles
// repository is one configuration — or a catalogue of them, one directory each.
// Both are read the same way, and what lands is a copy: the cell is yours from
// then on, with no link back to where it came from.
//
// Nothing that arrives is trusted. A repository is written by someone else and
// its names become paths on this machine, so what may be copied is decided here
// rather than by whatever the archive happens to contain.
package clone

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Source is where a cell definition is to be read from.
type Source struct {
	// URL is what git is asked to clone. It may be a local path.
	URL string

	// Path is the directory inside the repository holding the cell, empty
	// when the caller did not name one. An empty Path is not the same as
	// the root: the layout decides, once the repository is on disk.
	Path string

	// Display is the source as it is worth showing back to someone —
	// shorthand expanded, credentials never introduced.
	Display string

	// Local reports that URL is a path on this machine rather than
	// somewhere to be reached. git clones one differently, and says so
	// loudly when asked for a shallow copy of it.
	Local bool
}

// Parse resolves what was typed into somewhere git can be pointed.
//
// Four forms are accepted, and the cell inside a repository can be named on any
// of them with a #fragment:
//
//	owner/repo               github shorthand
//	owner/repo/cell          shorthand, naming the cell directly
//	https://host/repo.git#cell   any git URL
//	./cells#claude           a local path, which is how this is tested
func Parse(arg string) (Source, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Source{}, fmt.Errorf("name a repository to clone from, e.g. 'solitary clone owner/repo'")
	}

	repo, cell, _ := strings.Cut(arg, "#")
	if repo == "" {
		return Source{}, fmt.Errorf("%q names a cell but no repository to find it in", arg)
	}

	if !shorthand(repo) {
		inside, err := clean(cell)
		if err != nil {
			return Source{}, err
		}

		return Source{URL: repo, Path: inside, Display: repo, Local: local(repo)}, nil
	}

	// Shorthand: the first two segments are the repository, since a GitHub
	// repository name cannot contain a slash, and anything after them is
	// the cell inside it.
	parts := strings.Split(repo, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Source{}, fmt.Errorf("%q is not a repository: use owner/repo, a git URL, or a path", arg)
	}
	if cell == "" {
		cell = strings.Join(parts[2:], "/")
	} else if len(parts) > 2 {
		return Source{}, fmt.Errorf("%q names a cell twice: drop one of the path or the #fragment", arg)
	}

	inside, err := clean(cell)
	if err != nil {
		return Source{}, err
	}
	owner, name := parts[0], parts[1]

	return Source{
		URL:     "https://github.com/" + owner + "/" + name + ".git",
		Path:    inside,
		Display: "github.com/" + owner + "/" + name,
	}, nil
}

// shorthand reports whether an argument is owner/repo rather than something git
// can be handed as it stands.
func shorthand(repo string) bool {
	switch {
	case local(repo):
		return false
	case strings.Contains(repo, "://"):
		return false // a URL
	case strings.Contains(repo, ":"):
		return false // scp-style, git@host:owner/repo
	default:
		return true
	}
}

// local reports whether a source is a path on this machine.
func local(repo string) bool {
	return strings.HasPrefix(repo, ".") || strings.HasPrefix(repo, "/") || strings.HasPrefix(repo, "~")
}

// clean normalises the cell path inside a repository and refuses one that would
// reach outside it. git will not produce such a path; it can still be typed.
func clean(cell string) (string, error) {
	cell = strings.Trim(slashed(cell), "/")
	if cell == "" {
		return "", nil
	}

	cleaned := path.Clean(cell)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%q leads out of the repository", cell)
	}

	return cleaned, nil
}

// DefaultName is what a cell cloned from this source is called when the caller
// does not say. It is the name of the cell's directory, or of the repository
// when the repository is itself one cell.
//
// The result is not necessarily a usable cell name — a repository can be called
// anything. The caller validates it and says so, rather than quietly mangling
// what someone published.
func (s Source) DefaultName() string {
	if s.Path != "" {
		return path.Base(s.Path)
	}

	return repoName(s.URL)
}

// repoName pulls the repository's own name out of a URL or path, whichever form
// it took.
func repoName(raw string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(raw, "/"), ".git")

	// A URL's path is what names the repository; its host and any query are
	// not part of the name.
	if u, err := url.Parse(trimmed); err == nil && u.Host != "" && u.Path != "" {
		trimmed = strings.TrimRight(u.Path, "/")
	} else if _, after, ok := strings.Cut(trimmed, ":"); ok && !strings.Contains(trimmed, "://") {
		trimmed = after // scp-style, git@host:owner/repo
	}

	return path.Base(slashed(trimmed))
}

// slashed turns a Windows-style separator into the one path.Base understands,
// so a local path on either kind of host names the same repository.
func slashed(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}
