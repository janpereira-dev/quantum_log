// Package wrapper captures privacy-safe lifecycle evidence around arbitrary CLIs.
package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type Config struct {
	Project string
	Agent   string
	Command []string
	Input   io.Reader
	Output  io.Writer
	Errors  io.Writer
}

type Result struct {
	SessionID string
	ExitCode  int
	Duration  time.Duration
}

// Run records lifecycle metadata only. Arguments, environment and process output
// are intentionally never persisted because they can contain user content or secrets.
func Run(ctx context.Context, service *app.Service, config Config) (Result, error) {
	if len(config.Command) == 0 || config.Command[0] == "" {
		return Result{}, errors.New("command is required")
	}
	resolved, err := service.ResolveProject(ctx, config.Project, "", "")
	if err != nil {
		return Result{}, err
	}
	agent := config.Agent
	if agent == "" {
		agent = filepath.Base(config.Command[0])
	}
	startedAt := time.Now().UTC()
	sessionID := newSessionID()
	if err := service.Store.EnsureSession(ctx, sessionID, agent, startedAt); err != nil {
		return Result{}, err
	}

	command := exec.CommandContext(ctx, config.Command[0], config.Command[1:]...)
	command.Stdin = config.Input
	command.Stdout = config.Output
	command.Stderr = config.Errors
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start wrapped command: %w", err)
	}
	pid := command.Process.Pid
	if _, err := appendEvent(ctx, service, resolved, sessionID, "process.started", startedAt, processMetadata{Agent: agent, Command: filepath.Base(config.Command[0]), PID: pid, StartedAt: startedAt, CWD: resolved.CWD, GitRoot: resolved.GitRoot}); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return Result{}, err
	}

	runErr := command.Wait()
	finishedAt := time.Now().UTC()
	result := Result{SessionID: sessionID, ExitCode: exitCode(runErr), Duration: finishedAt.Sub(startedAt)}
	_, appendErr := appendEvent(ctx, service, resolved, sessionID, "process.exited", finishedAt, processMetadata{Agent: agent, Command: filepath.Base(config.Command[0]), PID: pid, StartedAt: startedAt, FinishedAt: &finishedAt, ExitCode: result.ExitCode, DurationMS: result.Duration.Milliseconds(), CWD: resolved.CWD, GitRoot: resolved.GitRoot})
	if appendErr != nil {
		return result, appendErr
	}
	if runErr != nil {
		return result, fmt.Errorf("wrapped command exited with code %d: %w", result.ExitCode, runErr)
	}
	return result, nil
}

type processMetadata struct {
	Agent      string     `json:"agent"`
	Command    string     `json:"command"`
	PID        int        `json:"pid"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExitCode   int        `json:"exit_code,omitempty"`
	DurationMS int64      `json:"duration_ms,omitempty"`
	CWD        string     `json:"working_directory,omitempty"`
	GitRoot    string     `json:"git_root,omitempty"`
}

func appendEvent(ctx context.Context, service *app.Service, resolved app.ResolvedProject, sessionID, eventType string, occurredAt time.Time, metadata processMetadata) (string, error) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return service.Store.AppendRawEvent(ctx, storepkg.RawEventInput{Source: "generic-cli-wrapper", SessionID: sessionID, EventType: eventType, Payload: payload, OccurredAt: occurredAt, ProjectID: resolved.ProjectID, ProjectLocationID: resolved.LocationID, ResolutionMethod: string(resolved.Resolution.Method), ResolutionConfidence: string(resolved.Resolution.Confidence), EvidenceJSON: `{"source":"central-project-resolver"}`})
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

func newSessionID() string { return fmt.Sprintf("wrap-%d", time.Now().UTC().UnixNano()) }
