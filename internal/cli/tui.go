package cli

import (
	"context"
	"io"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/janpereira-dev/quantum_log/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newTUICommand(home *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the accessible terminal dashboard",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runTUI(command.Context(), command.OutOrStdout(), *home)
		},
	}
}

func runTUI(ctx context.Context, output io.Writer, home string) error {
	overview, err := loadOverview(ctx, home)
	if err != nil {
		return err
	}
	program := tea.NewProgram(tui.New(overview, os.Getenv("NO_COLOR") != ""), tea.WithContext(ctx), tea.WithOutput(output))
	_, err = program.Run()
	return err
}

func loadOverview(ctx context.Context, home string) (tui.OverviewData, error) {
	service, err := app.Open(ctx, home)
	if err != nil {
		return tui.OverviewData{}, err
	}
	defer func() { _ = service.Close() }()

	usage, err := service.Store.Usage(ctx, sqlite.UsageQuery{GroupBy: []string{"project", "provider", "model"}})
	if err != nil {
		return tui.OverviewData{}, err
	}
	projects, err := service.Store.ListProjects(ctx)
	if err != nil {
		return tui.OverviewData{}, err
	}
	tasks, err := service.Store.ListTasks(ctx, "")
	if err != nil {
		return tui.OverviewData{}, err
	}

	data := tui.OverviewData{
		TotalTokens:            usage.TotalTokens,
		AllocatedCostUSDMicros: usage.AllocatedCostUSDMicros,
		Projects:               make([]tui.Project, 0, len(projects)),
		Tasks:                  make([]tui.Task, 0, len(tasks)),
		Integrity:              tui.Integrity{SQLite: "ok", Ledger: "verified"},
	}
	for _, project := range projects {
		data.Projects = append(data.Projects, tui.Project{Name: project.Name, Slug: project.Slug, LocationCount: project.LocationCount, TagCount: project.TagCount})
	}
	for _, task := range tasks {
		data.Tasks = append(data.Tasks, tui.Task{ProjectSlug: task.ProjectSlug, Title: task.Title, Type: task.TaskType, Status: task.Status})
	}
	if err := service.Store.Doctor(ctx); err != nil {
		data.Integrity.SQLite = err.Error()
	}
	if err := service.Store.VerifyLedger(ctx, ""); err != nil {
		data.Integrity.Ledger = err.Error()
	}
	return data, nil
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
