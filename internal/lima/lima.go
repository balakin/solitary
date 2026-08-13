// Package lima renders Lima machine definitions and drives limactl.
//
// The template is embedded in the binary: users describe a cell in cell.yaml
// and never write Lima YAML by hand.
package lima

import (
	_ "embed"

	"github.com/dm-balakin/solitary/internal/config"
)

//go:embed templates/cell.yaml.tmpl
var cellTemplate string

// Render fills the embedded template with the resolved machine settings and
// returns a Lima machine definition.
func Render(vm config.VM, ports []int) (string, error) {
	_ = cellTemplate // remove once this is implemented
	panic("not implemented")
}
