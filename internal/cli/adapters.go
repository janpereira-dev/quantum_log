package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type adapterVerifyStage struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type adapterVerifyResult struct {
	AdapterID string               `json:"adapter_id"`
	Ready     bool                 `json:"ready"`
	Stages    []adapterVerifyStage `json:"stages"`
	Message   string               `json:"message"`
}

func newAdapterCommand(home *string) *cobra.Command {
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

	var statusJSON bool
	status := &cobra.Command{Use: "status [adapter]", Short: "Show adapter setup status", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		items := registry.List()
		if len(args) == 1 {
			adapter, found := registry.Get(args[0])
			if !found {
				return fmt.Errorf("adapter %q not found", args[0])
			}
			items = []adapters.Adapter{adapter}
		}
		statuses := make([]adapters.SetupStatus, 0, len(items))
		for _, adapter := range items {
			status, err := adapter.Status(command.Context())
			if err != nil {
				return err
			}
			statuses = append(statuses, status)
		}
		if statusJSON {
			if len(args) == 1 {
				return writeJSON(command.Root().OutOrStdout(), statuses[0])
			}
			return writeJSON(command.Root().OutOrStdout(), statuses)
		}
		for _, status := range statuses {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | installed=%t | capture=%s | %s\n", status.AdapterID, status.State, status.Installed, status.CaptureQuality, status.Evidence); err != nil {
				return err
			}
		}
		return nil
	}}
	status.Flags().BoolVar(&statusJSON, "json", false, "output JSON")

	var testJSON bool
	test := &cobra.Command{Use: "test <adapter>", Short: "Test one adapter capture readiness", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		adapter, found := registry.Get(args[0])
		if !found {
			return fmt.Errorf("adapter %q not found", args[0])
		}
		result, err := adapter.Test(command.Context())
		if err != nil {
			return err
		}
		if testJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "%s | passed=%t | capture=%s | %s\n", result.AdapterID, result.Passed, result.CaptureQuality, result.Message)
		return err
	}}
	test.Flags().BoolVar(&testJSON, "json", false, "output JSON")

	var verifyJSON bool
	var verifyProject string
	var verifySince string
	verify := &cobra.Command{Use: "verify <adapter>", Short: "Verify adapter capture readiness and evidence", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		adapter, found := registry.Get(args[0])
		if !found {
			return fmt.Errorf("unknown adapter %q", args[0])
		}
		result := verifyAdapter(command.Context(), *home, adapter, verifyProject, verifySince)
		if verifyJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		for _, stage := range result.Stages {
			state := "FAIL"
			if stage.Passed {
				state = "PASS"
			}
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s %s: %s\n", state, stage.Name, stage.Message); err != nil {
				return err
			}
		}
		if !result.Ready {
			return nil
		}
		_, err := fmt.Fprintln(command.Root().OutOrStdout(), result.Message)
		return err
	}}
	verify.Flags().StringVar(&verifyProject, "project", "", "project slug")
	verify.Flags().StringVar(&verifySince, "since", "1h", "lookback duration for local capture evidence")
	verify.Flags().BoolVar(&verifyJSON, "json", false, "output JSON")

	var uninstallDryRun, uninstallJSON bool
	uninstall := &cobra.Command{Use: "uninstall <adapter>", Short: "Uninstall qlog-owned adapter setup", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		adapter, found := registry.Get(args[0])
		if !found {
			return fmt.Errorf("adapter %q not found", args[0])
		}
		result, err := adapter.Uninstall(command.Context(), adapters.InstallOptions{DryRun: uninstallDryRun})
		if err != nil {
			return err
		}
		if uninstallJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		for _, action := range result.Actions {
			if _, err := fmt.Fprintln(command.Root().OutOrStdout(), action); err != nil {
				return err
			}
		}
		return nil
	}}
	uninstall.Flags().BoolVar(&uninstallDryRun, "dry-run", false, "show changes without writing files")
	uninstall.Flags().BoolVar(&uninstallJSON, "json", false, "output JSON")

	command.AddCommand(list, detect, install, status, test, verify, uninstall)
	return command
}

func verifyAdapter(ctx context.Context, home string, adapter adapters.Adapter, projectSlug, since string) adapterVerifyResult {
	status, err := adapter.Status(ctx)
	stages := []adapterVerifyStage{}
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "status", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "adapter status failed"}
	}
	stages = append(stages, adapterVerifyStage{Name: "settings", Passed: status.Installed, Message: string(status.State)})
	if adapter.Descriptor().ID != "copilot-vscode" {
		ready := status.Installed
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Ready: ready, Stages: stages, Message: "generic adapter verification complete"}
	}
	duration, err := time.ParseDuration(since)
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "since", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: "copilot-vscode", Stages: stages, Message: "invalid verification window"}
	}
	service, err := app.OpenReadOnly(ctx, home)
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "database", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: "copilot-vscode", Stages: stages, Message: "database unavailable"}
	}
	defer func() { _ = service.Close() }()
	now := time.Now().UTC()
	report, err := service.Store.Usage(ctx, sqlite.UsageQuery{From: now.Add(-duration), To: now, ProjectSlug: projectSlug, GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "usage", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: "copilot-vscode", Stages: stages, Message: "usage query failed"}
	}
	foundCopilot := false
	for _, row := range report.Rows {
		if row.CaptureQuality == string(adapters.CaptureOTELReported) && strings.Contains(strings.ToLower(row.AgentName), "copilot") && row.TotalTokens > 0 {
			foundCopilot = true
			break
		}
	}
	stages = append(stages, adapterVerifyStage{Name: "copilot_model_call", Passed: foundCopilot, Message: "requires recent Copilot-originated otel_reported model call with tokens in local storage"})
	ready := status.Installed && foundCopilot
	message := "Copilot capture is not verified yet"
	if ready {
		message = "Copilot capture verified"
	}
	return adapterVerifyResult{AdapterID: "copilot-vscode", Ready: ready, Stages: stages, Message: message}
}
