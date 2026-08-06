package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/spf13/cobra"
)

const defaultCollectorListen = "127.0.0.1:4318"

var newSetupCollectorManager = newCollectorManager

// BootstrapResult reports the consented collector and adapter setup actions.
type BootstrapResult struct {
	Consent              bool                     `json:"consent"`
	PlanOnly             bool                     `json:"plan_only,omitempty"`
	Collector            CollectorBootstrapStatus `json:"collector"`
	Adapters             []adapters.SetupPlan     `json:"adapters"`
	VerificationCommands []string                 `json:"verification_commands,omitempty"`
}

// CollectorBootstrapStatus distinguishes requested operations from their results.
type CollectorBootstrapStatus struct {
	Installed bool     `json:"installed"`
	Started   bool     `json:"started"`
	Healthy   bool     `json:"healthy"`
	Health    string   `json:"health,omitempty"`
	Actions   []string `json:"actions"`
}

type collectorFallbackInstaller interface {
	InstallFallback(home, listen string) (CollectorStatus, error)
}

func newSetupCommand(home *string) *cobra.Command {
	registry := adapters.Default()
	var all, yes, dryRun, jsonOutput bool
	var executable string
	command := &cobra.Command{Use: "setup [adapter]", Short: "Set up agent auto-capture integrations", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 && !all {
			paths, err := config.Resolve(*home)
			if err != nil {
				return err
			}
			result, err := bootstrapSupportedAdapters(command.Context(), paths.Home, executable, yes, dryRun, registry, newSetupCollectorManager())
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(command.Root().OutOrStdout(), result)
			}
			return writeBootstrapResult(command.Root().OutOrStdout(), result)
		}

		items := registry.Stable()
		if all {
			items = registry.List()
		}
		if len(args) == 1 {
			adapter, found := registry.Get(args[0])
			if !found {
				return fmt.Errorf("adapter %q not found", args[0])
			}
			items = []adapters.Adapter{adapter}
		} else if !all {
			var err error
			items, err = setupDefaultAdapters(command.Context(), items)
			if err != nil {
				return err
			}
		}

		paths, err := config.Resolve(*home)
		if err != nil {
			return err
		}
		resolvedHome := paths.Home

		items, err = availableSetupAdapters(command.Context(), items)
		if err != nil {
			return err
		}
		if len(args) == 1 && yes && !dryRun && len(items) == 0 {
			return fmt.Errorf("adapter %s is unavailable", args[0])
		}
		plans := make([]adapters.SetupPlan, 0, len(items))
		for _, adapter := range items {
			if adapter.Descriptor().ID == "generic-jsonl" {
				continue
			}
			var plan adapters.SetupPlan
			var err error
			if dryRun || !yes {
				plan, err = adapter.PlanInstall(command.Context(), adapters.SetupOptions{DryRun: true, Yes: yes, Home: resolvedHome})
				plan = markSetupPlanOnly(plan)
			} else {
				installOptions, installOptionsErr := setupInstallOptions(resolvedHome, executable)
				if installOptionsErr != nil {
					return installOptionsErr
				}
				result, installErr := adapter.Install(command.Context(), installOptions)
				if installErr != nil {
					return installErr
				}
				plan, err = adapter.PlanInstall(command.Context(), adapters.SetupOptions{Yes: yes, Home: resolvedHome})
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
			prefix := ""
			if plan.PlanOnly {
				prefix = "plan-only "
			}
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s%s | %s | capture=%s\n", prefix, plan.AdapterID, plan.State, plan.CaptureQuality); err != nil {
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
	command.Flags().StringVar(&executable, "executable", "", "absolute qlog executable used by installed hooks")
	_ = command.Flags().MarkHidden("executable")
	return command
}

func bootstrapSupportedAdapters(ctx context.Context, home, executable string, yes, dryRun bool, registry *adapters.Registry, manager collectorManager) (BootstrapResult, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return BootstrapResult{}, err
	}
	items := registry.Stable()
	items, err = availableSetupAdapters(ctx, items)
	if err != nil {
		return BootstrapResult{}, err
	}
	plans, err := planSetupAdapters(ctx, paths.Home, items, yes, dryRun)
	if err != nil {
		return BootstrapResult{}, err
	}
	result := BootstrapResult{Consent: yes, PlanOnly: !yes || dryRun, Adapters: plans}
	if !yes || dryRun {
		if dryRun {
			result.Collector.Actions = []string{"dry run: collector install and start skipped"}
		}
		return result, nil
	}
	installOptions, err := setupInstallOptions(paths.Home, executable)
	if err != nil {
		return BootstrapResult{}, err
	}
	service, err := app.Initialize(ctx, paths.Home)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("initialize ledger before collector setup: %w", err)
	}
	if err := service.Close(); err != nil {
		return BootstrapResult{}, fmt.Errorf("close initialized ledger: %w", err)
	}

	installed, err := manager.Install(paths.Home, defaultCollectorListen)
	if err != nil {
		if !recordCollectorExternalPolicy(&result.Collector, err) {
			return BootstrapResult{}, err
		}
		fallback, ok := manager.(collectorFallbackInstaller)
		if !ok {
			return BootstrapResult{}, fmt.Errorf("install Windows user fallback collector: unsupported collector manager")
		}
		fallbackStatus, fallbackErr := fallback.InstallFallback(paths.Home, defaultCollectorListen)
		if fallbackErr != nil {
			return BootstrapResult{}, fmt.Errorf("install Windows user fallback collector: %w", fallbackErr)
		}
		result.Collector.Installed = fallbackStatus.Installed
		result.Collector.Started = fallbackStatus.Running
		result.Collector.Actions = append(result.Collector.Actions, fallbackStatus.Message)
	} else {
		result.Collector.Installed = true
		result.Collector.Actions = append(result.Collector.Actions, installed.Message)
		started, startErr := manager.Start(paths.Home, defaultCollectorListen)
		if startErr != nil {
			if !recordCollectorExternalPolicy(&result.Collector, startErr) {
				return BootstrapResult{}, startErr
			}
		} else {
			result.Collector.Started = true
			result.Collector.Actions = append(result.Collector.Actions, started.Message)
		}
	}
	recordCollectorHealth(ctx, &result.Collector, manager)

	for index, adapter := range items {
		installResult, err := adapter.Install(ctx, installOptions)
		if err != nil {
			return BootstrapResult{}, err
		}
		result.Adapters[index].Changes = installResultChanges(adapter.Descriptor().ID, installResult)
		result.VerificationCommands = append(result.VerificationCommands, "qlog adapter verify "+adapter.Descriptor().ID)
	}
	return result, nil
}

func recordCollectorExternalPolicy(status *CollectorBootstrapStatus, err error) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	diagnosis := err.Error()
	lower := strings.ToLower(diagnosis)
	if !strings.Contains(lower, "task scheduler operation /create") || (!strings.Contains(lower, "access denied") && !strings.Contains(lower, "acceso denegado")) {
		return false
	}
	status.Actions = append(status.Actions, "collector activation blocked by external policy: "+diagnosis)
	return true
}

func recordCollectorHealth(ctx context.Context, status *CollectorBootstrapStatus, manager collectorManager) {
	health, err := pollCollectorReadiness(ctx, func(ctx context.Context) (CollectorStatus, error) {
		return manager.Status(ctx, defaultCollectorListen)
	})
	if err != nil {
		status.Actions = append(status.Actions, "collector health check failed: "+err.Error())
		return
	}
	status.Healthy = health.Reachable
	status.Health = health.Message
	status.Actions = append(status.Actions, fmt.Sprintf("collector health check: reachable=%t running=%t: %s", health.Reachable, health.Running, health.Message))
}

const collectorReadinessTimeout = 3 * time.Second

func pollCollectorReadiness(ctx context.Context, check func(context.Context) (CollectorStatus, error)) (CollectorStatus, error) {
	deadline := time.NewTimer(collectorReadinessTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var last CollectorStatus
	var lastErr error
	for {
		status, err := check(ctx)
		if err == nil {
			last = status
			if status.Reachable {
				return status, nil
			}
			if !status.Installed && !status.Running {
				if status.Message == "" {
					status.Message = "collector is not installed or running"
				}
				return status, nil
			}
			lastErr = nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-deadline.C:
			if lastErr != nil {
				return last, fmt.Errorf("collector did not become ready within %s: %w", collectorReadinessTimeout, lastErr)
			}
			if last.Message == "" {
				last.Message = "no collector health response"
			}
			return last, fmt.Errorf("collector did not become ready within %s: %s", collectorReadinessTimeout, last.Message)
		case <-ticker.C:
		}
	}
}

func setupInstallOptions(home, executable string) (adapters.InstallOptions, error) {
	executablePath, err := durableExecutablePath(executable)
	if err != nil {
		return adapters.InstallOptions{}, err
	}
	return adapters.InstallOptions{Home: home, ExecutablePath: executablePath}, nil
}

func durableExecutablePath(executable string) (string, error) {
	if executable == "" {
		path, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve qlog executable: %w", err)
		}
		executable = path
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("qlog executable path must be absolute: %q", executable)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve qlog executable path %q: %w", executable, err)
	}
	resolved = filepath.Clean(resolved)
	if err := validateCollectorExecutable(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func planSetupAdapters(ctx context.Context, home string, items []adapters.Adapter, yes, dryRun bool) ([]adapters.SetupPlan, error) {
	plans := make([]adapters.SetupPlan, 0, len(items))
	for _, adapter := range items {
		plan, err := adapter.PlanInstall(ctx, adapters.SetupOptions{DryRun: true, Yes: yes, Home: home})
		if err != nil {
			return nil, err
		}
		if !yes || dryRun {
			plan = markSetupPlanOnly(plan)
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func markSetupPlanOnly(plan adapters.SetupPlan) adapters.SetupPlan {
	plan.PlanOnly = true
	for index := range plan.Changes {
		plan.Changes[index].Action = "planned"
		plan.Changes[index].BackupPath = ""
		description := plan.Changes[index].Description
		for strings.HasPrefix(description, "dry run: ") {
			description = strings.TrimPrefix(description, "dry run: ")
		}
		plan.Changes[index].Description = "plan only: " + description
	}
	return plan
}

func writeBootstrapResult(writer interface{ Write([]byte) (int, error) }, result BootstrapResult) error {
	if _, err := fmt.Fprintf(writer, "consent=%t plan_only=%t collector installed=%t started=%t healthy=%t\n", result.Consent, result.PlanOnly, result.Collector.Installed, result.Collector.Started, result.Collector.Healthy); err != nil {
		return err
	}
	for _, action := range result.Collector.Actions {
		if _, err := fmt.Fprintf(writer, "  collector %s\n", action); err != nil {
			return err
		}
	}
	for _, plan := range result.Adapters {
		prefix := ""
		if plan.PlanOnly {
			prefix = "plan-only "
		}
		if _, err := fmt.Fprintf(writer, "%s%s | %s | capture=%s\n", prefix, plan.AdapterID, plan.State, plan.CaptureQuality); err != nil {
			return err
		}
		for _, change := range plan.Changes {
			if _, err := fmt.Fprintf(writer, "  %s %s\n", change.Action, change.Path); err != nil {
				return err
			}
		}
	}
	for _, command := range result.VerificationCommands {
		if _, err := fmt.Fprintf(writer, "after your first agent event, verify: %s\n", command); err != nil {
			return err
		}
	}
	return nil
}

func setupDefaultAdapters(ctx context.Context, items []adapters.Adapter) ([]adapters.Adapter, error) {
	result := make([]adapters.Adapter, 0, len(items))
	for _, item := range items {
		if !item.Descriptor().Stable {
			continue
		}
		status, err := item.Status(ctx)
		if err != nil {
			return nil, err
		}
		if status.Available || status.Installed {
			result = append(result, item)
		}
	}
	return result, nil
}

func availableSetupAdapters(ctx context.Context, items []adapters.Adapter) ([]adapters.Adapter, error) {
	available := make([]adapters.Adapter, 0, len(items))
	for _, adapter := range items {
		if adapter.Descriptor().ID == "generic-jsonl" {
			continue
		}
		detection, err := adapter.Detect(ctx)
		if err != nil {
			return nil, err
		}
		if detection.Available {
			available = append(available, adapter)
		}
	}
	return available, nil
}

func installResultChanges(adapterID string, result adapters.InstallResult) []adapters.SetupChange {
	if len(result.Changes) != 0 {
		return result.Changes
	}
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
