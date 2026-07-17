// Package mcpserver exposes application workflows to MCP clients over stdio.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type server struct {
	home string
}

type projectContext struct {
	ProjectSlug string `json:"project_slug,omitempty"`
	LocationID  string `json:"project_location_id,omitempty"`
	ContextID   string `json:"work_context_id,omitempty"`
	Method      string `json:"method"`
	Confidence  string `json:"confidence"`
	Evidence    string `json:"evidence"`
}

type registerProjectInput struct {
	Name string `json:"name" jsonschema:"human-readable project name"`
	Slug string `json:"slug,omitempty" jsonschema:"stable project slug; defaults to a normalized name"`
	Path string `json:"path" jsonschema:"local project directory"`
}

type registerProjectOutput struct {
	ProjectID   string `json:"project_id"`
	ProjectSlug string `json:"project_slug"`
	LocationID  string `json:"project_location_id"`
}

type getCurrentProjectInput struct {
	Project string `json:"project,omitempty" jsonschema:"optional explicit registered project slug"`
	CWD     string `json:"cwd,omitempty" jsonschema:"agent working directory used for resolution"`
}

type switchProjectInput struct {
	Project   string `json:"project" jsonschema:"registered project slug to activate"`
	SessionID string `json:"session_id,omitempty" jsonschema:"optional agent session identifier"`
	CWD       string `json:"cwd,omitempty" jsonschema:"agent working directory"`
}

type addProjectTagInput struct {
	Project string `json:"project" jsonschema:"registered project slug"`
	Tag     string `json:"tag" jsonschema:"normalized tag in key=value form"`
}

type startTaskInput struct {
	Project string `json:"project" jsonschema:"registered project slug"`
	Title   string `json:"title" jsonschema:"task title"`
	Type    string `json:"type,omitempty" jsonschema:"task type; defaults to other"`
}

type finishTaskInput struct {
	TaskID string `json:"task_id" jsonschema:"active task identifier"`
	Result string `json:"result,omitempty" jsonschema:"completed task outcome"`
}

type projectInput struct {
	Project string `json:"project" jsonschema:"registered project slug"`
}

type allocationPart struct {
	Project     string `json:"project" jsonschema:"registered project slug"`
	BasisPoints int64  `json:"basis_points" jsonschema:"share in basis points; all shares must total 10000"`
}

type assignUsageInput struct {
	ModelCallID string `json:"model_call_id" jsonschema:"unattributed model call identifier"`
	Project     string `json:"project" jsonschema:"registered project slug"`
}

type splitUsageInput struct {
	ModelCallID string           `json:"model_call_id" jsonschema:"model call identifier"`
	Allocations []allocationPart `json:"allocations" jsonschema:"allocation shares totaling 10000 basis points"`
}

type allocationsOutput struct {
	Allocations []sqlite.Allocation `json:"allocations"`
}

// New constructs a stdio-safe MCP server. Tool handlers open their own local
// database connection, keeping the protocol adapter separate from core logic.
func New(home, version string) *mcp.Server {
	implementation := &mcp.Implementation{Name: "quantum-log", Version: version}
	mcpServer := mcp.NewServer(implementation, nil)
	s := &server{home: home}
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "register_project", Description: "Register or reuse a local project location."}, s.registerProject)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "get_current_project", Description: "Resolve current project using explicit project or agent CWD."}, s.getCurrentProject)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "switch_project", Description: "Record a project work context for an agent session."}, s.switchProject)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "add_project_tag", Description: "Add a normalized key=value tag to a project."}, s.addProjectTag)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "start_task", Description: "Start a task in a registered project."}, s.startTask)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "finish_task", Description: "Finish a task and return its recorded usage summary."}, s.finishTask)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "get_project_summary", Description: "Return recorded project, task, usage, tag, and budget-alert summary."}, s.getProjectSummary)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "get_unattributed_summary", Description: "List model calls without allocations and their repair queue."}, s.getUnattributedSummary)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "assign_usage", Description: "Assign one unattributed model call to a project. Does not change observed tokens."}, s.assignUsage)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "split_usage", Description: "Split one model call cost across projects with basis points totaling 10000."}, s.splitUsage)
	return mcpServer
}

// Run serves MCP JSON-RPC on stdin/stdout until its client disconnects.
func Run(ctx context.Context, home, version string) error {
	return New(home, version).Run(ctx, &mcp.StdioTransport{})
}

func (s *server) open(ctx context.Context) (*app.Service, error) {
	return app.Open(ctx, s.home)
}

func (s *server) registerProject(ctx context.Context, _ *mcp.CallToolRequest, input registerProjectInput) (*mcp.CallToolResult, registerProjectOutput, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, registerProjectOutput{}, err
	}
	defer func() { _ = service.Close() }()
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	project, location, err := service.Store.RegisterProject(ctx, input.Name, slug, input.Path)
	if err != nil {
		return nil, registerProjectOutput{}, err
	}
	return nil, registerProjectOutput{ProjectID: project.ID, ProjectSlug: project.Slug, LocationID: location.ID}, nil
}

func (s *server) getCurrentProject(ctx context.Context, _ *mcp.CallToolRequest, input getCurrentProjectInput) (*mcp.CallToolResult, projectContext, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, projectContext{}, err
	}
	defer func() { _ = service.Close() }()
	resolved, err := service.ResolveProject(ctx, input.Project, "", input.CWD)
	if err != nil {
		return nil, projectContext{}, err
	}
	return nil, contextOutput(resolved, ""), nil
}

func (s *server) switchProject(ctx context.Context, _ *mcp.CallToolRequest, input switchProjectInput) (*mcp.CallToolResult, projectContext, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, projectContext{}, err
	}
	defer func() { _ = service.Close() }()
	resolved, err := service.ResolveProject(ctx, input.Project, "", input.CWD)
	if err != nil {
		return nil, projectContext{}, err
	}
	if resolved.ProjectID == "" {
		return nil, projectContext{}, fmt.Errorf("project %q could not be resolved", input.Project)
	}
	evidence, _ := json.Marshal(map[string]string{"source": "mcp", "resolution": resolved.Resolution.Evidence})
	workContext, err := service.Store.CreateWorkContext(ctx, sqlite.WorkContextInput{
		ProjectID: resolved.ProjectID, LocationID: resolved.LocationID, SessionID: input.SessionID, CWD: resolved.CWD, GitRoot: resolved.GitRoot,
		ResolutionMethod: string(resolved.Resolution.Method), ResolutionConfidence: string(resolved.Resolution.Confidence), EvidenceJSON: string(evidence),
	})
	if err != nil {
		return nil, projectContext{}, err
	}
	return nil, contextOutput(resolved, workContext.ID), nil
}

func (s *server) addProjectTag(ctx context.Context, _ *mcp.CallToolRequest, input addProjectTagInput) (*mcp.CallToolResult, sqlite.ProjectTag, error) {
	key, value, err := splitTag(input.Tag)
	if err != nil {
		return nil, sqlite.ProjectTag{}, err
	}
	service, err := s.open(ctx)
	if err != nil {
		return nil, sqlite.ProjectTag{}, err
	}
	defer func() { _ = service.Close() }()
	project, _, found, err := service.Store.ProjectBySlug(ctx, input.Project)
	if err != nil {
		return nil, sqlite.ProjectTag{}, err
	}
	if !found {
		return nil, sqlite.ProjectTag{}, fmt.Errorf("project %q not found", input.Project)
	}
	if err := service.Store.AddProjectTag(ctx, project.ID, key, value); err != nil {
		return nil, sqlite.ProjectTag{}, err
	}
	return nil, sqlite.ProjectTag{Key: strings.ToLower(key), Value: strings.ToLower(value)}, nil
}

func (s *server) startTask(ctx context.Context, _ *mcp.CallToolRequest, input startTaskInput) (*mcp.CallToolResult, sqlite.TaskRecord, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, sqlite.TaskRecord{}, err
	}
	defer func() { _ = service.Close() }()
	project, _, found, err := service.Store.ProjectBySlug(ctx, input.Project)
	if err != nil {
		return nil, sqlite.TaskRecord{}, err
	}
	if !found {
		return nil, sqlite.TaskRecord{}, fmt.Errorf("project %q not found", input.Project)
	}
	taskType := input.Type
	if taskType == "" {
		taskType = "other"
	}
	id, err := service.Store.StartTask(ctx, sqlite.TaskInput{ProjectID: project.ID, Title: input.Title, TaskType: taskType})
	if err != nil {
		return nil, sqlite.TaskRecord{}, err
	}
	summary, err := service.Store.TaskSummary(ctx, id)
	return nil, summary.TaskRecord, err
}

func (s *server) finishTask(ctx context.Context, _ *mcp.CallToolRequest, input finishTaskInput) (*mcp.CallToolResult, sqlite.TaskSummary, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, sqlite.TaskSummary{}, err
	}
	defer func() { _ = service.Close() }()
	if err := service.Store.FinishTask(ctx, input.TaskID, input.Result); err != nil {
		return nil, sqlite.TaskSummary{}, err
	}
	summary, err := service.Store.TaskSummary(ctx, input.TaskID)
	return nil, summary, err
}

func (s *server) getProjectSummary(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, sqlite.ProjectReport, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, sqlite.ProjectReport{}, err
	}
	defer func() { _ = service.Close() }()
	report, err := service.Store.ProjectReport(ctx, input.Project, time.Now().UTC())
	return nil, report, err
}

func (s *server) getUnattributedSummary(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, sqlite.UnattributedSummary, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, sqlite.UnattributedSummary{}, err
	}
	defer func() { _ = service.Close() }()
	summary, err := service.Store.UnattributedSummary(ctx)
	return nil, summary, err
}

func (s *server) assignUsage(ctx context.Context, _ *mcp.CallToolRequest, input assignUsageInput) (*mcp.CallToolResult, allocationsOutput, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, allocationsOutput{}, err
	}
	defer func() { _ = service.Close() }()
	project, _, found, err := service.Store.ProjectBySlug(ctx, input.Project)
	if err != nil {
		return nil, allocationsOutput{}, err
	}
	if !found {
		return nil, allocationsOutput{}, fmt.Errorf("project %q not found", input.Project)
	}
	if err := service.Store.RepairModelCallAllocation(ctx, input.ModelCallID, project.ID); err != nil {
		return nil, allocationsOutput{}, err
	}
	allocations, err := service.Store.ModelCallAllocations(ctx, input.ModelCallID)
	return nil, allocationsOutput{Allocations: allocations}, err
}

func (s *server) splitUsage(ctx context.Context, _ *mcp.CallToolRequest, input splitUsageInput) (*mcp.CallToolResult, allocationsOutput, error) {
	service, err := s.open(ctx)
	if err != nil {
		return nil, allocationsOutput{}, err
	}
	defer func() { _ = service.Close() }()
	allocations := make([]sqlite.AllocationInput, 0, len(input.Allocations))
	for _, part := range input.Allocations {
		project, _, found, err := service.Store.ProjectBySlug(ctx, part.Project)
		if err != nil {
			return nil, allocationsOutput{}, err
		}
		if !found {
			return nil, allocationsOutput{}, fmt.Errorf("project %q not found", part.Project)
		}
		allocations = append(allocations, sqlite.AllocationInput{ProjectID: project.ID, BasisPoints: part.BasisPoints})
	}
	if err := service.Store.ReplaceAllocations(ctx, "model_call", input.ModelCallID, allocations); err != nil {
		return nil, allocationsOutput{}, err
	}
	result, err := service.Store.ModelCallAllocations(ctx, input.ModelCallID)
	return nil, allocationsOutput{Allocations: result}, err
}

func contextOutput(resolved app.ResolvedProject, contextID string) projectContext {
	return projectContext{ProjectSlug: resolved.Resolution.ProjectSlug, LocationID: resolved.LocationID, ContextID: contextID, Method: string(resolved.Resolution.Method), Confidence: string(resolved.Resolution.Confidence), Evidence: resolved.Resolution.Evidence}
}

func splitTag(value string) (string, string, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("tag must use key=value")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	return strings.Trim(value, "-")
}
