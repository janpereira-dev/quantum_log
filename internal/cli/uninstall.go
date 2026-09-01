package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/config"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
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
var removeUninstallDataDirectory = os.RemoveAll
var prepareUninstallDataPurge = storepkg.PreparePurge

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
			manager := newUninstallCollectorManager()
			purgeHome, purgeListen := "", ""
			if purgeData {
				var preflightErr error
				purgeHome, purgeListen, preflightErr = resolveUninstallPurgeTarget(command, manager, *home)
				if preflightErr != nil {
					failures = append(failures, preflightErr)
				}
			}

			for _, adapter := range registry.List() {
				adapterResult, err := adapter.Uninstall(command.Context(), adapters.InstallOptions{DryRun: dryRun})
				if err != nil {
					failures = append(failures, fmt.Errorf("uninstall adapter %s: %w", adapter.Descriptor().ID, err))
					continue
				}
				result.Adapters[adapter.Descriptor().ID] = adapterResult
			}

			if dryRun {
				result.Collector = CollectorStatus{Message: "dry run: collector uninstall skipped"}
			} else {
				collector, err := manager.Uninstall()
				if err != nil {
					failures = append(failures, fmt.Errorf("uninstall collector: %w", err))
				} else {
					result.Collector = collector
				}
			}

			if purgeData && !dryRun {
				if len(failures) != 0 {
					failures = append(failures, errors.New("local data was retained because qlog-owned cleanup or purge preflight was incomplete"))
				} else if status, err := manager.Status(command.Context(), purgeListen); err != nil {
					failures = append(failures, fmt.Errorf("inspect collector after uninstall before purging local data: %w", err))
				} else if status.Reachable && status.ManagedHealth {
					failures = append(failures, errors.New("refusing to purge local data while a reachable foreground qlog collector is active; stop qlog collector serve and retry"))
				} else if err := purgeUninstallData(command, purgeHome); err != nil {
					failures = append(failures, err)
				} else {
					result.DataPurged = true
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

func resolveUninstallPurgeTarget(command *cobra.Command, manager collectorManager, home string) (string, string, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return "", "", fmt.Errorf("resolve local data directory: %w", err)
	}
	resolvedHome, resolvedListen := resolveManagedCollectorSettings(manager, paths.Home, defaultCollectorListen, command.Flags().Changed("home"), false)
	paths, err = config.Resolve(resolvedHome)
	if err != nil {
		return "", "", fmt.Errorf("resolve managed local data directory: %w", err)
	}
	if err := rejectUnsafeUninstallHome(paths.Home); err != nil {
		return "", "", err
	}
	return paths.Home, resolvedListen, nil
}

func purgeUninstallData(command *cobra.Command, home string) error {
	if err := rejectUnsafeUninstallHome(home); err != nil {
		return err
	}
	paths, err := config.Resolve(home)
	if err != nil {
		return fmt.Errorf("resolve local data directory: %w", err)
	}
	if _, err := os.Lstat(paths.Home); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect local data directory: %w", err)
	}
	guard, err := prepareUninstallDataPurge(command.Context(), paths.Database)
	if err != nil {
		return fmt.Errorf("refusing to purge data that is not an idle, valid qlog ledger: %w", err)
	}
	deletionPath, err := guard.DetachForPurge()
	if err != nil {
		if abortErr := guard.Abort(); abortErr != nil {
			return errors.Join(fmt.Errorf("detach qlog ledger for purge: %w", err), fmt.Errorf("restore access to retained local data: %w", abortErr))
		}
		return fmt.Errorf("detach qlog ledger for purge: %w", err)
	}
	if deletionPath != "" {
		if err := removeUninstallDataDirectory(deletionPath); err != nil {
			if abortErr := guard.Abort(); abortErr != nil {
				return errors.Join(fmt.Errorf("remove local data directory: %w", err), fmt.Errorf("restore access to retained local data: %w", abortErr))
			}
			return fmt.Errorf("remove local data directory: %w", err)
		}
	}
	if err := guard.Complete(); err != nil {
		return fmt.Errorf("complete local data purge: %w", err)
	}
	return nil
}

func rejectUnsafeUninstallHome(home string) error {
	clean := filepath.Clean(home)
	volumeRoot := filepath.VolumeName(clean) + string(filepath.Separator)
	if clean == string(filepath.Separator) || (filepath.VolumeName(clean) != "" && clean == volumeRoot) || filepath.Dir(clean) == clean {
		return fmt.Errorf("refusing to purge unsafe qlog home %q", home)
	}
	if userHome, err := os.UserHomeDir(); err == nil {
		if resolvedUserHome, resolveErr := filepath.Abs(userHome); resolveErr == nil && filepath.Clean(resolvedUserHome) == clean {
			return fmt.Errorf("refusing to purge user home %q", home)
		}
	}
	info, err := os.Lstat(clean)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect qlog home before purge: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to purge non-directory or symbolic-link qlog home %q", home)
	}
	return nil
}
