package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

func newUnattributedCommand(home *string) *cobra.Command {
	command := &cobra.Command{Use: "unattributed", Short: "Inspect and repair model calls without allocations"}
	var listJSON bool
	list := &cobra.Command{Use: "list", Short: "List unattributed model calls and guided repair IDs", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		summary, err := service.Store.UnattributedSummary(command.Context())
		if err != nil {
			return err
		}
		if listJSON {
			return writeJSON(command.Root().OutOrStdout(), summary)
		}
		if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%d model call(s) | %d tokens | $%d micros\n", summary.ModelCallCount, summary.ObservedTokens, summary.EstimatedCostUSDMicros); err != nil {
			return err
		}
		for _, call := range summary.ModelCalls {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s | %s/%s | %d tokens | repair: qlog unattributed repair %s --project <slug>\n", call.ID, call.Provider, call.Model, call.TotalTokens, call.ID); err != nil {
				return err
			}
		}
		return nil
	}}
	list.Flags().BoolVar(&listJSON, "json", false, "output JSON")
	var projectSlug string
	repair := &cobra.Command{Use: "repair <model-call-id>", Short: "Assign one unattributed model call to an explicit project", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
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
		if err := service.Store.RepairModelCallAllocation(command.Context(), args[0], project.ID); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "unattributed usage: assigned")
		return err
	}}
	repair.Flags().StringVar(&projectSlug, "project", "", "project slug")
	_ = repair.MarkFlagRequired("project")
	command.AddCommand(list, repair)
	return command
}

func newBudgetCommand(home *string) *cobra.Command {
	command := &cobra.Command{Use: "budget", Short: "Configure monthly allocated-cost alerts; budgets do not block usage"}
	command.AddCommand(newBudgetSetProjectCommand(home), newBudgetSetTagCommand(home), newBudgetStatusCommand(home))
	return command
}

func newBudgetSetProjectCommand(home *string) *cobra.Command {
	var micros, alertPercent int64
	command := &cobra.Command{Use: "set-project <slug>", Short: "Set a project monthly cost alert budget", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		project, _, found, err := service.Store.ProjectBySlug(command.Context(), args[0])
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("project %q not found", args[0])
		}
		_, err = service.Store.SetBudget(command.Context(), sqlite.BudgetInput{Scope: "project", Target: project.ID, MonthlyCostUSDMicros: micros, AlertPercent: alertPercent})
		return err
	}}
	command.Flags().Int64Var(&micros, "monthly-usd-micros", 0, "monthly allocated-cost threshold in USD micros")
	command.Flags().Int64Var(&alertPercent, "alert-percent", 80, "warning threshold percent")
	_ = command.MarkFlagRequired("monthly-usd-micros")
	return command
}

func newBudgetSetTagCommand(home *string) *cobra.Command {
	var micros, alertPercent int64
	command := &cobra.Command{Use: "set-tag <key=value>", Short: "Set a tag monthly cost alert budget", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return fmt.Errorf("tag must use key=value")
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		_, err = service.Store.SetBudget(command.Context(), sqlite.BudgetInput{Scope: "tag", Target: strings.ToLower(args[0]), MonthlyCostUSDMicros: micros, AlertPercent: alertPercent})
		return err
	}}
	command.Flags().Int64Var(&micros, "monthly-usd-micros", 0, "monthly allocated-cost threshold in USD micros")
	command.Flags().Int64Var(&alertPercent, "alert-percent", 80, "warning threshold percent")
	_ = command.MarkFlagRequired("monthly-usd-micros")
	return command
}

func newBudgetStatusCommand(home *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "status", Short: "Show current-month allocated-cost budget alerts", RunE: func(command *cobra.Command, _ []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		alerts, err := service.Store.BudgetAlerts(command.Context(), time.Now().UTC())
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), alerts)
		}
		for _, alert := range alerts {
			if _, err := fmt.Fprintf(command.Root().OutOrStdout(), "%s %s | %s | %s/%s USD micros\n", alert.Scope, alert.Target, alert.Alert, strconv.FormatInt(alert.AllocatedCostUSDMicros, 10), strconv.FormatInt(alert.MonthlyCostUSDMicros, 10)); err != nil {
				return err
			}
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}
