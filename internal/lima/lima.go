// Package lima renders Lima machine definitions and drives limactl.
//
// The template is embedded in the binary: users describe a cell in cell.yaml
// and never write Lima YAML by hand.
package lima

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/balakin/solitary/internal/config"
)

//go:embed templates/cell.yaml.tmpl
var cellTemplate string

// templateData is what cell.yaml.tmpl is rendered against.
type templateData struct {
	VM      config.VM
	Ports   []int
	Network config.Network
}

var funcs = template.FuncMap{
	// indent shifts every line of s right by n spaces, including the first,
	// so that a multi-line script can be dropped into a YAML block scalar.
	"indent": func(n int, s string) string {
		pad := strings.Repeat(" ", n)
		lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
		for i, line := range lines {
			if line == "" {
				continue
			}
			lines[i] = pad + line
		}
		return strings.Join(lines, "\n")
	},
	// join is strings.Join, for rendering a list into one line of a rule.
	"join": strings.Join,
	// vpnInterface and vpnConfig name the tunnel inside the machine. They
	// come from config so that the definition rendered here and the file
	// solitary later places in the machine cannot disagree.
	"vpnInterface": func() string { return config.VPNInterface },
	"vpnConfig":    func() string { return config.VPNConfigFile },
}

// Render fills the embedded template with the resolved machine settings and
// returns a Lima machine definition.
func Render(vm config.VM, ports []int, network config.Network) (string, error) {
	tmpl, err := template.New("cell").Funcs(funcs).Parse(cellTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing embedded template: %w", err)
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, templateData{VM: vm, Ports: ports, Network: network}); err != nil {
		return "", fmt.Errorf("rendering machine definition: %w", err)
	}

	return out.String(), nil
}
