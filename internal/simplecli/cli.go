// Package simplecli exposes the on-demand, local-only Quantum Log product.
package simplecli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/ingest/codexsession"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type Version struct {
	Version, Commit, BuildDate string
}

type Row struct {
	Project string `json:"project"`
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	Tokens  int64  `json:"tokens"`
	Quality string `json:"quality"`
}

type Summary struct {
	UsageSamples int64  `json:"usage_samples"`
	TotalTokens  int64  `json:"total_tokens"`
	CostQuality  string `json:"cost_quality"`
	Rows         []Row  `json:"rows"`
}

type ImportResult struct {
	Files      int `json:"files"`
	Candidates int `json:"candidate_files"`
	Samples    int `json:"samples"`
	Duplicates int `json:"duplicates"`
	Skipped    int `json:"skipped"`
}

const help = `Quantum Log — local Codex usage, on demand

Usage:
  qlog                         Show imported usage
  qlog import [flags] [path]    Read local Codex session logs
  qlog report [flags]          Show usage by project and model
  qlog export [flags]          Write an aggregate CSV to stdout

Import flags: --days 7 --limit 100 --json
Report flags: --today --project <slug> --json
Export flags: --today --project <slug>
Global flags: --home <directory> --version --help

Import reads the newest 100 files modified in the last 7 days by default.
An explicit file is imported regardless of its age. Put flags before the path.
No service, watcher, startup entry, or agent configuration is installed.
Usage samples are counter intervals, not a count of model calls. Costs are unavailable.
`

// Run performs one operation and returns; it never starts background processes.
func Run(ctx context.Context, args []string, out, errOut io.Writer, version Version) error {
	home, args, err := extractHome(args)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			_, err := io.WriteString(out, help)
			return err
		case "--version":
			_, err := fmt.Fprintf(out, "qlog %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
			return err
		}
	}
	if home == "" {
		paths, err := config.Resolve("")
		if err != nil {
			return err
		}
		home = filepath.Join(paths.Home, "simple")
	}
	command := "report"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	switch command {
	case "import":
		return importSessions(ctx, home, args, out, errOut)
	case "report", "export":
		return report(ctx, home, command, args, out, errOut)
	default:
		return fmt.Errorf("unknown command %q; run qlog --help", command)
	}
}

func extractHome(args []string) (string, []string, error) {
	var home string
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--home":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", nil, errors.New("--home requires a directory")
			}
			i++
			home = args[i]
		case strings.HasPrefix(args[i], "--home="):
			home = strings.TrimPrefix(args[i], "--home=")
		default:
			remaining = append(remaining, args[i])
		}
	}
	return home, remaining, nil
}

func importSessions(ctx context.Context, home string, args []string, out, errOut io.Writer) (resultErr error) {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(errOut)
	days := flags.Int("days", 7, "include files modified in the last N days")
	limit := flags.Int("limit", 100, "maximum files per import (1-100)")
	jsonOutput := flags.Bool("json", false, "output a JSON import result")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("import accepts one file or directory; put flags before the path")
	}
	if *days < 1 || *days > 36500 || *limit < 1 || *limit > 100 {
		return errors.New("--days must be 1-36500 and --limit must be 1-100")
	}
	path := flags.Arg(0)
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("inspect explicit source: %w", err)
		}
	}
	if path == "" {
		base := os.Getenv("CODEX_HOME")
		if base == "" {
			var err error
			base, err = os.UserHomeDir()
			if err != nil {
				return err
			}
			base = filepath.Join(base, ".codex")
		}
		path = filepath.Join(base, "sessions")
	}
	files, candidates, err := discover(ctx, path, time.Now().AddDate(0, 0, -*days), *limit)
	if err != nil {
		return err
	}
	result := ImportResult{Candidates: candidates}
	if len(files) > 0 {
		service, err := app.Initialize(ctx, home)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, service.Close()) }()
		importer := codexsession.New(service)
		for _, path := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open source session: %w", err)
			}
			imported, importErr := importer.Import(ctx, file)
			closeErr := file.Close()
			if err := errors.Join(importErr, closeErr); err != nil {
				return fmt.Errorf("import session %s: %w", filepath.Base(path), err)
			}
			result.Files++
			result.Samples += imported.Samples
			result.Duplicates += imported.Duplicates
			result.Skipped += imported.Skipped
		}
	}
	if *jsonOutput {
		return json.NewEncoder(out).Encode(result)
	}
	_, err = fmt.Fprintf(out, "Imported %d usage samples from %d files; %d duplicates, %d skipped.\n", result.Samples, result.Files, result.Duplicates, result.Skipped)
	if err != nil {
		return err
	}
	if result.Candidates > result.Files {
		_, err = fmt.Fprintf(out, "Selected the newest %d of %d recent files. Use an explicit file or narrower directory for older evidence.\n", result.Files, result.Candidates)
	}
	if len(files) == 0 {
		_, err = fmt.Fprintln(out, "No recent session files found. Use qlog import <file> for an older session.")
	}
	return err
}

type candidate struct {
	path     string
	modified time.Time
}

func discover(ctx context.Context, root string, since time.Time, limit int) ([]string, int, error) {
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("inspect source: %w", err)
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, 0, errors.New("source must be a regular file or directory")
		}
		return []string{root}, 1, nil
	}
	selected := make([]candidate, 0, limit+1)
	count := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return fmt.Errorf("read source directory: %w", walkErr)
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.ModTime().Before(since) {
			return nil
		}
		count++
		selected = append(selected, candidate{path, info.ModTime()})
		sort.Slice(selected, func(i, j int) bool {
			if selected[i].modified.Equal(selected[j].modified) {
				return selected[i].path < selected[j].path
			}
			return selected[i].modified.After(selected[j].modified)
		})
		if len(selected) > limit {
			selected = selected[:limit]
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	paths := make([]string, len(selected))
	for i, file := range selected {
		paths[i] = file.path
	}
	return paths, count, nil
}

func report(ctx context.Context, home, command string, args []string, out, errOut io.Writer) (resultErr error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	today := flags.Bool("today", false, "filter the current local calendar day")
	project := flags.String("project", "", "filter one project slug")
	var jsonOutput bool
	if command == "report" {
		flags.BoolVar(&jsonOutput, "json", false, "output JSON")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("report and export accept flags only")
	}
	summary := Summary{CostQuality: "unavailable", Rows: make([]Row, 0)}
	paths, err := config.Resolve(home)
	if err != nil {
		return err
	}
	if _, err := os.Stat(paths.Database); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	} else if err == nil {
		service, err := app.OpenSnapshotReadOnly(ctx, home)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, service.Close()) }()
		query := sqlite.UsageQuery{ProjectSlug: *project, GroupBy: []string{"project", "agent", "model", "capture_quality"}}
		if *today {
			now := time.Now()
			query.From = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			query.To = query.From.AddDate(0, 0, 1)
		}
		usage, err := service.Store.Usage(ctx, query)
		if err != nil {
			return err
		}
		summary.TotalTokens = usage.TotalTokens
		for _, measure := range usage.Measurements {
			summary.UsageSamples += measure.ModelCallCount
		}
		for _, row := range usage.Rows {
			summary.Rows = append(summary.Rows, Row{Project: row.ProjectSlug, Agent: row.AgentName, Model: row.Model, Tokens: row.TotalTokens, Quality: row.CaptureQuality})
		}
	}
	if command == "export" {
		writer := csv.NewWriter(out)
		if err := writer.Write([]string{"project", "agent", "model", "tokens", "quality", "cost_quality"}); err != nil {
			return err
		}
		for _, row := range summary.Rows {
			if err := writer.Write([]string{row.Project, row.Agent, row.Model, strconv.FormatInt(row.Tokens, 10), row.Quality, summary.CostQuality}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	}
	if jsonOutput {
		return json.NewEncoder(out).Encode(summary)
	}
	if summary.UsageSamples == 0 {
		_, err := fmt.Fprintln(out, "No imported usage. Run qlog import, then qlog report.")
		return err
	}
	if _, err := fmt.Fprintf(out, "Usage samples: %d | Tokens: %d | Cost: unavailable\n", summary.UsageSamples, summary.TotalTokens); err != nil {
		return err
	}
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "PROJECT\tAGENT\tMODEL\tTOKENS\tQUALITY"); err != nil {
		return err
	}
	for _, row := range summary.Rows {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\n", row.Project, row.Agent, row.Model, row.Tokens, row.Quality); err != nil {
			return err
		}
	}
	return writer.Flush()
}
