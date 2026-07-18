package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestAgentToolHandlersManageProjectTaskAndUsage(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	initialized, err := app.Initialize(ctx, home)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := initialized.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if New(home, "test") == nil {
		t.Fatal("New() returned nil MCP server")
	}
	s := &server{home: home}
	registered, err := callRegister(s, ctx, registerProjectInput{Name: "Agent Project", Path: filepath.Join(t.TempDir(), "project")})
	if err != nil {
		t.Fatalf("register_project error = %v", err)
	}
	if registered.ProjectSlug != "agent-project" {
		t.Fatalf("register_project = %#v", registered)
	}
	if _, _, err := s.addProjectTag(ctx, nil, addProjectTagInput{Project: registered.ProjectSlug, Tag: "team=core"}); err != nil {
		t.Fatalf("add_project_tag error = %v", err)
	}
	current, err := callCurrent(s, ctx, getCurrentProjectInput{Project: registered.ProjectSlug})
	if err != nil || current.ProjectSlug != registered.ProjectSlug || current.Method != "explicit" {
		t.Fatalf("get_current_project = %#v, %v", current, err)
	}
	switched, err := callSwitch(s, ctx, switchProjectInput{Project: registered.ProjectSlug, SessionID: "agent-session"})
	if err != nil || switched.ContextID == "" {
		t.Fatalf("switch_project = %#v, %v", switched, err)
	}
	task, err := callStartTask(s, ctx, startTaskInput{Project: registered.ProjectSlug, Title: "Implement MCP"})
	if err != nil {
		t.Fatalf("start_task error = %v", err)
	}
	finished, err := callFinishTask(s, ctx, finishTaskInput{TaskID: task.ID, Result: "complete"})
	if err != nil || finished.Status != "finished" || finished.Title != "Implement MCP" {
		t.Fatalf("finish_task = %#v, %v", finished, err)
	}
	project, err := callProjectSummary(s, ctx, projectInput{Project: registered.ProjectSlug})
	if err != nil || project.Project.Slug != registered.ProjectSlug || len(project.Tags) != 1 {
		t.Fatalf("get_project_summary = %#v, %v", project, err)
	}
}

func TestAssignUsageRejectsModelCallsWithExistingAllocations(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	initialized, err := app.Initialize(ctx, home)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := initialized.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	service, err := app.Open(ctx, home)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	source, _, err := service.Store.RegisterProject(ctx, "Source", "source", filepath.Join(t.TempDir(), "source"))
	if err != nil {
		t.Fatalf("RegisterProject(source) error = %v", err)
	}
	target, _, err := service.Store.RegisterProject(ctx, "Target", "target", filepath.Join(t.TempDir(), "target"))
	if err != nil {
		t.Fatalf("RegisterProject(target) error = %v", err)
	}
	callID, err := service.Store.RecordModelCall(ctx, sqlite.ModelCallInput{ProjectID: source.ID, Provider: "provider", ModelID: "model"})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}

	s := &server{home: home}
	if _, _, err := s.assignUsage(ctx, nil, assignUsageInput{ModelCallID: callID, Project: target.Slug}); err == nil {
		t.Fatal("assignUsage() accepted a model call with an existing allocation")
	}
	allocations, err := service.Store.ModelCallAllocations(ctx, callID)
	if err != nil {
		t.Fatalf("ModelCallAllocations() error = %v", err)
	}
	if len(allocations) != 1 || allocations[0].ProjectID != source.ID || allocations[0].BasisPoints != 10_000 || allocations[0].Method != "direct" {
		t.Fatalf("allocations after assignUsage() = %#v", allocations)
	}
}

func callRegister(s *server, ctx context.Context, input registerProjectInput) (registerProjectOutput, error) {
	_, output, err := s.registerProject(ctx, nil, input)
	return output, err
}

func callCurrent(s *server, ctx context.Context, input getCurrentProjectInput) (projectContext, error) {
	_, output, err := s.getCurrentProject(ctx, nil, input)
	return output, err
}

func callSwitch(s *server, ctx context.Context, input switchProjectInput) (projectContext, error) {
	_, output, err := s.switchProject(ctx, nil, input)
	return output, err
}

func callStartTask(s *server, ctx context.Context, input startTaskInput) (sqlite.TaskRecord, error) {
	_, output, err := s.startTask(ctx, nil, input)
	return output, err
}

func callFinishTask(s *server, ctx context.Context, input finishTaskInput) (sqlite.TaskSummary, error) {
	_, output, err := s.finishTask(ctx, nil, input)
	return output, err
}

func callProjectSummary(s *server, ctx context.Context, input projectInput) (sqlite.ProjectReport, error) {
	_, output, err := s.getProjectSummary(ctx, nil, input)
	return output, err
}
