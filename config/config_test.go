package config

import (
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{
		JAMFSchool: JAMFSchoolConfig{
			URL:       "https://school.jamfcloud.com",
			NetworkID: "12345",
			APIKey:    "real-api-key",
		},
		SnipeIT: SnipeITConfig{
			URL:             "https://snipe.example.org",
			APIKey:          "real-snipe-key",
			ManufacturerID:  1,
			DefaultStatusID: 2,
			CategoryID:      3,
		},
	}
}

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

func TestValidateValidConfig(t *testing.T) {
	t.Parallel()

	if err := validConfig().Validate(); err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateMissingJamfAPIKey(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.JAMFSchool.APIKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing JAMF API key")
	}
	if !strings.Contains(err.Error(), "jamf_school.api_key") {
		t.Fatalf("expected jamf api key in error, got: %v", err)
	}
}

func TestValidatePlaceholderAPIKey(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.JAMFSchool.APIKey = "your-jamf-school-api-key"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected placeholder api key to fail validation")
	}
}

func TestValidateHTTPNotHTTPS(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.JAMFSchool.URL = "http://school.jamfcloud.com"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected non-https url to fail validation")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https mention in error, got: %v", err)
	}
}

func TestValidateBetaUserAssignmentMissingGate(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Sync.BetaUserAssignment.Enabled = true
	cfg.Sync.BetaUserAssignment.Beta = false
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected beta gate failure")
	}
	if !strings.Contains(err.Error(), "beta") {
		t.Fatalf("expected beta mention in error, got: %v", err)
	}
}

func TestValidateMissingSnipeURL(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.SnipeIT.URL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing snipe url to fail validation")
	}
}

func TestValidateInvalidManufacturerID(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.SnipeIT.ManufacturerID = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected manufacturer id zero to fail validation")
	}
}
