package cli

import (
	"fmt"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/spf13/cobra"
)

func newSetupCommand() *cobra.Command {
	registry := adapters.Default()
	var all, yes, dryRun, jsonOutput bool
	command := &cobra.Command{Use: "setup [adapter]", Short: "Set up agent auto-capture integrations", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		items := registry.List()
		if len(args) == 1 {
			adapter, found := registry.Get(args[0])
			if !found {
				return fmt.Errorf("adapter %q not found", args[0])
			}
			items = []adapters.Adapter{adapter}
		} else if !all {
			items = setupDefaultAdapters(items)
		}

		plans := make([]adapters.SetupPlan, 0, len(items))
		for _, adapter := range items {
			if adapter.Descriptor().ID == "generic-jsonl" {
				continue
			}
			var plan adapters.SetupPlan
			var err error
			if dryRun || !yes {
				plan, err = adapter.PlanInstall(command.Context(), adapters.SetupOptions{DryRun: true, Yes: yes})
			} else {
				result, installErr := adapter.Install(command.Context(), adapters.InstallOptions{})
				if installErr != nil {
					return installErr
				}
				plan, err = adapter.PlanInstall(command.Context(), adapters.SetupOptions{Yes: yes})
				plan.Changes = installResultChanges(adapter.Descriptor().ID, result)
			}
			if err != nil {
				return err
			}
			plans = append(plans, plan)
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), plans)
		}
		for _, plan := range plans {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | capture=%s\n", plan.AdapterID, plan.State, plan.CaptureQuality); err != nil {
				return err
			}
			for _, change := range plan.Changes {
				if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "  %s %s\n", change.Action, change.Path); err != nil {
					return err
				}
			}
		}
		return nil
	}}
	command.Flags().BoolVar(&all, "all", false, "include all known setup-capable adapters")
	command.Flags().BoolVar(&yes, "yes", false, "apply setup changes without prompting")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without writing files")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func setupDefaultAdapters(items []adapters.Adapter) []adapters.Adapter {
	result := make([]adapters.Adapter, 0, len(items))
	for _, item := range items {
		if item.Descriptor().ID != "generic-jsonl" {
			result = append(result, item)
		}
	}
	return result
}

func installResultChanges(adapterID string, result adapters.InstallResult) []adapters.SetupChange {
	changes := make([]adapters.SetupChange, 0, len(result.Actions))
	for _, action := range result.Actions {
		changes = append(changes, adapters.SetupChange{Action: actionAction(result.Changed), Description: action})
	}
	if len(changes) == 0 {
		changes = append(changes, adapters.SetupChange{Action: "unchanged", Description: "no setup changes"})
	}
	_ = adapterID
	return changes
}

func actionAction(changed bool) string {
	if changed {
		return "updated"
	}
	return "unchanged"
}
