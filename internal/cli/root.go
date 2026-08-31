// Package cli exposes the QUANTUM_LOG Cobra command tree.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/ingest/jsonl"
	"github.com/janpereira-dev/quantum_log/internal/pricing"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type Version struct {
	Version   string
	Commit    string
	BuildDate string
}

type currentProjectOutput struct {
	ProjectSlug  string `json:"project_slug,omitempty"`
	LocationID   string `json:"project_location_id,omitempty"`
	LocationPath string `json:"location_path,omitempty"`
	Method       string `json:"method"`
	Confidence   string `json:"confidence"`
	Evidence     string `json:"evidence"`
}

func (v Version) String() string {
	return fmt.Sprintf("qlog %s (commit %s, built %s)", v.Version, v.Commit, v.BuildDate)
}

func New(version Version) *cobra.Command {
	var home string
	root := &cobra.Command{
		Use:           "qlog",
		Short:         "Local-first observability and FinOps for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		RunE: func(command *cobra.Command, _ []string) error {
			if isTerminal(command.OutOrStdout()) {
				return runTUI(command.Context(), command.OutOrStdout(), home)
			}
			return command.Help()
		},
	}
	root.PersistentFlags().StringVar(&home, "home", "", "override the local QUANTUM_LOG data directory")
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newInitCommand(&home), newConfigCommand(&home), newDoctorCommand(&home), newVerifyCommand(&home), newMaintenanceCommand(&home), newProjectCommand(&home), newIngestCommand(&home), newUsageCommand(&home), newLogCommand(&home), newReportCommand(&home), newLegacySummaryCommand(&home), newAllocationCommand(&home), newPricingCommand(&home), newTaskCommand(&home), newSessionCommand(&home), newExportCommand(&home), newTUICommand(&home), newAdapterCommand(&home), newSetupCommand(&home), newCollectorCommand(&home), newHookCommand(&home), newUninstallCommand(&home), newRunCommand(&home), newMCPCommand(&home, version), newUnattributedCommand(&home), newBudgetCommand(&home), newAnchorCommand(&home), newAcceptanceCommand(&home, version))
	return root
}

func newInitCommand(home *string) *cobra.Command {
	return &cobra.Command{Use: "init", Short: "Initialize local configuration and ledger", RunE: func(command *cobra.Command, _ []string) (result error) {
		service, err := app.Initialize(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, service.Close()) }()
		_, result = fmt.Fprintf(command.Root().OutOrStdout(), "initialized QUANTUM_LOG at %s\n", service.Paths.Home)
		return result
	}}
}

func newConfigCommand(home *string) *cobra.Command {
	configCommand := &cobra.Command{Use: "config", Short: "Manage local privacy configuration"}
	configCommand.AddCommand(&cobra.Command{Use: "set prompt-capture <off|hash|full>", Short: "Set prompt capture policy", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if args[0] != "prompt-capture" {
			return fmt.Errorf("unsupported configuration key %q", args[0])
		}
		paths, err := config.Resolve(*home)
		if err != nil {
			return err
		}
		if err := config.SetPromptCaptureMode(paths, args[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "prompt-capture: %s\n", args[1])
		return err
	}})
	return configCommand
}

func newDoctorCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Check local ledger health without modifying it", RunE: func(command *cobra.Command, _ []string) (result error) {
		service, err := app.OpenReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, service.Close()) }()
		if err := service.Store.Doctor(command.Context()); err != nil {
			return err
		}
		if jsonOutput {
			output := map[string]string{"status": "ok", "database": service.Paths.Database}
			if warnings := service.Store.Warnings(); len(warnings) != 0 {
				output["warning"] = strings.Join(warnings, "; ")
			}
			return writeJSON(command.Root().OutOrStdout(), output)
		}
		for _, warning := range service.Store.Warnings() {
			_, _ = fmt.Fprintln(command.ErrOrStderr(), warning)
		}
		_, result = fmt.Fprintln(command.Root().OutOrStdout(), "doctor: ok")
		return result
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newVerifyCommand(home *string) *cobra.Command {
	var sessionID string
	command := &cobra.Command{Use: "verify", Short: "Verify append-only ledger hash chains", RunE: func(command *cobra.Command, _ []string) (result error) {
		service, err := app.OpenReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, service.Close()) }()
		if err := service.Store.VerifyLedger(command.Context(), sessionID); err != nil {
			return err
		}
		for _, warning := range service.Store.Warnings() {
			_, _ = fmt.Fprintln(command.ErrOrStderr(), warning)
		}
		_, result = fmt.Fprintln(command.Root().OutOrStdout(), "ledger: verified")
		return result
	}, Args: cobra.NoArgs}
	command.Flags().StringVar(&sessionID, "session", "", "verify one session")
	return command
}

func newMaintenanceCommand(home *string) *cobra.Command {
	maintenance := &cobra.Command{Use: "maintenance", Short: "Manage controlled local ledger maintenance"}
	maintenance.AddCommand(
		&cobra.Command{Use: "status", Short: "Show maintenance availability", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(command.OutOrStdout(), "maintenance status: checkpoint available; recover and rebuild-anchor blocked pending anchor task")
			return err
		}},
		&cobra.Command{Use: "checkpoint", Short: "Validate and checkpoint a quiescent ledger", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
			if err := app.Checkpoint(command.Context(), *home); err != nil {
				return err
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), "maintenance checkpoint: WAL cleared")
			return err
		}},
		&cobra.Command{Use: "recover", Short: "Recover a damaged ledger", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
			return errors.New("maintenance recover is not implemented; blocked pending anchor task")
		}},
		&cobra.Command{Use: "rebuild-anchor", Short: "Rebuild the ledger anchor", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
			return errors.New("maintenance rebuild-anchor is not implemented; blocked pending anchor task")
		}},
	)
	return maintenance
}

func newProjectCommand(home *string) *cobra.Command {
	project := &cobra.Command{Use: "project", Short: "Manage logical projects and physical locations"}
	project.AddCommand(newProjectRegisterCommand(home), newProjectCurrentCommand(home, "current"), newProjectCurrentCommand(home, "detect"), newProjectListCommand(home), newProjectShowCommand(home), newProjectTagCommand(home))
	return project
}

func newProjectTagCommand(home *string) *cobra.Command {
	var projectSlug string
	command := &cobra.Command{Use: "tag <key=value>", Short: "Add a normalized project tag", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("tag must use key=value")
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, _, found, err := service.Store.ProjectBySlug(command.Context(), projectSlug)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", projectSlug)
		}
		if err := service.Store.AddProjectTag(command.Context(), project.ID, parts[0], parts[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "tag: added")
		return err
	}}
	command.Flags().StringVar(&projectSlug, "project", "", "project slug")
	_ = command.MarkFlagRequired("project")
	command.AddCommand(newProjectTagListCommand(home))
	return command
}

func newProjectTagListCommand(home *string) *cobra.Command {
	var projectSlug string
	var jsonOutput bool
	command := &cobra.Command{Use: "list", Short: "List normalized project tags", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, _, found, err := service.Store.ProjectBySlug(command.Context(), projectSlug)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", projectSlug)
		}
		tags, err := service.Store.ProjectTags(command.Context(), project.ID)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), tags)
		}
		for _, tag := range tags {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s=%s\n", tag.Key, tag.Value); err != nil {
				return err
			}
		}
		return nil
	}}
	command.Flags().StringVar(&projectSlug, "project", "", "project slug")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	_ = command.MarkFlagRequired("project")
	return command
}

func newProjectListCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "list", Short: "List registered projects", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		projects, err := service.Store.ListProjects(command.Context())
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), projects)
		}
		for _, project := range projects {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %d locations | %d tags\n", project.Slug, project.Name, project.LocationCount, project.TagCount); err != nil {
				return err
			}
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newProjectShowCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "show <slug>", Short: "Show a registered project", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, location, found, err := service.Store.ProjectBySlug(command.Context(), args[0])
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", args[0])
		}
		tags, err := service.Store.ProjectTags(command.Context(), project.ID)
		if err != nil {
			return err
		}
		output := struct {
			ID       string              `json:"id"`
			Slug     string              `json:"slug"`
			Name     string              `json:"name"`
			Location string              `json:"location"`
			Tags     []sqlite.ProjectTag `json:"tags"`
		}{project.ID, project.Slug, project.Name, location.AbsolutePath, tags}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), output)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "%s\nname: %s\nlocation: %s\ntags: %d\n", output.Slug, output.Name, output.Location, len(output.Tags))
		return err
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newProjectRegisterCommand(home *string) *cobra.Command {
	var path, name, slug string
	command := &cobra.Command{Use: "register", Short: "Register a project location", RunE: func(command *cobra.Command, _ []string) error {
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if slug == "" {
			slug = slugify(name)
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		registered, location, err := service.Store.RegisterProject(command.Context(), name, slug, path)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "registered %s at %s\n", registered.Slug, location.AbsolutePath)
		return err
	}}
	command.Flags().StringVar(&path, "path", ".", "project location")
	command.Flags().StringVar(&name, "name", "", "human-readable project name")
	command.Flags().StringVar(&slug, "slug", "", "stable project slug")
	return command
}

func newProjectCurrentCommand(home *string, use string) *cobra.Command {
	var explicitProject string
	var jsonOutput bool
	command := &cobra.Command{Use: use, Short: "Resolve the active project", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		resolved, err := service.ResolveProject(command.Context(), explicitProject, "", "")
		if err != nil {
			return err
		}
		resolution := resolved.Resolution
		output := currentProjectOutput{ProjectSlug: resolution.ProjectSlug, Method: string(resolution.Method), Confidence: string(resolution.Confidence), Evidence: resolution.Evidence}
		if resolved.LocationID != "" {
			output.LocationID = resolved.LocationID
			output.LocationPath = resolved.LocationPath
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), output)
		}
		if resolution.ProjectSlug == "" {
			_, err = fmt.Fprintf(command.Root().OutOrStdout(), "project: unattributed\nmethod: %s\nconfidence: %s\n", resolution.Method, resolution.Confidence)
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "project: %s\nlocation: %s\nmethod: %s\nconfidence: %s\n", resolution.ProjectSlug, output.LocationPath, resolution.Method, resolution.Confidence)
		return err
	}}
	command.Flags().StringVar(&explicitProject, "project", "", "explicit project slug")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newIngestCommand(home *string) *cobra.Command {
	ingest := &cobra.Command{Use: "ingest", Short: "Import normalized raw events"}
	ingest.AddCommand(&cobra.Command{Use: "file <path>", Short: "Import NDJSON from a file", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open NDJSON file: %w", err)
		}
		defer func() { _ = file.Close() }()
		return importReader(command, home, file)
	}})
	ingest.AddCommand(&cobra.Command{Use: "stdin", Short: "Import NDJSON from standard input", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		return importReader(command, home, command.InOrStdin())
	}})
	return ingest
}

func importReader(command *cobra.Command, home *string, reader io.Reader) error {
	service, err := app.Open(command.Context(), *home)
	if err != nil {
		return err
	}
	defer func() { _ = service.Close() }()
	count, err := jsonl.ImportWithPromptCapture(command.Context(), service.Store, reader, config.PromptCaptureMode(service.Paths))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(command.Root().OutOrStdout(), "imported %d event(s)\n", count)
	return err
}

func newUsageCommand(home *string) *cobra.Command {
	usage := &cobra.Command{Use: "usage", Short: "Show observed token usage"}
	for _, period := range []string{"today", "week", "month"} {
		usage.AddCommand(newUsagePeriodCommand(home, period))
	}
	usage.AddCommand(newUsageProjectCommand(home))
	return usage
}

func newLogCommand(home *string) *cobra.Command {
	log := &cobra.Command{Use: "log", Short: "Show canonical prompt interactions"}
	list := func(command *cobra.Command, from time.Time, limit int, jsonOutput bool) error {
		service, err := app.OpenSnapshotReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		interactions, err := service.Store.ListInteractions(command.Context(), from, limit)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), interactions)
		}
		for _, interaction := range interactions {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %s | %s\n", interaction.OccurredAt.Format(time.RFC3339), interaction.Source, interaction.SessionID, interaction.ID); err != nil {
				return err
			}
		}
		return nil
	}
	var jsonOutput bool
	log.AddCommand(&cobra.Command{Use: "today", Short: "List today's interactions", RunE: func(command *cobra.Command, _ []string) error {
		return list(command, time.Now().UTC().Truncate(24*time.Hour), 1000, jsonOutput)
	}})
	log.Commands()[0].Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	var limit int
	var listJSON bool
	listCommand := &cobra.Command{Use: "list", Short: "List interactions", RunE: func(command *cobra.Command, _ []string) error { return list(command, time.Time{}, limit, listJSON) }}
	listCommand.Flags().IntVar(&limit, "limit", 100, "maximum interactions")
	listCommand.Flags().BoolVar(&listJSON, "json", false, "output JSON")
	log.AddCommand(listCommand)
	var tailJSON bool
	tail := &cobra.Command{Use: "tail", Short: "Show latest interactions", RunE: func(command *cobra.Command, _ []string) error { return list(command, time.Time{}, 20, tailJSON) }}
	tail.Flags().BoolVar(&tailJSON, "json", false, "output JSON")
	log.AddCommand(tail)
	var showJSON bool
	show := &cobra.Command{Use: "show <id>", Short: "Show one interaction", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.OpenSnapshotReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		interaction, found, err := service.Store.Interaction(command.Context(), args[0])
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("interaction %q not found", args[0])
		}
		if showJSON {
			return writeJSON(command.Root().OutOrStdout(), interaction)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %s | %s\n", interaction.OccurredAt.Format(time.RFC3339), interaction.Source, interaction.SessionID, interaction.ID)
		return err
	}}
	show.Flags().BoolVar(&showJSON, "json", false, "output JSON")
	log.AddCommand(show)
	return log
}

func newUsagePeriodCommand(home *string, period string) *cobra.Command {
	var groupBy string
	var jsonOutput bool
	command := &cobra.Command{Use: period, Short: "Report usage for " + period, RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		now := time.Now().UTC()
		from := now.AddDate(0, 0, -1)
		if period == "week" {
			from = now.AddDate(0, 0, -7)
		}
		if period == "month" {
			from = now.AddDate(0, -1, 0)
		}
		report, err := service.Store.Usage(command.Context(), storeUsageQuery(from, now, groupBy))
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), report)
		}
		return writeUsageReport(command.Root().OutOrStdout(), report)
	}}
	command.Flags().StringVar(&groupBy, "group-by", "project,agent,provider,model,capture_quality", "comma-separated dimensions")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newUsageProjectCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "project <slug>", Short: "Report usage for one project", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		report, err := service.Store.Usage(command.Context(), sqlite.UsageQuery{ProjectSlug: args[0], GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), report)
		}
		return writeUsageReport(command.Root().OutOrStdout(), report)
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func writeUsageReport(writer io.Writer, report sqlite.UsageReport) error {
	for _, row := range report.Rows {
		if _, err := fmt.Fprintf(writer, "%s | %s | %s/%s | %s | %d tokens\n", row.ProjectSlug, row.AgentName, row.Provider, row.Model, row.CaptureQuality, row.TotalTokens); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer, "TOTAL | %d tokens\n", report.TotalTokens)
	return err
}

func storeUsageQuery(from, to time.Time, groupBy string) sqlite.UsageQuery {
	return sqlite.UsageQuery{From: from, To: to, GroupBy: strings.Split(groupBy, ",")}
}

func newReportCommand(home *string) *cobra.Command {
	var from, to, groupBy string
	var jsonOutput, csvOutput bool
	report := &cobra.Command{Use: "report", Short: "Summarize observed usage and allocated cost", RunE: func(command *cobra.Command, _ []string) error {
		parsedFrom, err := parseDate(from)
		if err != nil {
			return err
		}
		parsedTo, err := parseDate(to)
		if err != nil {
			return err
		}
		return runCapabilityReport(command, home, sqlite.CapabilityQuery{From: parsedFrom, To: parsedTo}, jsonOutput, csvOutput)
	}}
	report.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	report.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	report.Flags().StringVar(&groupBy, "group-by", "project,provider,model", "comma-separated dimensions")
	report.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	report.Flags().BoolVar(&csvOutput, "csv", false, "output CSV metric coverage")
	report.AddCommand(newCapabilityReportCommand("today", "Report capability-aware evidence from last 24 hours", home, func(query *sqlite.CapabilityQuery, _ []string) {
		if query.From.IsZero() {
			now := time.Now().UTC()
			query.From = now.Add(-24 * time.Hour)
			query.To = now
		}
	}), newCapabilityReportCommand("project <project>", "Report capability-aware evidence for one project", home, func(query *sqlite.CapabilityQuery, args []string) {
		query.ProjectSlug = args[0]
	}), newCapabilityReportCommand("agent <agent>", "Report capability-aware evidence for one agent", home, func(query *sqlite.CapabilityQuery, args []string) {
		query.AgentName = args[0]
	}), newCapabilityReportCommand("session <session>", "Report capability-aware evidence for one session", home, func(query *sqlite.CapabilityQuery, args []string) {
		query.SessionID = args[0]
	}))

	var summaryFrom, summaryTo, summaryGroupBy string
	var summaryJSON bool
	summary := &cobra.Command{Use: "summary", Short: "Summarize observed usage and allocated cost", RunE: func(command *cobra.Command, _ []string) error {
		return runReportSummary(command, home, summaryFrom, summaryTo, summaryGroupBy, summaryJSON)
	}}
	summary.Flags().StringVar(&summaryFrom, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	summary.Flags().StringVar(&summaryTo, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	summary.Flags().StringVar(&summaryGroupBy, "group-by", "project,provider,model", "comma-separated dimensions")
	summary.Flags().BoolVar(&summaryJSON, "json", false, "output JSON")
	report.AddCommand(summary)
	var usageFrom, usageTo, usageGroupBy string
	var usageJSON bool
	usage := &cobra.Command{Use: "usage", Short: "Summarize quality-separated usage", RunE: func(command *cobra.Command, _ []string) error {
		return runReportSummary(command, home, usageFrom, usageTo, usageGroupBy, usageJSON)
	}}
	usage.Flags().StringVar(&usageFrom, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	usage.Flags().StringVar(&usageTo, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	usage.Flags().StringVar(&usageGroupBy, "group-by", "project,agent,provider,model,capture_quality", "comma-separated dimensions")
	usage.Flags().BoolVar(&usageJSON, "json", false, "output JSON")
	report.AddCommand(usage)
	return report
}

func newLegacySummaryCommand(home *string) *cobra.Command {
	var from, to, groupBy string
	var jsonOutput bool
	command := &cobra.Command{Use: "summary", Short: "Summarize observed usage and allocated cost", RunE: func(command *cobra.Command, _ []string) error {
		return runReportSummary(command, home, from, to, groupBy, jsonOutput)
	}}
	command.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	command.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	command.Flags().StringVar(&groupBy, "group-by", "project,provider,model", "comma-separated dimensions")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newCapabilityReportCommand(use, short string, home *string, scope func(*sqlite.CapabilityQuery, []string)) *cobra.Command {
	var from, to string
	var jsonOutput, csvOutput bool
	command := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		parsedFrom, err := parseDate(from)
		if err != nil {
			return err
		}
		parsedTo, err := parseDate(to)
		if err != nil {
			return err
		}
		query := sqlite.CapabilityQuery{From: parsedFrom, To: parsedTo}
		scope(&query, args)
		return runCapabilityReport(command, home, query, jsonOutput, csvOutput)
	}}
	if strings.HasPrefix(use, "today") {
		command.Args = cobra.NoArgs
	}
	command.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	command.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	command.Flags().BoolVar(&csvOutput, "csv", false, "output CSV metric coverage")
	return command
}

func runCapabilityReport(command *cobra.Command, home *string, query sqlite.CapabilityQuery, jsonOutput, csvOutput bool) error {
	if jsonOutput && csvOutput {
		return errors.New("--json and --csv cannot be used together")
	}
	service, err := app.Open(command.Context(), *home)
	if err != nil {
		return err
	}
	defer func() { _ = service.Close() }()
	report, err := service.Store.CapabilityReport(command.Context(), query)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(command.Root().OutOrStdout(), report)
	}
	if csvOutput {
		return writeCapabilityCSV(command.Root().OutOrStdout(), report)
	}
	return writeCapabilityReport(command.Root().OutOrStdout(), report)
}

func writeCapabilityReport(writer io.Writer, report sqlite.CapabilityReport) error {
	if _, err := fmt.Fprintf(writer, "INTERACTIONS %d | PROMPTS %d | MODEL CALLS %d | TOKENS %d | LIFECYCLE %d | TOOL %d | MCP %d | ERRORS %d | UNATTRIBUTED %d/%d\n", report.Interactions, report.Prompts, report.ModelCalls, report.Tokens, report.LifecycleEvents, report.ToolCalls, report.MCPCalls, report.Errors, report.UnattributedModelCalls, report.UnattributedTokens); err != nil {
		return err
	}
	for _, source := range report.Sources {
		version := "—"
		if source.Version != nil {
			version = *source.Version
		}
		if _, err := fmt.Fprintf(writer, "SOURCE %-16s QUALITY %-16s VERSION %-12s CALLS %d\n", source.Source, source.Quality, version, source.ModelCalls); err != nil {
			return err
		}
	}
	for _, metric := range report.MetricCoverage {
		if _, err := fmt.Fprintf(writer, "METRIC %-20s %s | %d/%d emitted | zero=%d\n", metric.Name, displayMetric(metric), metric.ReportedCount, metric.ReportedCount+metric.MissingCount, metric.ReportedZeroCount); err != nil {
			return err
		}
	}
	return nil
}

func writeCapabilityCSV(writer io.Writer, report sqlite.CapabilityReport) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"interactions", "prompts", "metric", "state", "value", "reported_count", "missing_count", "reported_zero_count", "source", "raw_key", "confidence", "provenance_count"}); err != nil {
		return err
	}
	for _, metric := range report.MetricCoverage {
		provenance := metric.Provenance
		if len(provenance) == 0 {
			provenance = []sqlite.MetricProvenance{{Source: "—", RawKey: "—", Confidence: "—"}}
		}
		for _, item := range provenance {
			if err := csvWriter.Write([]string{strconv.FormatInt(report.Interactions, 10), strconv.FormatInt(report.Prompts, 10), metric.Name, metric.State, displayMetric(metric), strconv.FormatInt(metric.ReportedCount, 10), strconv.FormatInt(metric.MissingCount, 10), strconv.FormatInt(metric.ReportedZeroCount, 10), item.Source, item.RawKey, item.Confidence, strconv.FormatInt(item.Count, 10)}); err != nil {
				return err
			}
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func displayMetric(metric sqlite.MetricCoverage) string {
	if metric.Value != nil {
		return strconv.FormatInt(*metric.Value, 10)
	}
	if metric.State == "not_emitted" {
		return "—"
	}
	return "?"
}

func runReportSummary(command *cobra.Command, home *string, fromValue, toValue, groupBy string, jsonOutput bool) error {
	from, err := parseDate(fromValue)
	if err != nil {
		return err
	}
	to, err := parseDate(toValue)
	if err != nil {
		return err
	}
	service, err := app.Open(command.Context(), *home)
	if err != nil {
		return err
	}
	defer func() { _ = service.Close() }()
	report, err := service.Store.Usage(command.Context(), storeUsageQuery(from, to, groupBy))
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(command.Root().OutOrStdout(), report)
	}
	for _, row := range report.Rows {
		if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s/%s | %d tokens | $%d micros\n", row.ProjectSlug, row.Provider, row.Model, row.TotalTokens, row.AllocatedCostUSDMicros); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(command.Root().OutOrStdout(), "TOTAL | %d tokens | $%d micros\n", report.TotalTokens, report.AllocatedCostUSDMicros)
	return err
}

func newAllocationCommand(home *string) *cobra.Command {
	allocation := &cobra.Command{Use: "allocation", Short: "Manage model call cost allocations"}
	allocation.AddCommand(&cobra.Command{Use: "split <model-call-id> <project=basis-points>...", Short: "Split a model call cost", Args: cobra.MinimumNArgs(3), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		allocations := make([]sqlite.AllocationInput, 0, len(args)-1)
		for _, raw := range args[1:] {
			parts := strings.SplitN(raw, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("allocation must use project=basis-points")
			}
			var basis int64
			if _, err := fmt.Sscan(parts[1], &basis); err != nil {
				return fmt.Errorf("parse allocation: %w", err)
			}
			project, _, found, err := service.Store.ProjectBySlug(command.Context(), parts[0])
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("project %q not found", parts[0])
			}
			allocations = append(allocations, sqlite.AllocationInput{ProjectID: project.ID, BasisPoints: basis})
		}
		if err := service.Store.ReplaceAllocations(command.Context(), "model_call", args[0], allocations); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "allocation: updated")
		return err
	}})
	var showJSON bool
	show := &cobra.Command{Use: "show <model-call-id>", Short: "Show model call allocations", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		allocations, err := service.Store.ModelCallAllocations(command.Context(), args[0])
		if err != nil {
			return err
		}
		if showJSON {
			return writeJSON(command.Root().OutOrStdout(), allocations)
		}
		for _, item := range allocations {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %d bp | %s\n", item.ProjectSlug, item.BasisPoints, item.Method); err != nil {
				return err
			}
		}
		return nil
	}}
	show.Flags().BoolVar(&showJSON, "json", false, "output JSON")
	allocation.AddCommand(show)

	var repairProject string
	repair := &cobra.Command{Use: "repair <model-call-id>", Short: "Repair an allocation with one explicit project", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, _, found, err := service.Store.ProjectBySlug(command.Context(), repairProject)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", repairProject)
		}
		if err := service.Store.RepairModelCallAllocation(command.Context(), args[0], project.ID); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "allocation: repaired")
		return err
	}}
	repair.Flags().StringVar(&repairProject, "project", "", "project slug")
	_ = repair.MarkFlagRequired("project")
	allocation.AddCommand(repair)
	return allocation
}

func newPricingCommand(home *string) *cobra.Command {
	pricingCommand := &cobra.Command{Use: "pricing", Short: "Manage versioned pricing registries"}
	pricingCommand.AddCommand(&cobra.Command{Use: "validate <file>", Short: "Validate a JSON pricing rule", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		file, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		if _, err := pricing.Load(file); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "pricing: valid")
		return err
	}})
	pricingCommand.AddCommand(&cobra.Command{Use: "add <file>", Short: "Persist a JSON pricing rule", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		file, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		rule, err := pricing.Load(file)
		if err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		record, err := service.Store.AddPricingRule(command.Context(), rule)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), record.ID)
		return err
	}})
	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List persisted pricing rules", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		rules, err := service.Store.ListPricingRules(command.Context())
		if err != nil {
			return err
		}
		if listJSON {
			return writeJSON(command.Root().OutOrStdout(), rules)
		}
		for _, record := range rules {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s/%s | %s | %s\n", record.ID, record.Rule.Provider, record.Rule.ModelPattern, record.Rule.ValidFrom.Format(time.RFC3339), record.Rule.Version); err != nil {
				return err
			}
		}
		return nil
	}}
	list.Flags().BoolVar(&listJSON, "json", false, "output JSON")
	pricingCommand.AddCommand(list)
	var showJSON bool
	show := &cobra.Command{Use: "show <provider/model>", Short: "Show persisted rules for one provider and model pattern", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("pricing identity must use provider/model")
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		rules, err := service.Store.ListPricingRules(command.Context())
		if err != nil {
			return err
		}
		filtered := make([]sqlite.PricingRuleRecord, 0)
		for _, record := range rules {
			if record.Rule.Provider == parts[0] && record.Rule.ModelPattern == parts[1] {
				filtered = append(filtered, record)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("pricing rule %q not found", args[0])
		}
		if showJSON {
			return writeJSON(command.Root().OutOrStdout(), filtered)
		}
		for _, record := range filtered {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | valid from %s | %s\n", record.ID, record.Rule.ValidFrom.Format(time.RFC3339), record.Rule.Version); err != nil {
				return err
			}
		}
		return nil
	}}
	show.Flags().BoolVar(&showJSON, "json", false, "output JSON")
	pricingCommand.AddCommand(show)
	var from, to string
	recalculate := &cobra.Command{Use: "recalculate", Short: "Recalculate model call costs using persisted rules", RunE: func(command *cobra.Command, _ []string) error {
		fromTime, err := parseDate(from)
		if err != nil {
			return err
		}
		toTime, err := parseDate(to)
		if err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		count, err := service.Store.RecalculateCosts(command.Context(), sqlite.PricingRecalculateQuery{From: fromTime, To: toTime})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "recalculated %d model call(s)\n", count)
		return err
	}}
	recalculate.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	recalculate.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	pricingCommand.AddCommand(recalculate)
	return pricingCommand
}

func newTaskCommand(home *string) *cobra.Command {
	var projectSlug, taskType, title string
	start := &cobra.Command{Use: "start", Short: "Start a project task", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, _, found, err := service.Store.ProjectBySlug(command.Context(), projectSlug)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", projectSlug)
		}
		id, err := service.Store.StartTask(command.Context(), sqlite.TaskInput{ProjectID: project.ID, Title: title, TaskType: taskType})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), id)
		return err
	}}
	start.Flags().StringVar(&projectSlug, "project", "", "project slug")
	start.Flags().StringVar(&taskType, "type", "other", "task type")
	start.Flags().StringVar(&title, "title", "", "task title")
	_ = start.MarkFlagRequired("project")
	_ = start.MarkFlagRequired("title")
	task := &cobra.Command{Use: "task", Short: "Manage tasks"}
	var result string
	finish := &cobra.Command{Use: "finish <task-id>", Short: "Finish an active task", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		if err := service.Store.FinishTask(command.Context(), args[0], result); err != nil {
			return err
		}
		summary, err := service.Store.TaskSummary(command.Context(), args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "task: finished | %d model call(s) | %d tokens | $%d micros\n", summary.ModelCallCount, summary.ObservedTokens, summary.AllocatedCostUSDMicros)
		return err
	}}
	finish.Flags().StringVar(&result, "result", "", "task result")
	var listProject string
	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List tasks", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		tasks, err := service.Store.ListTasks(command.Context(), listProject)
		if err != nil {
			return err
		}
		if listJSON {
			return writeJSON(command.Root().OutOrStdout(), tasks)
		}
		for _, item := range tasks {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %s | %s\n", item.ID, item.ProjectSlug, item.Status, item.Title); err != nil {
				return err
			}
		}
		return nil
	}}
	list.Flags().StringVar(&listProject, "project", "", "project slug")
	list.Flags().BoolVar(&listJSON, "json", false, "output JSON")
	var summaryJSON bool
	summary := &cobra.Command{Use: "summary <task-id>", Short: "Show recorded task usage summary", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		result, err := service.Store.TaskSummary(command.Context(), args[0])
		if err != nil {
			return err
		}
		if summaryJSON {
			return writeJSON(command.Root().OutOrStdout(), result)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %d model call(s) | %d tokens | $%d micros\n", result.ID, result.Status, result.ModelCallCount, result.ObservedTokens, result.AllocatedCostUSDMicros)
		return err
	}}
	summary.Flags().BoolVar(&summaryJSON, "json", false, "output JSON")
	task.AddCommand(start, finish, list, summary)
	return task
}

func newSessionCommand(home *string) *cobra.Command {
	var jsonOutput bool
	summary := &cobra.Command{Use: "summary <session-id>", Short: "Show recorded session evidence summary", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		snapshot, err := service.Store.SessionSnapshot(command.Context(), args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), snapshot)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s | %d raw event(s) | %d model call(s)\n", snapshot.SessionID, snapshot.AgentName, snapshot.RawEventCount, snapshot.ModelCallCount)
		return err
	}}
	summary.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	session := &cobra.Command{Use: "session", Short: "Inspect recorded sessions", Args: cobra.NoArgs}
	session.AddCommand(summary)
	return session
}

func newExportCommand(home *string) *cobra.Command {
	var format, from, to string
	var redactPaths bool
	command := &cobra.Command{Use: "export", Short: "Export normalized model calls as JSON or CSV", RunE: func(command *cobra.Command, _ []string) error {
		fromTime, err := parseDate(from)
		if err != nil {
			return err
		}
		toTime, err := parseDate(to)
		if err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		records, err := service.Store.ExportModelCalls(command.Context(), sqlite.PricingRecalculateQuery{From: fromTime, To: toTime})
		if err != nil {
			return err
		}
		if redactPaths {
			for index := range records {
				if records[index].ProjectLocationPath != "" {
					records[index].ProjectLocationPath = "[redacted]"
				}
			}
		}
		switch format {
		case "json":
			return writeJSON(command.Root().OutOrStdout(), records)
		case "csv":
			return writeCSV(command.Root().OutOrStdout(), records)
		default:
			return fmt.Errorf("unsupported export format %q", format)
		}
	}}
	command.Flags().StringVar(&format, "format", "json", "export format: json or csv")
	command.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	command.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	command.Flags().BoolVar(&redactPaths, "redact-paths", false, "replace project location paths in output")
	return command
}

func writeCSV(writer io.Writer, records []sqlite.ExportRecord) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"id", "occurred_at", "project_slug", "project_location_path", "provider", "model", "agent", "input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "total_tokens", "estimated_cost_usd_micros", "capture_quality", "allocation_project_slug", "allocation_basis_points", "allocation_method"}); err != nil {
		return err
	}
	for _, record := range records {
		allocations := record.Allocations
		if len(allocations) == 0 {
			allocations = []sqlite.Allocation{{}}
		}
		for _, allocation := range allocations {
			if err := csvWriter.Write([]string{record.ID, record.OccurredAt.Format(time.RFC3339Nano), record.ProjectSlug, record.ProjectLocationPath, record.Provider, record.Model, record.Agent, strconv.FormatInt(record.InputTokens, 10), strconv.FormatInt(record.OutputTokens, 10), strconv.FormatInt(record.ReasoningTokens, 10), strconv.FormatInt(record.CachedInputTokens, 10), strconv.FormatInt(record.CacheWriteTokens, 10), strconv.FormatInt(record.TotalTokens, 10), strconv.FormatInt(record.EstimatedCostUSDMicros, 10), record.CaptureQuality, allocation.ProjectSlug, strconv.FormatInt(allocation.BasisPoints, 10), allocation.Method}); err != nil {
				return err
			}
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date %q: use RFC3339 or YYYY-MM-DD", value)
	}
	return parsed, nil
}

func writeJSON(writer io.Writer, value any) error {
	return json.NewEncoder(writer).Encode(value)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = filepath.Base(value)
	return strings.Trim(value, "-")
}

func newAnchorCommand(home *string) *cobra.Command {
	anchor := &cobra.Command{Use: "anchor", Short: "Export and verify external ledger anchors for tamper and truncation detection"}
	check := &cobra.Command{Use: "check", Short: "Verify current ledger against previously exported anchors (use --file <path>)", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) (result error) {
		path, _ := command.Flags().GetString("file")
		if strings.TrimSpace(path) == "" {
			return errors.New("anchor check requires --file <path>")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read anchor file: %w", err)
		}
		var expected []sqlite.LedgerAnchor
		if err := json.Unmarshal(data, &expected); err != nil {
			return fmt.Errorf("parse anchor file: %w", err)
		}
		service, err := app.OpenReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, service.Close()) }()
		mismatches, err := service.Store.VerifyAnchors(command.Context(), expected)
		if err != nil {
			return err
		}
		if len(mismatches) == 0 {
			_, result = fmt.Fprintln(command.OutOrStdout(), "anchors: ok")
			return result
		}
		for _, m := range mismatches {
			kind := "mismatch"
			if m.Truncated {
				kind = "truncation"
			}
			_, _ = fmt.Fprintf(command.OutOrStdout(), "anchor %s: source=%s session=%s expected=%s actual=%s\n", kind, m.Source, m.SessionID, m.Expected, m.Actual)
		}
		return errors.New("anchor verification failed")
	}}
	check.Flags().String("file", "", "path to anchor JSON file")
	exportCmd := &cobra.Command{Use: "export", Short: "Export current ledger head anchors as JSON", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) (result error) {
		service, err := app.OpenReadOnly(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, service.Close()) }()
		anchors, err := service.Store.LedgerAnchors(command.Context())
		if err != nil {
			return err
		}
		return writeJSON(command.OutOrStdout(), anchors)
	}}
	anchor.AddCommand(exportCmd, check)
	return anchor
}
