package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseAgents(t *testing.T, document string) (Agents, error) {
	t.Helper()

	var parsed struct {
		Agents Agents `yaml:"agents"`
	}
	err := yaml.Unmarshal([]byte(document), &parsed)

	return parsed.Agents, err
}

func TestAgentsTellAbsentFromEmptyFromAll(t *testing.T) {
	for document, want := range map[string]Agents{
		"image: alpine\n":   {},
		"agents:\n":         {},
		"agents: []\n":      {Set: true},
		"agents: all\n":     {Set: true, All: true},
		"agents: [codex]\n": {Set: true, List: []Agent{{Name: "codex"}}},
	} {
		got, err := parseAgents(t, document)
		if err != nil {
			t.Errorf("%q: %v", document, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %+v, want %+v", document, got, want)
		}
	}
}

func TestAgentsTakeIDsCustomAgentsAndOverrides(t *testing.T) {
	got, err := parseAgents(t, `agents:
  - claude-code
  - name: codex
    skills: ~/.config/codex/skills/
  - name: my-agent
    skills: ~/my-agent/skills
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Agent{
		{Name: "claude-code"},
		{Name: "codex", Skills: ".config/codex/skills"},
		{Name: "my-agent", Skills: "my-agent/skills"},
	}
	if !reflect.DeepEqual(got.List, want) {
		t.Errorf("List = %+v, want %+v", got.List, want)
	}
}

func TestAgentsRefuseWhatTheyCannotPlace(t *testing.T) {
	for document, wantErr := range map[string]string{
		"agents: some\n":                                    `not "some"`,
		"agents: [claude]\n":                                `unknown agent "claude"`,
		"agents:\n  - name: my-agent\n":                     `unknown agent "my-agent"`,
		"agents: [codex, codex]\n":                          "listed twice",
		"agents:\n  - name: My Agent\n    skills: ~/x\n":    "not an agent name",
		"agents:\n  - name: a\n    skills: /home/cell/x\n":  "under the home",
		"agents:\n  - name: a\n    skills: ~/../etc\n":      "under the home",
		"agents:\n  - name: a\n    skills: ~/x/../../etc\n": "under the home",
		"agents:\n  - name: a\n    skills: ~//etc\n":        "under the home",
		"agents:\n  - name: a\n    skills: \"~\"\n":         "under the home",
		"agents:\n  - name: a\n    skills: ~/\n":            "under the home",
		"agents:\n  - [codex]\n":                            "each entry",
	} {
		_, err := parseAgents(t, document)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%q: error = %v, want one containing %q", document, err, wantErr)
		}
	}
}
