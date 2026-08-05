package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type adapterVerifyStage struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
	Message  string `json:"message"`
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
		items := registry.Stable()
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
		if !dryRun {
			detection, err := adapter.Detect(command.Context())
			if err != nil {
				return err
			}
			if !detection.Available {
				return fmt.Errorf("adapter %s is unavailable: %s", adapter.Descriptor().ID, detection.Evidence)
			}
		}
		options := adapters.InstallOptions{DryRun: dryRun}
		if adapter.Descriptor().ID == "copilot" && !dryRun {
			paths, err := config.Resolve(*home)
			if err != nil {
				return err
			}
			options, err = setupInstallOptions(paths.Home, "")
			if err != nil {
				return fmt.Errorf("install Copilot CLI adapter: %w", err)
			}
		}
		result, err := adapter.Install(command.Context(), options)
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
		paths, err := config.Resolve(*home)
		if err != nil {
			return err
		}
		items := registry.Stable()
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
			status = enrichAdapterStatus(command.Context(), paths.Home, status, localAdapterStatusAccess{})
			statuses = append(statuses, status)
		}
		if statusJSON {
			if len(args) == 1 {
				return writeJSON(command.Root().OutOrStdout(), statuses[0])
			}
			return writeJSON(command.Root().OutOrStdout(), statuses)
		}
		for _, status := range statuses {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | installed=%t | capture=%s | %s\n", status.AdapterID, status.InstallationState, status.Installed, status.CaptureQuality, status.Evidence); err != nil {
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
		if !adapter.Descriptor().Stable {
			return fmt.Errorf("adapter %q is not a stable capture adapter", args[0])
		}
		result := verifyAdapter(command.Context(), *home, adapter, verifyProject, verifySince)
		if verifyJSON {
			if err := writeJSON(command.Root().OutOrStdout(), result); err != nil {
				return err
			}
		} else {
			for _, stage := range result.Stages {
				state := "FAIL"
				if stage.Passed {
					state = "PASS"
				}
				if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s %s: %s\n", state, stage.Name, stage.Message); err != nil {
					return err
				}
			}
		}
		if !result.Ready {
			return adapterVerificationError{AdapterID: result.AdapterID}
		}
		if !verifyJSON {
			_, err := fmt.Fprintln(command.Root().OutOrStdout(), result.Message)
			return err
		}
		return nil
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

type adapterStatusAccess interface {
	CollectorReachable(context.Context) bool
	HasRecentEvidence(context.Context, string, adapters.SetupStatus) (bool, error)
}

type localAdapterStatusAccess struct{}

func (localAdapterStatusAccess) CollectorReachable(ctx context.Context) bool {
	reachable, _ := verifyCollectorReachability(ctx)
	return reachable
}

func (localAdapterStatusAccess) HasRecentEvidence(ctx context.Context, home string, status adapters.SetupStatus) (bool, error) {
	contract := evidenceContract(status.AdapterID)
	if contract.Source == "" || contract.Quality == "" {
		return false, nil
	}
	service, err := app.OpenReadOnly(ctx, home)
	if err != nil {
		return false, err
	}
	defer func() { _ = service.Close() }()
	now := time.Now().UTC()
	return service.Store.HasRecentAdapterEvidence(ctx, sqlite.AdapterEvidenceQuery{
		AdapterID:                     status.AdapterID,
		AllowedAgentNames:             contract.AllowedAgentNames,
		Source:                        contract.Source,
		From:                          now.Add(-time.Hour),
		To:                            now,
		RequiredQuality:               string(contract.Quality),
		RequiredProvider:              contract.RequiredProvider,
		RequireCodexResponseCompleted: contract.RequireCodexResponseCompleted,
	})
}

func enrichAdapterStatus(ctx context.Context, home string, status adapters.SetupStatus, access adapterStatusAccess) adapters.SetupStatus {
	status.CollectorReachable = access.CollectorReachable(ctx)
	status.RecentEvidence, _ = access.HasRecentEvidence(ctx, home, status)
	return status
}

type adapterVerificationError struct{ AdapterID string }

func (e adapterVerificationError) Error() string {
	return fmt.Sprintf("adapter %s is not verified", e.AdapterID)
}

type adapterEvidenceContract struct {
	Source                        string
	Quality                       adapters.CaptureQuality
	AllowedAgentNames             []string
	RequiredProvider              string
	RequireCodexResponseCompleted bool
	SourceEvidence                bool
	SourceEvidenceMessage         string
}

func evidenceContract(adapterID string) adapterEvidenceContract {
	switch adapterID {
	case "claude-code":
		return adapterEvidenceContract{Source: "claude-code-hook", Quality: adapters.CaptureLifecycleOnly, SourceEvidence: true, SourceEvidenceMessage: "Claude Code hooks emit lifecycle evidence only"}
	case "opencode":
		return adapterEvidenceContract{Source: "opencode-plugin", Quality: adapters.CaptureLifecycleOnly, SourceEvidenceMessage: "documented source-backed OpenCode usage evidence is required before verification"}
	case "codex":
		return adapterEvidenceContract{Source: "otlp-http", Quality: adapters.CaptureOTELReported, RequireCodexResponseCompleted: true, SourceEvidence: true, SourceEvidenceMessage: "Codex 0.145.0 documents OTLP response.completed logs with source-reported tokens"}
	case "copilot":
		return adapterEvidenceContract{Source: "copilot-cli-hook", Quality: adapters.CaptureLifecycleOnly, SourceEvidence: true, SourceEvidenceMessage: "GitHub Copilot CLI documents local lifecycle and tool hooks; qlog records only sanitized lifecycle metadata"}
	case "copilot-vscode":
		return adapterEvidenceContract{Source: "otlp-http", Quality: adapters.CaptureOTELReported, AllowedAgentNames: []string{"GitHub Copilot Chat"}, RequiredProvider: "github", SourceEvidence: true, SourceEvidenceMessage: "VS Code documents Copilot OTel trace/span identity and gen_ai usage fields"}
	default:
		return adapterEvidenceContract{SourceEvidenceMessage: "adapter is outside stable verification scope"}
	}
}

func requiredStagesPassed(stages []adapterVerifyStage) bool {
	for _, stage := range stages {
		if stage.Required && !stage.Passed {
			return false
		}
	}
	return true
}

func verifyAdapter(ctx context.Context, home string, adapter adapters.Adapter, projectSlug, since string) adapterVerifyResult {
	status, err := adapter.Status(ctx)
	stages := []adapterVerifyStage{}
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "status", Passed: false, Required: true, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "adapter status failed"}
	}
	contract := evidenceContract(adapter.Descriptor().ID)
	stages = append(stages,
		adapterVerifyStage{Name: "settings", Passed: status.Installed, Required: true, Message: string(status.InstallationState)},
		adapterVerifyStage{Name: "availability", Passed: status.Available, Required: true, Message: status.Evidence},
	)
	collectorOK, collectorMessage := verifyCollectorReachability(ctx)
	stages = append(stages, adapterVerifyStage{Name: "collector", Passed: collectorOK, Required: true, Message: collectorMessage})
	duration, err := time.ParseDuration(since)
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "since", Passed: false, Required: true, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "invalid verification window"}
	}
	service, err := app.OpenReadOnly(ctx, home)
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "database", Passed: false, Required: true, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "database unavailable"}
	}
	defer func() { _ = service.Close() }()
	now := time.Now().UTC()
	foundEvidence, err := service.Store.HasRecentAdapterEvidence(ctx, sqlite.AdapterEvidenceQuery{AdapterID: adapter.Descriptor().ID, AllowedAgentNames: contract.AllowedAgentNames, Source: contract.Source, From: now.Add(-duration), To: now, ProjectSlug: projectSlug, RequiredQuality: string(contract.Quality), RequiredProvider: contract.RequiredProvider, RequireCodexResponseCompleted: contract.RequireCodexResponseCompleted})
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "raw_evidence", Passed: false, Required: true, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "evidence query failed"}
	}
	evidenceMessage := fmt.Sprintf("requires recent %s evidence from %s with %s quality", adapter.Descriptor().ID, contract.Source, contract.Quality)
	if contract.Quality != adapters.CaptureLifecycleOnly {
		evidenceMessage += " and one linked normalized model call with source-reported tokens"
	}
	stages = append(stages,
		adapterVerifyStage{Name: "capture_quality", Passed: status.CaptureQuality == contract.Quality, Required: true, Message: fmt.Sprintf("expected %s, got %s", contract.Quality, status.CaptureQuality)},
		adapterVerifyStage{Name: "raw_evidence", Passed: foundEvidence, Required: true, Message: evidenceMessage},
		adapterVerifyStage{Name: "source_evidence", Passed: contract.SourceEvidence, Required: true, Message: contract.SourceEvidenceMessage},
	)
	ready := requiredStagesPassed(stages)
	message := fmt.Sprintf("%s capture is not verified yet", adapter.Descriptor().Name)
	if ready {
		message = fmt.Sprintf("%s capture verified", adapter.Descriptor().Name)
	}
	return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Ready: ready, Stages: stages, Message: message}
}

func verifyCollectorReachability(ctx context.Context) (bool, string) {
	endpoint := os.Getenv("QLOG_COLLECTOR_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:4318/v1/traces"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false, fmt.Sprintf("invalid collector URL %q", endpoint)
	}
	healthEndpoint := parsed.Scheme + "://" + parsed.Host + "/healthz"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthEndpoint, nil)
	if err != nil {
		return false, err.Error()
	}
	request.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, fmt.Sprintf("collector %s returned HTTP %d", healthEndpoint, response.StatusCode)
	}
	return true, "collector reachable at " + healthEndpoint
}
