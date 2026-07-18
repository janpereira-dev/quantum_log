// Package cli exposes the QUANTUM_LOG Cobra command tree.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
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
	root.AddCommand(newInitCommand(&home), newDoctorCommand(&home), newVerifyCommand(&home), newProjectCommand(&home), newIngestCommand(&home), newUsageCommand(&home), newReportCommand(&home), newAllocationCommand(&home), newPricingCommand(&home), newTaskCommand(&home), newExportCommand(&home), newTUICommand(&home), newAdapterCommand(), newCollectorCommand(&home), newRunCommand(&home), newMCPCommand(&home, version), newUnattributedCommand(&home), newBudgetCommand(&home))
	return root
}

func newInitCommand(home *string) *cobra.Command {
	return &cobra.Command{Use: "init", Short: "Initialize local configuration and ledger", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Initialize(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "initialized QUANTUM_LOG at %s\n", service.Paths.Home)
		return err
	}}
}

func newDoctorCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Check local ledger health without modifying it", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		if err := service.Store.Doctor(command.Context()); err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), map[string]string{"status": "ok", "database": service.Paths.Database})
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "doctor: ok")
		return err
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newVerifyCommand(home *string) *cobra.Command {
	var sessionID string
	command := &cobra.Command{Use: "verify", Short: "Verify append-only ledger hash chains", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		if err := service.Store.VerifyLedger(command.Context(), sessionID); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "ledger: verified")
		return err
	}, Args: cobra.NoArgs}
	command.Flags().StringVar(&sessionID, "session", "", "verify one session")
	return command
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
		resolution, err := service.ResolveCurrent(command.Context(), explicitProject)
		if err != nil {
			return err
		}
		output := currentProjectOutput{ProjectSlug: resolution.ProjectSlug, Method: string(resolution.Method), Confidence: string(resolution.Confidence), Evidence: resolution.Evidence}
		if resolution.ProjectSlug != "" {
			_, location, found, err := service.Store.ProjectBySlug(command.Context(), resolution.ProjectSlug)
			if err != nil {
				return err
			}
			if found {
				output.LocationID = location.ID
				output.LocationPath = location.AbsolutePath
			}
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
	count, err := jsonl.Import(command.Context(), service.Store, reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(command.Root().OutOrStdout(), "imported %d event(s)\n", count)
	return err
}

func newUsageCommand(home *string) *cobra.Command {
	usage := &cobra.Command{Use: "usage", Short: "Show observed usage and allocated cost"}
	for _, period := range []string{"today", "week", "month"} {
		usage.AddCommand(newUsagePeriodCommand(home, period))
	}
	return usage
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
		for _, row := range report.Rows {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s/%s | %d tokens | $%d micros\n", row.ProjectSlug, row.Provider, row.Model, row.TotalTokens, row.AllocatedCostUSDMicros); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "TOTAL | %d tokens | $%d micros\n", report.TotalTokens, report.AllocatedCostUSDMicros)
		return err
	}}
	command.Flags().StringVar(&groupBy, "group-by", "project,provider,model", "comma-separated dimensions")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func storeUsageQuery(from, to time.Time, groupBy string) sqlite.UsageQuery {
	return sqlite.UsageQuery{From: from, To: to, GroupBy: strings.Split(groupBy, ",")}
}

func newReportCommand(home *string) *cobra.Command {
	var from, to, groupBy string
	var jsonOutput bool
	report := &cobra.Command{Use: "report", Aliases: []string{"summary"}, Short: "Summarize observed usage and allocated cost", RunE: func(command *cobra.Command, _ []string) error {
		return runReportSummary(command, home, from, to, groupBy, jsonOutput)
	}}
	report.Flags().StringVar(&from, "from", "", "inclusive RFC3339 or YYYY-MM-DD start")
	report.Flags().StringVar(&to, "to", "", "exclusive RFC3339 or YYYY-MM-DD end")
	report.Flags().StringVar(&groupBy, "group-by", "project,provider,model", "comma-separated dimensions")
	report.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")

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
	return report
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
