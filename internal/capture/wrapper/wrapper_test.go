package wrapper

import (
	"context"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
)

func TestRunRecordsLifecycleWithoutDependingOnArguments(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	worktree := filepath.Join(t.TempDir(), "project")
	if _, _, err := service.Store.RegisterProject(ctx, "Project", "project", worktree); err != nil {
		t.Fatalf("register project: %v", err)
	}
	command := []string{"cmd", "/C", "exit", "0"}
	if runtime.GOOS != "windows" {
		command = []string{"sh", "-c", "exit 0"}
	}
	result, err := Run(ctx, service, Config{Project: "project", Agent: "test-agent", Command: command, Input: io.Reader(nil), Output: io.Discard, Errors: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.SessionID == "" || result.ExitCode != 0 {
		t.Fatalf("Run() = %#v", result)
	}
	if err := service.Store.VerifyLedger(ctx, result.SessionID); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
}
