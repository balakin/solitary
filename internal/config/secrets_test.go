package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseSecrets(t *testing.T, document string) (Secrets, error) {
	t.Helper()

	var parsed struct {
		Secrets Secrets `yaml:"secrets"`
	}
	err := yaml.Unmarshal([]byte(document), &parsed)

	return parsed.Secrets, err
}

func TestSecretsKeepDeclarationOrderAndDefaultToRequired(t *testing.T) {
	got, err := parseSecrets(t, `secrets:
  ZED:
  ALPHA:
    description: an api key
  SPARE:
    required: false
    description: only needed to push
`)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	want := Secrets{
		{Name: "ZED", Required: true},
		{Name: "ALPHA", Required: true, Description: "an api key"},
		{Name: "SPARE", Required: false, Description: "only needed to push"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("secrets = %+v, want %+v", got, want)
	}

	if names := got.Names(); !reflect.DeepEqual(names, []string{"ZED", "ALPHA", "SPARE"}) {
		t.Errorf("Names() = %v, want the order they were written in", names)
	}
	if names := got.RequiredNames(); !reflect.DeepEqual(names, []string{"ZED", "ALPHA"}) {
		t.Errorf("RequiredNames() = %v, want the two that are required", names)
	}
}

func TestSecretsAcceptAnEmptyDeclaration(t *testing.T) {
	got, err := parseSecrets(t, "secrets:\n")
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("secrets = %+v, want none", got)
	}
}

// The list form is what every definition written before the labels existed
// uses, so it has to be refused with the edit that fixes it rather than with
// whatever the yaml package would say.
func TestSecretsRefuseTheListFormWithTheFix(t *testing.T) {
	_, err := parseSecrets(t, "secrets:\n  - GITHUB_TOKEN\n")
	if err == nil {
		t.Fatal("unmarshal = nil, want the migration error")
	}
	if !strings.Contains(err.Error(), `"GITHUB_TOKEN:"`) {
		t.Errorf("error = %v, want it to show the line to write instead", err)
	}
}

func TestSecretsRejectWhatCannotBeAnEnvironmentVariable(t *testing.T) {
	for _, document := range []string{
		"secrets:\n  GITHUB-TOKEN:\n",
		"secrets:\n  2FA:\n",
		"secrets:\n  \"\":\n",
		"secrets: GITHUB_TOKEN\n",
	} {
		if _, err := parseSecrets(t, document); err == nil {
			t.Errorf("unmarshal(%q) = nil, want an error", document)
		}
	}
}

func TestSecretsRejectADuplicateName(t *testing.T) {
	_, err := parseSecrets(t, "secrets:\n  TOKEN:\n  TOKEN:\n    required: false\n")
	if err == nil {
		t.Fatal("unmarshal = nil, want an error naming the repeat")
	}
}
