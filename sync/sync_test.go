package sync

import (
	"strings"
	"testing"

	"github.com/Jamf-Concepts/jamfschool-go-sdk/jamfschool"
	snipeit "github.com/michellepellon/go-snipeit"

	"github.com/jamescrawford/jamfschool2snipeIT/config"
)

func TestApplyFieldMapping(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		cfg: &config.Config{
			Sync: config.SyncConfig{
				FieldMapping: map[string]string{
					"_snipeit_udid_1": "udid",
					"_snipeit_os_2":   "os_version",
					"_snipeit_loc_3":  "location_name",
					"asset_tag":       "asset_tag",
				},
			},
		},
	}

	asset := snipeit.Asset{
		CommonFields: snipeit.CommonFields{CustomFields: map[string]string{}},
	}
	device := jamfschool.Device{
		UDID:         "UDID-001",
		AssetTag:     "LAB-001",
		SerialNumber: "C02X1234",
		OS:           jamfschool.DeviceOS{Version: "14.4"},
	}

	engine.applyFieldMapping(&asset, device, "teacher@example.org", "Middle School")

	if got := asset.AssetTag; got != "LAB-001" {
		t.Fatalf("expected asset tag LAB-001, got %q", got)
	}
	if got := asset.CustomFields["_snipeit_udid_1"]; got != "UDID-001" {
		t.Fatalf("expected UDID custom field, got %q", got)
	}
	if got := asset.CustomFields["_snipeit_os_2"]; got != "14.4" {
		t.Fatalf("expected OS version custom field, got %q", got)
	}
	if got := asset.CustomFields["_snipeit_loc_3"]; got != "Middle School" {
		t.Fatalf("expected location name custom field, got %q", got)
	}
}

func TestApplyManagedNotesPreservesManualText(t *testing.T) {
	t.Parallel()

	asset := snipeit.Asset{
		CommonFields: snipeit.CommonFields{
			Notes: "Manual note",
		},
	}
	device := jamfschool.Device{
		Name:         "iPad 1",
		SerialNumber: "C02X1234",
		UDID:         "UDID-001",
		Model:        jamfschool.DeviceModel{Name: "iPad", Identifier: "iPad14,1", Type: "tablet"},
		OS:           jamfschool.DeviceOS{Prefix: "iPadOS", Version: "18.0"},
	}

	applyManagedNotes(&asset, device, "teacher@example.org", "Middle School")

	if !strings.Contains(asset.Notes, "Manual note") {
		t.Fatal("expected manual note to be preserved")
	}
	if !strings.Contains(asset.Notes, managedNotesStart) {
		t.Fatal("expected managed notes block to be added")
	}
}
