// Package adapters defines verified capture integration contracts.
package adapters

import (
	"context"
	"errors"
	"io"
	"sort"
)

// Capabilities reports only data an adapter can actually provide.
type Capabilities struct {
	ModelIdentity       bool `json:"model_identity"`
	InputTokens         bool `json:"input_tokens"`
	OutputTokens        bool `json:"output_tokens"`
	ReasoningTokens     bool `json:"reasoning_tokens"`
	CacheTokens         bool `json:"cache_tokens"`
	ContextUsage        bool `json:"context_usage"`
	ToolCalls           bool `json:"tool_calls"`
	MCPCalls            bool `json:"mcp_calls"`
	Costs               bool `json:"costs"`
	PromptSizes         bool `json:"prompt_sizes"`
	ResponseSizes       bool `json:"response_sizes"`
	SessionLifecycle    bool `json:"session_lifecycle"`
	TaskMetadata        bool `json:"task_metadata"`
	ProjectIdentity     bool `json:"project_identity"`
	WorkingDirectory    bool `json:"working_directory"`
	VCSContext          bool `json:"vcs_context"`
	WorkspaceContext    bool `json:"workspace_context"`
	ProjectSwitchEvents bool `json:"project_switch_events"`
	StructuredEvents    bool `json:"structured_events"`
}

type Descriptor struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Capabilities Capabilities `json:"capabilities"`
}

type Detection struct {
	Available bool   `json:"available"`
	Evidence  string `json:"evidence"`
}

type InstallOptions struct {
	DryRun bool
}

type InstallResult struct {
	Changed bool     `json:"changed"`
	Actions []string `json:"actions"`
}

type RawRecord struct {
	Source  string
	Payload []byte
}

type ProjectSignals struct {
	Project   string
	CWD       string
	GitRoot   string
	Workspace string
}

// Adapter has one stable lifecycle. It emits signals; app.Service resolves projects.
type Adapter interface {
	Descriptor() Descriptor
	Detect(context.Context) (Detection, error)
	Install(context.Context, InstallOptions) (InstallResult, error)
	Uninstall(context.Context, InstallOptions) (InstallResult, error)
	PlanInstall(context.Context, SetupOptions) (SetupPlan, error)
	Status(context.Context) (SetupStatus, error)
	Test(context.Context) (TestResult, error)
	HealthCheck(context.Context) error
	Ingest(context.Context, io.Reader) ([]RawRecord, error)
	Normalize(RawRecord) (RawRecord, error)
	ExtractProjectSignals(RawRecord) ProjectSignals
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry(items ...Adapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[string]Adapter, len(items))}
	for _, item := range items {
		descriptor := item.Descriptor()
		if descriptor.ID == "" {
			return nil, errors.New("adapter id is required")
		}
		if _, exists := registry.adapters[descriptor.ID]; exists {
			return nil, errors.New("duplicate adapter id " + descriptor.ID)
		}
		registry.adapters[descriptor.ID] = item
	}
	return registry, nil
}

func (r *Registry) Get(id string) (Adapter, bool) {
	adapter, found := r.adapters[id]
	return adapter, found
}

func (r *Registry) List() []Adapter {
	items := make([]Adapter, 0, len(r.adapters))
	ids := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		items = append(items, r.adapters[id])
	}
	return items
}

func Default() *Registry {
	registry, err := NewRegistry(GenericJSONL{}, commandAdapter{id: "opencode", name: "OpenCode", executable: "opencode"}, commandAdapter{id: "claude-code", name: "Claude Code", executable: "claude"})
	if err != nil {
		panic(err)
	}
	return registry
}
