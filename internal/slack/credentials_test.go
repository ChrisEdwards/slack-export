package slack

import (
	"regexp"
	"testing"
)

func TestGetMachineID(t *testing.T) {
	id, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() error = %v", err)
	}

	if id == "" {
		t.Error("GetMachineID() returned empty string")
	}

	// UUID format: XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX (with or without hyphens)
	// machineid may return lowercase hex without hyphens on some platforms
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)
	if !uuidPattern.MatchString(id) {
		t.Errorf("GetMachineID() = %q, does not match UUID pattern", id)
	}
}

func TestGetMachineID_Consistent(t *testing.T) {
	id1, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() first call error = %v", err)
	}

	id2, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() second call error = %v", err)
	}

	if id1 != id2 {
		t.Errorf("GetMachineID() not consistent: %q != %q", id1, id2)
	}
}
