package skills

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEverySkillNamesItsDirectory(t *testing.T) {
	for set, src := range map[string]fs.FS{"host": Host(), "cell": Cell()} {
		entries, err := fs.ReadDir(src, ".")
		if err != nil || len(entries) == 0 {
			t.Fatalf("%s: %d skills, %v", set, len(entries), err)
		}
		for _, entry := range entries {
			data, err := fs.ReadFile(src, entry.Name()+"/SKILL.md")
			if err != nil {
				t.Errorf("%s/%s: %v", set, entry.Name(), err)
				continue
			}
			text := string(data)

			// The spec every agent reads requires the name to be the
			// directory's, and solitary owns the prefix.
			if !strings.HasPrefix(text, "---\nname: "+entry.Name()+"\ndescription: ") || !strings.HasPrefix(entry.Name(), "solitary-") {
				t.Errorf("%s/%s/SKILL.md does not open with its own solitary- name and a description", set, entry.Name())
			}

			// A cell's skills describe what up sets up in a cell, so npx
			// must not offer them to a host.
			internal := strings.Contains(text, "\nmetadata:\n  internal: true\n")
			if internal != (set == "cell") {
				t.Errorf("%s/%s: metadata.internal is %v", set, entry.Name(), internal)
			}
		}
	}
}
