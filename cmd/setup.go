package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jamescrawford/jamfschool2snipeIT/config"
	"github.com/jamescrawford/jamfschool2snipeIT/snipe"
)

func NewSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Create JAMF School custom fields in Snipe-IT",
		Long:  "Creates JAMF School-related custom fields in Snipe-IT, associates them with the configured fieldset, and writes the resulting field mappings into the config file.",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	if err := Cfg.ValidateSnipeIT(); err != nil {
		return err
	}
	if Cfg.SnipeIT.FieldsetID == 0 {
		return fmt.Errorf("snipe_it.fieldset_id must be set to use setup")
	}

	ctx, cancel := contextWithSignal()
	defer cancel()

	snipeClient, err := newSnipeClient()
	if err != nil {
		return err
	}

	fields := []snipe.FieldDef{
		{Name: "JAMF School: UDID", Element: "text", Format: "ANY", HelpText: "Unique device identifier from JAMF School"},
		{Name: "JAMF School: Last Check-in", Element: "text", Format: "ANY", HelpText: "Last device check-in from JAMF School"},
		{Name: "JAMF School: Model Identifier", Element: "text", Format: "ANY", HelpText: "Hardware model identifier from JAMF School"},
		{Name: "JAMF School: Model Type", Element: "listbox", Format: "ANY", HelpText: "Device type reported by JAMF School", FieldValues: "computer\ntablet\nphone\nappleTV"},
		{Name: "JAMF School: OS Name", Element: "text", Format: "ANY", HelpText: "Operating system family"},
		{Name: "JAMF School: OS Version", Element: "text", Format: "ANY", HelpText: "Operating system version"},
		{Name: "JAMF School: Managed", Element: "listbox", Format: "BOOLEAN", HelpText: "Whether the device is managed in JAMF School", FieldValues: "true\nfalse"},
		{Name: "JAMF School: Supervised", Element: "listbox", Format: "BOOLEAN", HelpText: "Whether the device is supervised in JAMF School", FieldValues: "true\nfalse"},
		{Name: "JAMF School: Battery Level", Element: "text", Format: "ANY", HelpText: "Battery level percentage from JAMF School"},
		{Name: "JAMF School: Total Capacity", Element: "text", Format: "NUMERIC", HelpText: "Reported storage capacity from JAMF School"},
		{Name: "JAMF School: Device Enroll Type", Element: "text", Format: "ANY", HelpText: "Enrollment type reported by JAMF School"},
		{Name: "JAMF School: Location ID", Element: "text", Format: "NUMERIC", HelpText: "JAMF School location ID"},
		{Name: "JAMF School: Location Name", Element: "text", Format: "ANY", HelpText: "JAMF School location name"},
		{Name: "JAMF School: Notes", Element: "textarea", Format: "ANY", HelpText: "Device notes from JAMF School"},
		{Name: "JAMF School: User Match", Element: "text", Format: "ANY", HelpText: "Heuristic user match used for Snipe-IT assignment"},
	}

	results, err := snipeClient.SetupFields(Cfg.SnipeIT.FieldsetID, fields)
	if err != nil {
		return fmt.Errorf("setting up fields: %w", err)
	}

	fieldSources := map[string]string{
		"JAMF School: UDID":               "udid",
		"JAMF School: Last Check-in":      "last_checkin",
		"JAMF School: Model Identifier":   "model_identifier",
		"JAMF School: Model Type":         "model_type",
		"JAMF School: OS Name":            "os_name",
		"JAMF School: OS Version":         "os_version",
		"JAMF School: Managed":            "managed",
		"JAMF School: Supervised":         "supervised",
		"JAMF School: Battery Level":      "battery_level",
		"JAMF School: Total Capacity":     "total_capacity",
		"JAMF School: Device Enroll Type": "device_enroll_type",
		"JAMF School: Location ID":        "location_id",
		"JAMF School: Location Name":      "location_name",
		"JAMF School: Notes":              "notes",
		"JAMF School: User Match":         "matched_user",
	}

	fieldMapping := make(map[string]string)
	replaceValues := make(map[string]bool)
	for name, dbCol := range results {
		if source, ok := fieldSources[name]; ok {
			fieldMapping[dbCol] = source
			replaceValues[source] = true
		}
	}

	if err := config.MergeFieldMapping(ConfigFile, fieldMapping, replaceValues); err != nil {
		return fmt.Errorf("saving field mappings to %s: %w", ConfigFile, err)
	}

	_ = ctx
	fmt.Printf("Created or updated %d custom fields and saved mappings to %s\n", len(results), ConfigFile)
	return nil
}
