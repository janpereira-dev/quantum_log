package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/spf13/cobra"
)

// uninstallResult contains only qlog-owned configuration. Local ledger data is
// retained unless the operator explicitly asks to remove it.
type uninstallResult struct {
	Adapters   map[string]adapters.InstallResult `json:"adapters"`
	Collector  CollectorStatus                   `json:"collector"`
	DataPurged bool                              `json:"data_purged"`
}

var newUninstallCollectorManager = newCollectorManager

func newUninstallCommand(home *string) *cobra.Command {
	registry := adapters.Default()
	var dryRun, jsonOutput, purgeData bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove all qlog-owned collector and adapter setup",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result := uninstallResult{Adapters: make(map[string]adapters.InstallResult)}
			failures := make([]error, 0)
			for _, adapter := range registry.List() {
				adapterResult, err := adapter.Uninstall(command.Context(), adapters.InstallOptions{DryRun: dryRun})
				if err != nil {
					failures = append(failures, fmt.Errorf("uninstall adapter %s: %w", adapter.Descriptor().ID, err))
					continue
				}
				result.Adapters[adapter.Descriptor().ID] = adapterResult
			}

			collector, err := newUninstallCollectorManager().Uninstall()
			if err != nil {
				failures = append(failures, fmt.Errorf("uninstall collector: %w", err))
			} else {
				result.Collector = collector
			}

			if purgeData {
				if len(failures) != 0 {
					failures = append(failures, errors.New("local data was retained because qlog-owned cleanup was incomplete"))
				} else if !dryRun {
					paths, resolveErr := config.Resolve(*home)
					if resolveErr != nil {
						failures = append(failures, fmt.Errorf("resolve local data directory: %w", resolveErr))
					} else if removeErr := os.RemoveAll(paths.Home); removeErr != nil {
						failures = append(failures, fmt.Errorf("remove local data directory: %w", removeErr))
					} else {
						result.DataPurged = true
					}
				}
			}

			if jsonOutput {
				if err := writeJSON(command.Root().OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				for _, adapter := range registry.List() {
					if adapterResult, found := result.Adapters[adapter.Descriptor().ID]; found {
						for _, action := range adapterResult.Actions {
							if _, err := fmt.Fprintln(command.Root().OutOrStdout(), action); err != nil {
								return err
							}
						}
					}
				}
				if result.Collector.Message != "" {
					if _, err := fmt.Fprintln(command.Root().OutOrStdout(), result.Collector.Message); err != nil {
						return err
					}
				}
				if purgeData && result.DataPurged {
					if _, err := fmt.Fprintln(command.Root().OutOrStdout(), "removed local QUANTUM_LOG data"); err != nil {
						return err
					}
				}
			}
			return errors.Join(failures...)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show qlog-owned cleanup without writing files")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	command.Flags().BoolVar(&purgeData, "purge-data", false, "also delete the local QUANTUM_LOG ledger after successful cleanup")
	return command
}
