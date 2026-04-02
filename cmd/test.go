package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func NewTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Test JAMF School and Snipe-IT connectivity",
		RunE:  runTest,
	}
}

func runTest(cmd *cobra.Command, args []string) error {
	if err := Cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jamfClient := newJamfClient()
	devices, err := jamfClient.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("testing JAMF School connection: %w", err)
	}

	snipeClient, err := newSnipeClient()
	if err != nil {
		return err
	}

	models, err := snipeClient.ListAllModels(ctx)
	if err != nil {
		return fmt.Errorf("testing Snipe-IT connection: %w", err)
	}

	if Cfg.Sync.LocationSync.Enabled {
		if _, err := snipeClient.ListAllLocations(ctx); err != nil {
			return fmt.Errorf("testing Snipe-IT locations: %w", err)
		}
		if _, err := jamfClient.GetLocations(ctx); err != nil {
			return fmt.Errorf("testing JAMF School locations: %w", err)
		}
	}

	if Cfg.Sync.UserAssignment.Enabled {
		if _, err := snipeClient.ListAllUsers(ctx); err != nil {
			return fmt.Errorf("testing Snipe-IT users: %w", err)
		}
		if _, err := jamfClient.GetUsers(ctx); err != nil {
			return fmt.Errorf("testing JAMF School users: %w", err)
		}
	}

	fmt.Printf("JAMF School OK: fetched %d devices\n", len(devices))
	fmt.Printf("Snipe-IT OK: fetched %d models\n", len(models))
	return nil
}
