package lima

import (
	"encoding/json"
	"testing"
)

// limactl reports the whole definition back with each machine, and the
// parameter solitary wrote into it comes with it. This is one line of real
// limactl list --json output, trimmed to the fields that are read.
func TestInstanceCell(t *testing.T) {
	const line = `{"name":"solitary-probe","status":"Running","dir":"/home/u/.lima/solitary-probe",` +
		`"config":{"param":{"internal_netplanOptional":"true","solitary_cell":"probe"}}}`

	var inst Instance
	if err := json.Unmarshal([]byte(line), &inst); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	name, ok := inst.Cell()
	if !ok || name != "probe" {
		t.Errorf("Cell() = %q, %v, want %q, true", name, ok, "probe")
	}
}

// A machine created before solitary wrote the parameter says nothing about
// which cell it belongs to, which has to be an answer rather than an empty
// name that reads like one.
func TestInstanceCellOfAnUnmarkedMachine(t *testing.T) {
	var inst Instance
	if err := json.Unmarshal([]byte(`{"name":"solitary-old","config":{"param":{}}}`), &inst); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	if name, ok := inst.Cell(); ok {
		t.Errorf("Cell() = %q, true, want no name", name)
	}
}
