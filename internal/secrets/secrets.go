// Package secrets manages the values a cell is allowed to see.
//
// Values live in a .env file next to the cell definition and never leave the
// host: they are passed to the container as environment variables when it
// starts. The file may hold more than a cell needs; only the names the cell
// declares are ever passed in.
package secrets

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"
)

// Load reads a .env file. A missing file is not an error: a cell that has never
// had secrets set simply has none.
func Load(path string) (map[string]string, error) {
	values := map[string]string{}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return values, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		key, value, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("%s:%d: malformed line", path, line)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return values, nil
}

// parseLine splits one line of a .env file. It reports ok=false for blank lines
// and comments, and key=="" for lines it cannot make sense of.
func parseLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")

	name, rest, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", true
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", true
	}

	return name, unquote(strings.TrimSpace(rest)), true
}

// unquote removes surrounding quotes and undoes the escaping applied by Save.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	switch {
	case strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`):
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		return strings.ReplaceAll(inner, `\\`, `\`)
	case strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`):
		return s[1 : len(s)-1]
	default:
		return s
	}
}

// Save writes values to a .env file readable only by its owner. Every value is
// quoted, so anything the user pasted survives a round trip.
func Save(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("# Written by solitary. Values stay on this machine and are passed\n")
	b.WriteString("# into the cell only when its cell.yaml declares the name.\n")
	for _, k := range keys {
		escaped := strings.ReplaceAll(values[k], `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		fmt.Fprintf(&b, "%s=\"%s\"\n", k, escaped)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// Missing returns the declared names that have no value yet, in declaration
// order.
func Missing(declared []string, values map[string]string) []string {
	var missing []string
	for _, name := range declared {
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// Env returns KEY=VALUE entries for the declared names that have values. It is
// the only path by which a value reaches a cell: anything in the .env file that
// the cell does not declare is left behind.
func Env(declared []string, values map[string]string) []string {
	env := make([]string, 0, len(declared))
	for _, name := range declared {
		if v, ok := values[name]; ok && v != "" {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// Set records one value, leaving every other value in the file untouched.
//
// It exists so that a caller changing a single secret never has to hold the
// others: it reads, replaces and writes in one step, and what it is given is
// the only value it sees.
func Set(path, name, value string) error {
	values, err := Load(path)
	if err != nil {
		return err
	}
	values[name] = value

	return Save(path, values)
}

// CanPrompt reports whether there is a terminal to ask on.
func CanPrompt() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Prompt asks for each name in turn and records what is typed. Input is not
// echoed. An empty answer keeps whatever value is already set, so re-running
// over a filled-in file is safe.
//
// It reports whether anything changed.
func Prompt(out io.Writer, names []string, values map[string]string) (bool, error) {
	if !CanPrompt() {
		return false, errors.New("no terminal to prompt on")
	}

	changed := false
	for _, name := range names {
		if values[name] != "" {
			fmt.Fprintf(out, "%s (set — enter to keep): ", name)
		} else {
			fmt.Fprintf(out, "%s: ", name)
		}

		typed, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(out)
		if err != nil {
			return changed, fmt.Errorf("reading %s: %w", name, err)
		}

		if v := strings.TrimSpace(string(typed)); v != "" {
			values[name] = v
			changed = true
		}
	}

	return changed, nil
}
