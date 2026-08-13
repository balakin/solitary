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
