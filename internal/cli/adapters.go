package cli

import (
	"fmt"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/spf13/cobra"
)

func newAdapterCommand() *cobra.Command {
	registry := adapters.Default()
	command := &cobra.Command{Use: "adapter", Short: "Inspect verified capture adapters"}
	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List adapters and their verified capabilities", RunE: func(command *cobra.Command, _ []string) error {
		descriptors := make([]adapters.Descriptor, 0)
		for _, adapter := range registry.List() {
			descriptors = append(descriptors, adapter.Descriptor())
		}
		if listJSON {
			return writeJSON(command.Root().OutOrStdout(), descriptors)
		}
		for _, descriptor := range descriptors {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %s\n", descriptor.ID, descriptor.Name, descriptor.Version); err != nil {
				return err
			}
		}
		return nil
	}}
	list.Flags().BoolVar(&listJSON, "json", false, "output JSON")

	var detectJSON bool
	detect := &cobra.Command{Use: "detect [adapter]", Short: "Detect installed adapters without changing files", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		items := registry.List()
		if len(args) == 1 {
			adapter, found := registry.Get(args[0])
			if !found {
				return fmt.Errorf("adapter %q not found", args[0])
			}
			items = []adapters.Adapter{adapter}
		}
		result := make(map[string]adapters.Detection, len(items))
		for _, adapter := range items {
			detection, err := adapter.Detect(command.Context())
			if err != nil {
				return err
			}
			result[adapter.Descriptor().ID] = detection
		}
		if detectJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		for _, adapter := range items {
			detection := result[adapter.Descriptor().ID]
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | available=%t | %s\n", adapter.Descriptor().ID, detection.Available, detection.Evidence); err != nil {
				return err
			}
		}
		return nil
	}}
	detect.Flags().BoolVar(&detectJSON, "json", false, "output JSON")

	var dryRun, installJSON bool
	install := &cobra.Command{Use: "install <adapter>", Short: "Install an adapter when it has a verified integration", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		adapter, found := registry.Get(args[0])
		if !found {
			return fmt.Errorf("adapter %q not found", args[0])
		}
		result, err := adapter.Install(command.Context(), adapters.InstallOptions{DryRun: dryRun})
		if err != nil {
			return err
		}
		if installJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		for _, action := range result.Actions {
			if _, err := fmt.Fprintln(command.Root().OutOrStdout(), action); err != nil {
				return err
			}
		}
		return nil
	}}
	install.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without writing files")
	install.Flags().BoolVar(&installJSON, "json", false, "output JSON")
	command.AddCommand(list, detect, install)
	return command
}
