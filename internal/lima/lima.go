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

// ParamCell is the Lima parameter the cell's name is written into, and the
// closest thing a machine has to a label: Lima has no field for metadata, and
// refuses a parameter nothing reads, so this one is read by the provision
// script that writes GuestCellFile. limactl reports it back under
// config.param, which is how a machine can name its cell after the definition
// it was created from is gone.
const ParamCell = "solitary_cell"

// GuestCellFile is where a machine records the cell it was created for.
// Writing it is what makes ParamCell a parameter Lima accepts rather than a
// marker it rejects — and inside a cell it is the only thing that says which
// one this is.
const GuestCellFile = "/etc/solitary/cell"

// templateData is what cell.yaml.tmpl is rendered against.
type templateData struct {
	Name    string
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
	// cellParam and cellParamEnv name the parameter carrying the cell's name,
	// and the environment variable Lima puts it in when it runs a provision
	// script. Reading the parameter through the shell rather than through
	// Lima's own templating keeps one set of delimiters in this file: both
	// templates use the same ones, and a reference meant for Lima would be
	// resolved here instead.
	"cellParam":    func() string { return ParamCell },
	"cellParamEnv": func() string { return "PARAM_" + ParamCell },
	// guestCellFile is the path in the guest, from the same constant the
	// rest of solitary reads it from.
	"guestCellFile": func() string { return GuestCellFile },
}

// Render fills the embedded template with the resolved machine settings and
// returns a Lima machine definition.
//
// The cell's name is rendered into the definition as well as into the
// machine's name, so that a machine can still say which cell it belongs to
// when the definition it came from is gone.
func Render(name string, vm config.VM, ports []int, network config.Network) (string, error) {
	tmpl, err := template.New("cell").Funcs(funcs).Parse(cellTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing embedded template: %w", err)
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, templateData{Name: name, VM: vm, Ports: ports, Network: network}); err != nil {
		return "", fmt.Errorf("rendering machine definition: %w", err)
	}

	return out.String(), nil
}
