package config

import "testing"

func TestDeviceTypeAllowed(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{DeviceTypes: []string{"computer", "tablet"}}
	if !cfg.DeviceTypeAllowed("computer") {
		t.Fatal("expected computer to be allowed")
	}
	if cfg.DeviceTypeAllowed("phone") {
		t.Fatal("expected phone to be rejected")
	}
}
