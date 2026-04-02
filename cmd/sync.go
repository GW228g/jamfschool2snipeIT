package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	jsssync "github.com/jamescrawford/jamfschool2snipeIT/sync"
)

func NewSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync JAMF School devices into Snipe-IT",
		Long:  "Fetches devices from JAMF School and creates or updates corresponding assets in Snipe-IT.",
		RunE:  runSync,
	}

	cmd.Flags().Bool("force", false, "Ignore timestamps and always update matching assets")
	cmd.Flags().Bool("update-only", false, "Only update existing assets, never create new ones")
	cmd.Flags().String("serial", "", "Sync a single device by serial number")
	cmd.Flags().StringSlice("device-type", nil, "Limit sync to JAMF School device model types (for example: computer, tablet)")

	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	applyBoolFlag(cmd, "force", &Cfg.Sync.Force)
	applyBoolFlag(cmd, "update-only", &Cfg.Sync.UpdateOnly)

	if cmd.Flags().Changed("device-type") {
		Cfg.Sync.DeviceTypes, _ = cmd.Flags().GetStringSlice("device-type")
	}

	if err := Cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := contextWithSignal()
	defer cancel()

	jamfClient := newJamfClient()
	snipeClient, err := newSnipeClient()
	if err != nil {
		return err
	}

	engine := jsssync.NewEngine(jamfClient, snipeClient, Cfg)

	serial, _ := cmd.Flags().GetString("serial")
	var stats *jsssync.Stats
	if serial != "" {
		Cfg.Sync.Force = true
		stats, err = engine.RunSingle(ctx, serial)
	} else {
		stats, err = engine.Run(ctx)
	}
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Printf("\nSync Results:\n")
	fmt.Printf("  Total devices processed: %d\n", stats.Total)
	fmt.Printf("  Assets created:          %d\n", stats.Created)
	fmt.Printf("  Assets updated:          %d\n", stats.Updated)
	fmt.Printf("  Assets skipped:          %d\n", stats.Skipped)
	fmt.Printf("  Errors:                  %d\n", stats.Errors)
	fmt.Printf("  New models created:      %d\n", stats.ModelNew)

	return nil
}
