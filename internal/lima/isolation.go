package lima

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CheckIsolation checks a stored machine before Solitary reuses it.
func CheckIsolation(instance Instance) error {
	if err := checkGlobalConfig(); err != nil {
		return err
	}
	stored, err := instance.StoredDefinition()
	if err != nil {
		return err
	}
	return checkDefinition(stored)
}

// StoredDefinition is the definition a machine boots from: the one Lima keeps
// in the machine's directory and reads again at every start.
func (i Instance) StoredDefinition() (string, error) {
	if i.Dir == "" {
		return "", fmt.Errorf("cannot read the definition of %s: Lima did not report its directory", i.Name)
	}
	data, err := os.ReadFile(filepath.Join(i.Dir, "lima.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading the definition of %s: %w", i.Name, err)
	}

	return string(data), nil
}

// Global Lima defaults can merge lists into an explicit empty mounts list.
// Solitary owns its complete machine policy, so global machine configuration
// is rejected rather than allowing that policy to change invisibly.
func checkGlobalConfig() error {
	home := os.Getenv("LIMA_HOME")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		home = filepath.Join(user, ".lima")
	}
	for _, name := range []string{"default.yaml", "override.yaml"} {
		path := filepath.Join(home, "_config", name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var settings map[string]any
		if err := yaml.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if len(settings) != 0 {
			return fmt.Errorf("%s supplies global Lima settings; Solitary requires an isolated LIMA_HOME without default.yaml or override.yaml settings", path)
		}
	}
	return nil
}

func checkDefinition(definition string) error {
	var doc struct {
		Mounts   []any `yaml:"mounts"`
		Copy     []any `yaml:"copyToHost"`
		Networks []any `yaml:"networks"`
		Forwards []struct {
			HostIP      string `yaml:"hostIP"`
			HostSocket  string `yaml:"hostSocket"`
			GuestSocket string `yaml:"guestSocket"`
			Reverse     bool   `yaml:"reverse"`
		} `yaml:"portForwards"`
	}
	if err := yaml.Unmarshal([]byte(definition), &doc); err != nil {
		return err
	}
	if len(doc.Mounts)+len(doc.Copy)+len(doc.Networks) != 0 {
		return errors.New("unsafe Lima definition: host mounts, copyToHost and additional networks are forbidden")
	}
	for _, f := range doc.Forwards {
		if (f.HostIP != "" && f.HostIP != "127.0.0.1" && f.HostIP != "::1") || f.HostSocket != "" || f.GuestSocket != "" || f.Reverse {
			return errors.New("unsafe Lima definition: port forwards must target host loopback without socket forwarding")
		}
	}
	return nil
}
