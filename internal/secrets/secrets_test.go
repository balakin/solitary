package secrets

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	values, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for a missing file", err)
	}
	if len(values) != 0 {
		t.Errorf("Load() = %v, want empty", values)
	}
}

func TestLoadParsesQuotingAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := `# a comment

PLAIN=value
QUOTED="with spaces"
SINGLE='single quoted'
ESCAPED="say \"hi\""
export EXPORTED=exported
EMPTY=
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := map[string]string{
		"PLAIN":    "value",
		"QUOTED":   "with spaces",
		"SINGLE":   "single quoted",
		"ESCAPED":  `say "hi"`,
		"EXPORTED": "exported",
		"EMPTY":    "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v", got, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	want := map[string]string{
		"TOKEN":   `sk-ant-"quoted"\and\backslashes`,
		"SPACED":  "a value with spaces",
		"SIMPLE":  "simple",
		"SPECIAL": "$dollar `backtick` #hash",
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed values:\n got %#v\nwant %#v", got, want)
	}
}

func TestSaveIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := Save(path, map[string]string{"A": "b"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("secrets file mode = %o, want 600", perm)
	}
}

func TestEnvPassesOnlyDeclaredNames(t *testing.T) {
	values := map[string]string{
		"DECLARED":     "yes",
		"ALSO":         "yes",
		"NOT_DECLARED": "must not leak",
		"BLANK":        "",
	}

	got := Env([]string{"DECLARED", "ALSO", "BLANK", "NEVER_SET"}, values)

	want := []string{"DECLARED=yes", "ALSO=yes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Env() = %v, want %v", got, want)
	}
	for _, kv := range got {
		if kv == "NOT_DECLARED=must not leak" {
			t.Error("Env() passed a value the cell does not declare")
		}
	}
}

func TestMissingReportsUnsetDeclaredNames(t *testing.T) {
	values := map[string]string{"SET": "value", "BLANK": ""}

	got := Missing([]string{"SET", "BLANK", "ABSENT"}, values)

	want := []string{"BLANK", "ABSENT"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestLabelSaysWhatEnterDoes(t *testing.T) {
	cases := []struct {
		field Field
		set   bool
		want  string
	}{
		{Field{Name: "TOKEN"}, false, "TOKEN: "},
		{Field{Name: "TOKEN"}, true, "TOKEN (set — enter to keep): "},
		{Field{Name: "SPARE", Optional: true}, false, "SPARE (optional, enter to skip): "},
		// A value it already has is kept the same way whether or not the
		// cell can do without it.
		{Field{Name: "SPARE", Optional: true}, true, "SPARE (set — enter to keep): "},
	}

	for _, c := range cases {
		if got := label(c.field, c.set); got != c.want {
			t.Errorf("label(%+v, set=%v) = %q, want %q", c.field, c.set, got, c.want)
		}
	}
}

// A secrets file created by hand, or by an older release, can be readable by
// other users. Saving to it is the moment to take that back.
func TestSaveTightensAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("OLD=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(path, map[string]string{"TOKEN": "secret"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("Save() left mode %o, want 600", got)
	}
}

func TestSaveDoesNotFollowASymlink(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "unrelated"), filepath.Join(dir, ".env")
	if err := os.WriteFile(target, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_ = Save(link, map[string]string{"TOKEN": "secret"})
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged" {
		t.Errorf("Save() wrote %q through a symlink", data)
	}
}
