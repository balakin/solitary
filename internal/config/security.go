package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	baseName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
	sizeName = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:[KMGTPE]i?B?|B)?$`)
	dnsLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

// ValidDomain accepts ASCII DNS names, including single-label/private names.
// International names must be supplied in their ASCII (punycode) form.
func ValidDomain(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	name = strings.TrimSuffix(name, ".")
	for _, label := range strings.Split(name, ".") {
		if !dnsLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func (v VM) Validate() error {
	if v.GPU != "" && !ValidGPU(v.GPU) {
		return fmt.Errorf("vm: unsafe host GPU path")
	}
	// Rendered into the definition, so held to a template name even though
	// solitary chose it.
	if !baseName.MatchString(v.Base) {
		return fmt.Errorf("vm.base: %q must be an image template name containing letters, digits, dots, underscores or dashes", v.Base)
	}
	if v.CPUs < 1 {
		return fmt.Errorf("vm.cpus must be positive")
	}
	for key, value := range map[string]string{"memory": v.Memory, "disk": v.Disk} {
		if !sizeName.MatchString(value) {
			return fmt.Errorf("vm.%s: invalid size %q", key, value)
		}
	}
	return nil
}

func (n Network) Validate() error {
	if err := n.validateResolvers(); err != nil {
		return err
	}
	if err := n.validateAllow(); err != nil {
		return err
	}
	if n.Tunnel != nil {
		return n.Tunnel.Validate()
	}
	return nil
}

func (t Tunnel) Validate() error {
	if ipv6(t.EndpointHost) {
		return fmt.Errorf("endpoint: IPv6 is not supported by the tunnel policy")
	}
	if net.ParseIP(t.EndpointHost) == nil && !ValidDomain(t.EndpointHost) {
		return fmt.Errorf("endpoint: invalid host %q", t.EndpointHost)
	}
	port, err := strconv.Atoi(t.EndpointPort)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != t.EndpointPort {
		return fmt.Errorf("endpoint: invalid UDP port %q", t.EndpointPort)
	}
	return nil
}

// LocalPath checks existing components without allowing a symlink to redirect
// a definition's relative path. The actual reads also use os.Root confinement.
func LocalPath(dir, name string) (string, error) {
	if !filepath.IsLocal(name) || name == "." || strings.Contains(name, `\`) {
		return "", fmt.Errorf("%q must stay inside the cell directory", name)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer r.Close()
	parts := strings.Split(filepath.Clean(name), string(filepath.Separator))
	for i := range parts {
		info, err := r.Lstat(filepath.Join(parts[:i+1]...))
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%q contains a symlink", name)
		}
	}
	return filepath.Join(dir, name), nil
}

// SensitiveName is a conservative exclusion shared by clone and build staging.
// It supplements context confinement; arbitrary files can still hold secrets.
func SensitiveName(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case ".env", "vpn.conf", ".ssh", ".aws", ".gnupg", ".kube", "credentials", "credentials.json":
		return true
	}
	if !strings.HasPrefix(n, ".env.") && (strings.HasSuffix(n, ".example") || strings.HasSuffix(n, ".sample")) {
		return false
	}
	return strings.HasPrefix(n, ".env.") || strings.HasPrefix(n, "id_") || strings.HasSuffix(n, ".env") || strings.Contains(n, ".env.") || strings.HasSuffix(n, ".pem") || strings.HasSuffix(n, ".key") || strings.HasSuffix(n, ".kubeconfig")
}

// Display renders controls visibly, preserving meaningful multiline script text.
func Display(s string) string {
	var out strings.Builder
	for _, r := range s {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.In(r, unicode.Cf) {
			fmt.Fprintf(&out, "\\u%04x", r)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
