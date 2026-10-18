package podman

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/balakin/solitary/internal/lima"
)

// idRange is one line of the table podman starts a rootless container with:
// Size ids from ContainerID on are the machine's ids from HostID on.
type idRange struct {
	ContainerID int `json:"container_id"`
	HostID      int `json:"host_id"`
	Size        int `json:"size"`
}

// idMappings is that table for users and for groups, as podman info reports it.
type idMappings struct {
	UIDs []idRange `json:"uidmap"`
	GIDs []idRange `json:"gidmap"`
}

// machineID is the id on the machine that an id inside the container is.
func machineID(ranges []idRange, id int) (int, bool) {
	for _, r := range ranges {
		if id >= r.ContainerID && id < r.ContainerID+r.Size {
			return r.HostID + id - r.ContainerID, true
		}
	}

	return 0, false
}

// viewRule is the X-mount.idmap rule of the view a cell's user is given its
// home through.
//
// The container runs under podman's default table, where its root is the
// machine's user and every other id is one of the machine's subordinate ids.
// That is the table the image's layers were stored under, which is why podman
// has nothing to copy. The home, though, is the machine user's, so the view
// swaps two ids: what the machine user owns appears as the cell's user, and
// what the container's root writes lands on the first subordinate id. That is
// the disk keep-id left behind, so a home moves between the two unchanged.
func viewRule(m idMappings, uid, gid int) (string, error) {
	var rule []string
	for _, kind := range []struct {
		prefix, name string
		ranges       []idRange
		id           int
	}{
		{"u", "user", m.UIDs, uid},
		{"g", "group", m.GIDs, gid},
	} {
		owner, ok := machineID(kind.ranges, 0)
		if !ok {
			return "", errors.New("podman reports no id for the container's root")
		}
		displaced, ok := machineID(kind.ranges, 1)
		if !ok {
			return "", errors.New("podman runs containers without subordinate ids to map a user onto")
		}
		cell, ok := machineID(kind.ranges, kind.id)
		if !ok {
			return "", fmt.Errorf("the cell's %s has the id %d, which is outside the ids a rootless container can map", kind.name, kind.id)
		}
		rule = append(rule, fmt.Sprintf("%[1]s:%[2]d:%[3]d:1 %[1]s:%[4]d:%[2]d:1", kind.prefix, owner, cell, displaced))
	}

	return strings.Join(rule, " "), nil
}

// homeRule reads the table rootless containers start with in this machine
// and works out the view rule for a user of the image.
func homeRule(instance string, uid, gid int) (string, error) {
	out, err := lima.Exec(instance, "podman", "info", "--format", "{{json .Host.IDMappings}}")
	if err != nil {
		return "", fmt.Errorf("reading the ids podman maps: %w", err)
	}

	var m idMappings
	if err := json.Unmarshal(out, &m); err != nil {
		return "", fmt.Errorf("reading the ids podman maps: %w", err)
	}

	return viewRule(m, uid, gid)
}

// mountHomeView makes a second door onto the home, through which it belongs
// to the cell's user, and returns its path.
//
// Rootless podman cannot make this itself — an idmapped mount wants
// CAP_SYS_ADMIN over the filesystem, and the machine's disk belongs to the
// initial namespace — so the machine's root does. The mount is gone after a
// reboot, which is why it is checked before every start rather than once.
//
// The rule is in the path, since a mount cannot be asked which rule it was
// made with: a user whose id moved gets a view of its own rather than one that
// only looks right. A view no container uses any more is harmless, and the
// next reboot takes it.
//
// What solitary does to the home from the machine — fetch, send, the artifact
// tool — goes through the home itself and never through here: the machine's
// user is not in the rule, so the view refuses what it writes.
func mountHomeView(instance, home, rule string) (string, error) {
	sum := sha256.Sum256([]byte(rule))
	view := path.Join(path.Dir(home), ".cell-idmap-"+hex.EncodeToString(sum[:6]))

	if _, err := lima.Exec(instance, "mountpoint", "-q", view); err == nil {
		return view, nil
	}
	if _, err := lima.Exec(instance, "mkdir", "-p", view); err != nil {
		return "", fmt.Errorf("creating %s in the machine: %w", view, err)
	}
	if _, err := lima.Exec(instance, "sudo", "mount", "--bind", "-o", "X-mount.idmap="+rule, home, view); err != nil {
		return "", fmt.Errorf("mounting the home for the cell's user: %w", err)
	}

	return view, nil
}
