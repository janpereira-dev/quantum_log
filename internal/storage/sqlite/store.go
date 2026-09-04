// Package sqlite persists QUANTUM_LOG data locally using a CGo-free driver.
package sqlite

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/audit"
	"github.com/janpereira-dev/quantum_log/internal/domain"
	"github.com/janpereira-dev/quantum_log/internal/pricing"
	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	db         *sql.DB
	quiescence *storelock.Handle
	writerLock *storelock.Handle
	writable   bool
	warnings   []string
}

type WorkContextInput struct {
	ProjectID            string
	LocationID           string
	SessionID            string
	CWD                  string
	GitRoot              string
	GitBranch            string
	GitCommit            string
	StartedAt            time.Time
	ResolutionMethod     string
	ResolutionConfidence string
	EvidenceJSON         string
}

type RawEventInput struct {
	IngestionIdentity          string
	Source                     string
	SourceVersion              string
	SessionID                  string
	EventType                  string
	TraceID                    string
	SpanID                     string
	ParentSpanID               string
	Payload                    []byte
	OccurredAt                 time.Time
	OmitOccurredAtFromIdentity bool
	ProjectID                  string
	ProjectLocationID          string
	WorkContextID              string
	ResolutionMethod           string
	ResolutionConfidence       string
	EvidenceJSON               string
	acceptanceBoundaryMarker   bool
}

type RawEventAppendResult struct {
	ID                string
	Accepted          bool
	SuppressionReason string
}

// AcceptanceBoundaryMarker is the authoritative ledger record created by qlog
// when a real-agent boundary is opened. It is deliberately not model evidence.
type AcceptanceBoundaryMarker struct {
	BoundaryID           string `json:"boundary_id"`
	Challenge            string `json:"challenge"`
	LedgerPositionSHA256 string `json:"ledger_position_sha256"`
	LedgerEventCount     int64  `json:"ledger_event_count"`
}

const (
	acceptanceBoundarySource    = "qlog.acceptance"
	acceptanceBoundaryEventType = "acceptance.boundary.v1"
)

type AllocationInput struct {
	ProjectID   string
	BasisPoints int64
	Method      string
	Confidence  string
}

// AllocationRevisionInput describes one immutable allocation decision.
type AllocationRevisionInput struct {
	SubjectType        string
	SubjectID          string
	Allocations        []AllocationInput
	IdempotencyKey     string
	Author             string
	Source             string
	Reason             string
	Method             string
	RequireUnallocated bool
}

// AllocationRevision is the authoritative history record for an allocation
// change. Its entries are immutable; usage_allocations is only a projection.
type AllocationRevision struct {
	ID               string       `json:"id"`
	SubjectType      string       `json:"subject_type"`
	SubjectID        string       `json:"subject_id"`
	RevisionNumber   int64        `json:"revision_number"`
	ParentRevisionID string       `json:"parent_revision_id,omitempty"`
	IdempotencyKey   string       `json:"idempotency_key"`
	Author           string       `json:"author"`
	Source           string       `json:"source"`
	Reason           string       `json:"reason"`
	PreviousHash     string       `json:"previous_hash,omitempty"`
	RevisionHash     string       `json:"revision_hash,omitempty"`
	Allocations      []Allocation `json:"allocations"`
	CreatedAt        time.Time    `json:"created_at"`
}

type ModelCallInput struct {
	RawEventID             string
	InteractionID          string
	InteractionUpstreamID  string
	ProjectID              string
	ProjectLocationID      string
	WorkContextID          string
	TaskID                 string
	SessionID              string
	TurnID                 string
	Provider               string
	ModelID                string
	AgentName              string
	InputTokens            int64
	OutputTokens           int64
	ReasoningTokens        int64
	CachedInputTokens      int64
	CacheWriteTokens       int64
	EstimatedCostUSDMicros int64
	EstimatedCostEURMicros int64
	OccurredAt             time.Time
	CompletedAt            time.Time
	DurationMS             *int64
	CaptureQuality         string
	Metrics                []MetricInput
}

type InteractionInput struct {
	RawEventID        string
	Source            string
	SessionID         string
	UpstreamID        string
	ProjectID         string
	ProjectLocationID string
	WorkContextID     string
	PromptCaptureMode string
	PromptHash        string
	PromptRedacted    string
	OccurredAt        time.Time
}

type Interaction struct {
	ID                string    `json:"id"`
	Source            string    `json:"source"`
	SessionID         string    `json:"session_id"`
	UpstreamID        string    `json:"upstream_id"`
	PromptCaptureMode string    `json:"prompt_capture_mode"`
	PromptHash        string    `json:"prompt_hash,omitempty"`
	PromptRedacted    string    `json:"prompt_redacted,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

// ToolCallInput is deliberately metadata-only: tool arguments and results never
// cross the normalization boundary.
type ToolCallInput struct {
	RawEventID            string
	InteractionID         string
	InteractionUpstreamID string
	ProjectID             string
	LocationID            string
	WorkContextID         string
	SessionID             string
	ToolName              string
	ToolType              string
	OccurredAt            time.Time
	CaptureQuality        string
}

// MetricInput is one explicitly emitted measurement. Absence is represented
// by no input, while a zero value remains a reported measurement.
type MetricInput struct {
	Name       string
	Value      *int64
	Source     string
	RawKey     string
	Confidence string
}

type UsageQuery struct {
	From        time.Time
	To          time.Time
	ProjectSlug string
	GroupBy     []string
}

type AdapterEvidenceQuery struct {
	AdapterID                     string
	AllowedAgentNames             []string
	Source                        string
	From                          time.Time
	To                            time.Time
	ProjectSlug                   string
	RequiredQuality               string
	RequiredProvider              string
	RequireCodexResponseCompleted bool
}

type UsageRow struct {
	ProjectSlug            string `json:"project_slug"`
	AgentName              string `json:"agent_name"`
	Provider               string `json:"provider"`
	Model                  string `json:"model"`
	CaptureQuality         string `json:"capture_quality"`
	InputTokens            int64  `json:"input_tokens"`
	OutputTokens           int64  `json:"output_tokens"`
	ReasoningTokens        int64  `json:"reasoning_tokens"`
	CachedInputTokens      int64  `json:"cached_input_tokens"`
	CacheWriteTokens       int64  `json:"cache_write_tokens"`
	TotalTokens            int64  `json:"total_tokens"`
	AllocatedCostUSDMicros int64  `json:"allocated_cost_usd_micros"`
}

// MeasurementSummary keeps reported and estimated measurements separate.
// Raw lifecycle evidence is counted without turning it into token usage.
type MeasurementSummary struct {
	Quality                string `json:"capture_quality"`
	RawEventCount          int64  `json:"raw_event_count"`
	ModelCallCount         int64  `json:"model_call_count"`
	InputTokens            int64  `json:"input_tokens"`
	OutputTokens           int64  `json:"output_tokens"`
	ReasoningTokens        int64  `json:"reasoning_tokens"`
	CachedInputTokens      int64  `json:"cached_input_tokens"`
	CacheWriteTokens       int64  `json:"cache_write_tokens"`
	TotalTokens            int64  `json:"total_tokens"`
	EstimatedCostUSDMicros int64  `json:"estimated_cost_usd_micros"`
}

type UsageReport struct {
	GroupBy                []string             `json:"group_by"`
	Rows                   []UsageRow           `json:"rows"`
	Measurements           []MeasurementSummary `json:"measurements"`
	TotalTokens            int64                `json:"total_tokens"`
	AllocatedCostUSDMicros int64                `json:"allocated_cost_usd_micros"`
}

// CapabilityQuery scopes one auditable report without inferring ownership.
type CapabilityQuery struct {
	From        time.Time
	To          time.Time
	ProjectSlug string
	AgentName   string
	SessionID   string
}

type MetricProvenance struct {
	Source     string `json:"source"`
	RawKey     string `json:"raw_key"`
	Confidence string `json:"confidence"`
	Count      int64  `json:"count"`
}

// MetricCoverage preserves absence independently from zero. Value is only
// populated when every included model call emitted a reconcilable value.
type MetricCoverage struct {
	Name              string             `json:"name"`
	State             string             `json:"state"`
	Value             *int64             `json:"value"`
	ReportedCount     int64              `json:"reported_count"`
	MissingCount      int64              `json:"missing_count"`
	ReportedZeroCount int64              `json:"reported_zero_count"`
	Provenance        []MetricProvenance `json:"provenance"`
}

type SourceCoverage struct {
	Source     string  `json:"source"`
	Quality    string  `json:"capture_quality"`
	Version    *string `json:"version"`
	ModelCalls int64   `json:"model_calls"`
}

type CapabilityReport struct {
	From                   time.Time        `json:"from,omitempty"`
	To                     time.Time        `json:"to,omitempty"`
	ProjectSlug            string           `json:"project_slug,omitempty"`
	AgentName              string           `json:"agent_name,omitempty"`
	SessionID              string           `json:"session_id,omitempty"`
	ModelCalls             int64            `json:"model_calls"`
	Interactions           int64            `json:"interactions"`
	Prompts                int64            `json:"prompts"`
	Tokens                 int64            `json:"tokens"`
	LifecycleEvents        int64            `json:"lifecycle_events"`
	ToolCalls              int64            `json:"tool_calls"`
	MCPCalls               int64            `json:"mcp_calls"`
	Errors                 int64            `json:"errors"`
	UnattributedModelCalls int64            `json:"unattributed_model_calls"`
	UnattributedTokens     int64            `json:"unattributed_tokens"`
	MetricCoverage         []MetricCoverage `json:"metric_coverage"`
	Sources                []SourceCoverage `json:"sources"`
}

func (report CapabilityReport) MarshalJSON() ([]byte, error) {
	type encodedReport struct {
		From                   *time.Time       `json:"from,omitempty"`
		To                     *time.Time       `json:"to,omitempty"`
		ProjectSlug            string           `json:"project_slug,omitempty"`
		AgentName              string           `json:"agent_name,omitempty"`
		SessionID              string           `json:"session_id,omitempty"`
		ModelCalls             int64            `json:"model_calls"`
		Interactions           int64            `json:"interactions"`
		Prompts                int64            `json:"prompts"`
		Tokens                 int64            `json:"tokens"`
		LifecycleEvents        int64            `json:"lifecycle_events"`
		ToolCalls              int64            `json:"tool_calls"`
		MCPCalls               int64            `json:"mcp_calls"`
		Errors                 int64            `json:"errors"`
		UnattributedModelCalls int64            `json:"unattributed_model_calls"`
		UnattributedTokens     int64            `json:"unattributed_tokens"`
		MetricCoverage         []MetricCoverage `json:"metric_coverage"`
		Sources                []SourceCoverage `json:"sources"`
	}
	encoded := encodedReport{ProjectSlug: report.ProjectSlug, AgentName: report.AgentName, SessionID: report.SessionID, ModelCalls: report.ModelCalls, Interactions: report.Interactions, Prompts: report.Prompts, Tokens: report.Tokens, LifecycleEvents: report.LifecycleEvents, ToolCalls: report.ToolCalls, MCPCalls: report.MCPCalls, Errors: report.Errors, UnattributedModelCalls: report.UnattributedModelCalls, UnattributedTokens: report.UnattributedTokens, MetricCoverage: report.MetricCoverage, Sources: report.Sources}
	if !report.From.IsZero() {
		encoded.From = &report.From
	}
	if !report.To.IsZero() {
		encoded.To = &report.To
	}
	return json.Marshal(encoded)
}

type TaskInput struct {
	ProjectID string
	Title     string
	TaskType  string
}

type ProjectSummary struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	LocationCount int64     `json:"location_count"`
	TagCount      int64     `json:"tag_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type ProjectTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TaskRecord struct {
	ID          string     `json:"id"`
	ProjectSlug string     `json:"project_slug"`
	Title       string     `json:"title"`
	TaskType    string     `json:"task_type"`
	Status      string     `json:"status"`
	Result      string     `json:"result"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// TaskSummary reports task lifecycle data plus usage already recorded against it.
// It does not infer usage from an agent session or project.
type TaskSummary struct {
	TaskRecord
	ModelCallCount         int64                `json:"model_call_count"`
	ObservedTokens         int64                `json:"observed_tokens"`
	AllocatedCostUSDMicros int64                `json:"allocated_cost_usd_micros"`
	Measurements           []MeasurementSummary `json:"measurements"`
}

type ProjectReport struct {
	Project                ProjectSummary       `json:"project"`
	Tags                   []ProjectTag         `json:"tags"`
	ActiveTaskCount        int64                `json:"active_task_count"`
	ObservedModelCallCount int64                `json:"observed_model_call_count"`
	ObservedTokens         int64                `json:"observed_tokens"`
	AllocatedCostUSDMicros int64                `json:"allocated_cost_usd_micros"`
	BudgetAlerts           []BudgetAlert        `json:"budget_alerts"`
	Measurements           []MeasurementSummary `json:"measurements"`
}

// SessionSnapshot records evidence already stored for one session. Lifecycle
// events remain raw evidence and have no fabricated model-call measurements.
type SessionSnapshot struct {
	SessionID            string               `json:"session_id"`
	AgentName            string               `json:"agent_name"`
	StartedAt            time.Time            `json:"started_at"`
	LastEventAt          time.Time            `json:"last_event_at"`
	RawEventCount        int64                `json:"raw_event_count"`
	LifecycleEventCount  int64                `json:"lifecycle_event_count"`
	ModelCallCount       int64                `json:"model_call_count"`
	Measurements         []MeasurementSummary `json:"measurements"`
	ResolutionMethod     string               `json:"resolution_method"`
	ResolutionConfidence string               `json:"resolution_confidence"`
}

type UnattributedModelCall struct {
	ID                     string    `json:"id"`
	OccurredAt             time.Time `json:"occurred_at"`
	Provider               string    `json:"provider"`
	Model                  string    `json:"model"`
	TotalTokens            int64     `json:"total_tokens"`
	EstimatedCostUSDMicros int64     `json:"estimated_cost_usd_micros"`
}

type UnattributedSummary struct {
	ModelCallCount         int64                   `json:"model_call_count"`
	ObservedTokens         int64                   `json:"observed_tokens"`
	EstimatedCostUSDMicros int64                   `json:"estimated_cost_usd_micros"`
	ModelCalls             []UnattributedModelCall `json:"model_calls"`
}

type BudgetInput struct {
	Scope                string
	Target               string
	MonthlyCostUSDMicros int64
	AlertPercent         int64
}

type BudgetRecord struct {
	ID                   string `json:"id"`
	Scope                string `json:"scope"`
	Target               string `json:"target"`
	MonthlyCostUSDMicros int64  `json:"monthly_cost_usd_micros"`
	AlertPercent         int64  `json:"alert_percent"`
}

type BudgetAlert struct {
	BudgetRecord
	AllocatedCostUSDMicros int64  `json:"allocated_cost_usd_micros"`
	ThresholdUSDMicros     int64  `json:"threshold_usd_micros"`
	Alert                  string `json:"alert"`
}

type PricingRuleRecord struct {
	ID        string       `json:"id"`
	Rule      pricing.Rule `json:"rule"`
	CreatedAt time.Time    `json:"created_at"`
}

type PricingRecalculateQuery struct {
	From time.Time
	To   time.Time
}

type Allocation struct {
	ProjectID   string `json:"project_id"`
	ProjectSlug string `json:"project_slug"`
	BasisPoints int64  `json:"basis_points"`
	Method      string `json:"method"`
	Confidence  string `json:"confidence"`
}

type ExportRecord struct {
	ID                     string       `json:"id"`
	OccurredAt             time.Time    `json:"occurred_at"`
	ProjectSlug            string       `json:"project_slug"`
	ProjectLocationPath    string       `json:"project_location_path,omitempty"`
	Provider               string       `json:"provider"`
	Model                  string       `json:"model"`
	Agent                  string       `json:"agent"`
	InputTokens            int64        `json:"input_tokens"`
	OutputTokens           int64        `json:"output_tokens"`
	ReasoningTokens        int64        `json:"reasoning_tokens"`
	CachedInputTokens      int64        `json:"cached_input_tokens"`
	CacheWriteTokens       int64        `json:"cache_write_tokens"`
	TotalTokens            int64        `json:"total_tokens"`
	EstimatedCostUSDMicros int64        `json:"estimated_cost_usd_micros"`
	CaptureQuality         string       `json:"capture_quality"`
	Allocations            []Allocation `json:"allocations"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := ensureParent(absolutePath); err != nil {
		return nil, err
	}
	quiescence, err := storelock.AcquireSharedCreate(quiescenceLockPath(absolutePath))
	if err != nil {
		return nil, writerQuiescenceError(err)
	}
	if err := rejectPurgeMarker(absolutePath); err != nil {
		_ = quiescence.Close()
		return nil, err
	}
	writerLock, err := storelock.AcquireExclusive(writerLockPath(absolutePath))
	if err != nil {
		_ = quiescence.Close()
		return nil, writerLockError(err)
	}
	// modernc accepts a SQLite URI with a Windows-safe forward-slash path.
	dsn := "file:" + filepath.ToSlash(absolutePath) + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writerLock.Close()
		_ = quiescence.Close()
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, quiescence: quiescence, writerLock: writerLock, writable: true}
	if err := store.migrate(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

// OpenReadOnly opens an initialized database without creating files or applying migrations.
func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(absolutePath); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	quiescence, err := storelock.AcquireExclusiveExisting(quiescenceLockPath(absolutePath))
	if err != nil {
		return nil, readerQuiescenceError(err)
	}
	if err := rejectPurgeMarker(absolutePath); err != nil {
		_ = quiescence.Close()
		return nil, err
	}
	if _, err := os.Stat(writerLockPath(absolutePath)); err != nil {
		_ = quiescence.Close()
		return nil, readerWriterLockError(err)
	}
	if err := rejectActiveWAL(absolutePath); err != nil {
		_ = quiescence.Close()
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(absolutePath) + "?mode=ro&immutable=1&_pragma=query_only(1)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = quiescence.Close()
		return nil, fmt.Errorf("open read-only sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_ = quiescence.Close()
		return nil, fmt.Errorf("open read-only sqlite: %w", err)
	}
	store := &Store{db: db, quiescence: quiescence, warnings: isolatedSHMWarning(absolutePath)}
	if err := store.validateSchema(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

// OpenSnapshotReadOnly opens a WAL-aware read snapshot while a qlog writer is active.
// It shares the cooperative quiescence lock and never uses immutable SQLite mode.
func OpenSnapshotReadOnly(ctx context.Context, path string) (*Store, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(absolutePath); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	quiescence, err := storelock.AcquireShared(quiescenceLockPath(absolutePath))
	if err != nil {
		return nil, readerQuiescenceError(err)
	}
	if err := rejectPurgeMarker(absolutePath); err != nil {
		_ = quiescence.Close()
		return nil, err
	}
	if _, err := os.Stat(writerLockPath(absolutePath)); err != nil {
		_ = quiescence.Close()
		return nil, readerWriterLockError(err)
	}
	dsn := "file:" + filepath.ToSlash(absolutePath) + "?mode=ro&_pragma=query_only(1)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = quiescence.Close()
		return nil, fmt.Errorf("open snapshot sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_ = quiescence.Close()
		return nil, fmt.Errorf("open snapshot sqlite: %w", err)
	}
	store := &Store{db: db, quiescence: quiescence}
	if err := store.validateSchema(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	var result error
	if s.writable {
		if err := s.checkpointWAL(context.Background()); err != nil {
			result = errors.Join(result, err)
		}
	}
	if err := s.db.Close(); err != nil {
		result = errors.Join(result, err)
	}
	if s.writerLock != nil {
		result = errors.Join(result, s.writerLock.Close())
	}
	if s.quiescence != nil {
		result = errors.Join(result, s.quiescence.Close())
	}
	return result
}

func (s *Store) Warnings() []string { return append([]string(nil), s.warnings...) }

// Checkpoint validates and checkpoints a quiescent local ledger without migrations.
func Checkpoint(ctx context.Context, path string) (result error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(absolutePath); err != nil {
		return fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	quiescence, err := storelock.AcquireExclusiveExisting(quiescenceLockPath(absolutePath))
	if err != nil {
		return maintenanceQuiescenceError(err)
	}
	defer func() { result = errors.Join(result, quiescence.Close()) }()
	if err := rejectPurgeMarker(absolutePath); err != nil {
		return err
	}
	writerLock, err := storelock.AcquireExclusiveExisting(writerLockPath(absolutePath))
	if err != nil {
		return maintenanceWriterLockError(err)
	}
	defer func() { result = errors.Join(result, writerLock.Close()) }()

	dsn := "file:" + filepath.ToSlash(absolutePath) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite for maintenance: %w", err)
	}
	defer func() { result = errors.Join(result, db.Close()) }()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("open sqlite for maintenance: %w", err)
	}
	store := &Store{db: db}
	if err := store.VerifyLedger(ctx, ""); err != nil {
		return fmt.Errorf("validate ledger before checkpoint: %w", err)
	}
	if err := store.checkpointWAL(ctx); err != nil {
		return err
	}
	if err := rejectActiveWAL(absolutePath); err != nil {
		return fmt.Errorf("confirm cleared WAL: %w", err)
	}
	return nil
}

func (s *Store) backfillReconstructableIngestionIdentities(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.source, COALESCE(r.session_id, ''), r.event_type, r.occurred_at, COALESCE(r.project_id, ''), COALESCE(r.project_location_id, ''), COALESCE(r.work_context_id, ''), r.project_resolution_method, r.project_resolution_confidence, r.project_resolution_evidence_json, r.payload_json_sanitized, r.created_at FROM raw_events r LEFT JOIN raw_event_dedup d ON d.raw_event_id = r.id WHERE d.raw_event_id IS NULL`)
	if err != nil {
		return fmt.Errorf("find raw events needing ingestion backfill: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		identity string
		id       string
		source   string
		created  string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var id, source, sessionID, eventType, occurredAt, projectID, locationID, workContextID, method, confidence, evidence, payload, created string
		if err := rows.Scan(&id, &source, &sessionID, &eventType, &occurredAt, &projectID, &locationID, &workContextID, &method, &confidence, &evidence, &payload, &created); err != nil {
			return fmt.Errorf("scan raw event ingestion backfill: %w", err)
		}
		identity, err := CanonicalIngestionIdentity(RawEventInput{Source: source, SessionID: sessionID, EventType: eventType, OccurredAt: parseTimestamp(occurredAt), ProjectID: projectID, ProjectLocationID: locationID, WorkContextID: workContextID, ResolutionMethod: method, ResolutionConfidence: confidence, EvidenceJSON: evidence}, []byte(payload))
		if err != nil {
			return fmt.Errorf("canonical raw event ingestion backfill: %w", err)
		}
		candidates = append(candidates, candidate{identity: identity, id: id, source: source, created: created})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read raw event ingestion backfill: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin raw event ingestion backfill: %w", err)
	}
	defer rollback(tx)
	for _, candidate := range candidates {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO raw_event_dedup (ingestion_identity, raw_event_id, source, first_received_at, last_received_at) VALUES (?, ?, ?, ?, ?)`, candidate.identity, candidate.id, candidate.source, candidate.created, candidate.created); err != nil {
			return fmt.Errorf("backfill ingestion identity: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit raw event ingestion backfill: %w", err)
	}
	return nil
}

func (s *Store) checkpointWAL(ctx context.Context) error {
	var busy, logFrames, checkpointedFrames int
	if err := s.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return fmt.Errorf("checkpoint WAL: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("WAL checkpoint busy: busy=%d log_frames=%d checkpointed_frames=%d", busy, logFrames, checkpointedFrames)
	}
	return nil
}

func (s *Store) RegisterProject(ctx context.Context, name, slug, path string) (domain.Project, domain.ProjectLocation, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return domain.Project{}, domain.ProjectLocation{}, errors.New("project slug is required")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, fmt.Errorf("resolve project path: %w", err)
	}
	gitRoot, gitRemote := registeredGitContext(ctx, absolutePath)
	now := timestamp(time.Now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, fmt.Errorf("begin registration: %w", err)
	}
	defer rollback(tx)

	project, location, found, err := projectByLocation(ctx, tx, absolutePath)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, err
	}
	if found {
		if err := tx.Commit(); err != nil {
			return domain.Project{}, domain.ProjectLocation{}, err
		}
		if gitRoot != "" && gitRemote != "" {
			if err := s.SetVerifiedGitContext(ctx, project.ID, location.ID, gitRoot, gitRemote); err != nil {
				return domain.Project{}, domain.ProjectLocation{}, err
			}
		}
		return project, location, nil
	}

	project, found, err = projectBySlug(ctx, tx, slug)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, err
	}
	if !found {
		project = domain.Project{ID: newID(), Slug: slug, Name: name, CanonicalKey: "local:" + slug, CreatedAt: time.Now().UTC()}
		_, err = tx.ExecContext(ctx, `INSERT INTO projects (id, slug, name, canonical_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, project.ID, project.Slug, project.Name, project.CanonicalKey, now, now)
		if err != nil {
			return domain.Project{}, domain.ProjectLocation{}, fmt.Errorf("insert project: %w", err)
		}
	}

	location = domain.ProjectLocation{ID: newID(), ProjectID: project.ID, AbsolutePath: absolutePath, PathHash: hashPath(absolutePath), CreatedAt: time.Now().UTC()}
	_, err = tx.ExecContext(ctx, `INSERT INTO project_locations (id, project_id, absolute_path, path_hash, first_seen_at, last_seen_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, location.ID, location.ProjectID, location.AbsolutePath, location.PathHash, now, now, now, now)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, fmt.Errorf("insert project location: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Project{}, domain.ProjectLocation{}, fmt.Errorf("commit registration: %w", err)
	}
	if gitRoot != "" && gitRemote != "" {
		if err := s.SetVerifiedGitContext(ctx, project.ID, location.ID, gitRoot, gitRemote); err != nil {
			return domain.Project{}, domain.ProjectLocation{}, err
		}
	}
	return project, location, nil
}

func registeredGitContext(ctx context.Context, path string) (string, string) {
	root, err := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", ""
	}
	remote, err := exec.CommandContext(ctx, "git", "-C", path, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(string(root)), strings.TrimSpace(string(remote))
}

// SetVerifiedGitContext records local Git evidence for one registered location.
func (s *Store) SetVerifiedGitContext(ctx context.Context, projectID, locationID, root, remote string) error {
	root, remote = normalizeLocationPath(root), normalizeGitRemote(remote)
	if projectID == "" || locationID == "" || root == "" || remote == "" {
		return errors.New("verified git context requires project, location, root, and remote")
	}
	var existingRoot, existingRemote sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT l.vcs_root, p.repository_url_normalized FROM project_locations l JOIN projects p ON p.id = l.project_id WHERE l.id = ? AND l.project_id = ?`, locationID, projectID).Scan(&existingRoot, &existingRemote); err != nil {
		return fmt.Errorf("read existing verified git context: %w", err)
	}
	if existingRoot.Valid && existingRoot.String != "" && normalizeLocationPath(existingRoot.String) != root {
		return errors.New("verified git root conflicts with existing project location")
	}
	if existingRemote.Valid && existingRemote.String != "" && normalizeGitRemote(existingRemote.String) != remote {
		return errors.New("verified git remote conflicts with existing project")
	}
	now := timestamp(time.Now())
	if _, err := s.db.ExecContext(ctx, `UPDATE projects SET repository_url_normalized = ?, updated_at = ? WHERE id = ?`, remote, now, projectID); err != nil {
		return fmt.Errorf("store verified git remote: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE project_locations SET vcs_root = ?, updated_at = ? WHERE id = ? AND project_id = ?`, root, now, locationID, projectID); err != nil {
		return fmt.Errorf("store verified git root: %w", err)
	}
	return nil
}

// ProjectByVerifiedGitContext returns only one exact root-and-remote match.
func (s *Store) ProjectByVerifiedGitContext(ctx context.Context, root, remote string) (domain.Project, domain.ProjectLocation, bool, error) {
	root, remote = normalizeLocationPath(root), normalizeGitRemote(remote)
	if root == "" || remote == "" {
		return domain.Project{}, domain.ProjectLocation{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.name, p.canonical_key, p.created_at, l.id, l.project_id, l.absolute_path, l.path_hash, l.created_at FROM project_locations l JOIN projects p ON p.id = l.project_id WHERE LOWER(REPLACE(l.vcs_root, '\', '/')) = ? AND p.repository_url_normalized = ? LIMIT 2`, root, remote)
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, false, fmt.Errorf("query verified git context: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type match struct {
		project  domain.Project
		location domain.ProjectLocation
	}
	matches := []match{}
	for rows.Next() {
		var projectCreatedAt, locationCreatedAt string
		var item match
		if err := rows.Scan(&item.project.ID, &item.project.Slug, &item.project.Name, &item.project.CanonicalKey, &projectCreatedAt, &item.location.ID, &item.location.ProjectID, &item.location.AbsolutePath, &item.location.PathHash, &locationCreatedAt); err != nil {
			return domain.Project{}, domain.ProjectLocation{}, false, fmt.Errorf("scan verified git context: %w", err)
		}
		item.project.CreatedAt, item.location.CreatedAt = parseTimestamp(projectCreatedAt), parseTimestamp(locationCreatedAt)
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Project{}, domain.ProjectLocation{}, false, err
	}
	if len(matches) != 1 {
		return domain.Project{}, domain.ProjectLocation{}, false, nil
	}
	return matches[0].project, matches[0].location, true, nil
}

func normalizeGitRemote(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value, _, _ = strings.Cut(value, "?")
	value, _, _ = strings.Cut(value, "#")
	if at := strings.LastIndex(value, "@"); at >= 0 {
		value = value[at+1:]
	}
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "ssh://")
	value = strings.Replace(value, ":", "/", 1)
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimSuffix(value, ".git")
	return strings.TrimSuffix(value, "/")
}

func (s *Store) CreateWorkContext(ctx context.Context, input WorkContextInput) (domain.WorkContext, error) {
	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now().UTC()
	}
	if input.ResolutionMethod == "" {
		input.ResolutionMethod = "unresolved"
	}
	if input.ResolutionConfidence == "" {
		input.ResolutionConfidence = "unknown"
	}
	if input.EvidenceJSON == "" {
		input.EvidenceJSON = "{}"
	}
	context := domain.WorkContext{ID: newID(), PrimaryProjectID: input.ProjectID, ProjectLocationID: input.LocationID, SessionID: input.SessionID, CWD: input.CWD, GitRoot: input.GitRoot, GitBranch: input.GitBranch, GitCommit: input.GitCommit, StartedAt: input.StartedAt.UTC(), ResolutionMethod: domain.ResolutionMethod(input.ResolutionMethod), Confidence: domain.Confidence(input.ResolutionConfidence), EvidenceJSON: input.EvidenceJSON}
	now := timestamp(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO work_contexts (id, primary_project_id, project_location_id, session_id, cwd, git_root, git_branch, git_commit, started_at, resolution_method, resolution_confidence, resolution_evidence_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, context.ID, nullable(context.PrimaryProjectID), nullable(context.ProjectLocationID), nullable(context.SessionID), context.CWD, context.GitRoot, context.GitBranch, context.GitCommit, timestamp(context.StartedAt), context.ResolutionMethod, context.Confidence, context.EvidenceJSON, now, now)
	if err != nil {
		return domain.WorkContext{}, fmt.Errorf("insert work context: %w", err)
	}
	return context, nil
}

func (s *Store) AppendRawEvent(ctx context.Context, input RawEventInput) (RawEventAppendResult, error) {
	if strings.TrimSpace(input.Source) == "" || strings.TrimSpace(input.EventType) == "" {
		return RawEventAppendResult{}, errors.New("raw event source and type are required")
	}
	if input.Source == acceptanceBoundarySource && input.EventType == acceptanceBoundaryEventType && !input.acceptanceBoundaryMarker {
		return RawEventAppendResult{}, errors.New("acceptance boundary markers are qlog-owned")
	}
	if input.OccurredAt.IsZero() {
		input.OmitOccurredAtFromIdentity = true
		input.OccurredAt = time.Now().UTC()
	}
	payload, err := sanitizePayload(input.Payload)
	if err != nil {
		return RawEventAppendResult{}, err
	}
	if input.ResolutionMethod == "" {
		input.ResolutionMethod = "unresolved"
	}
	if input.ResolutionConfidence == "" {
		input.ResolutionConfidence = "unknown"
	}
	if input.EvidenceJSON == "" {
		input.EvidenceJSON = "{}"
	}
	sanitizedEvidence, err := sanitizeEvidence(input.EvidenceJSON)
	if err != nil {
		return RawEventAppendResult{}, fmt.Errorf("sanitize evidence: %w", err)
	}
	input.EvidenceJSON = sanitizedEvidence
	ingestionIdentity, err := CanonicalIngestionIdentity(input, payload)
	if err != nil {
		return RawEventAppendResult{}, fmt.Errorf("canonical ingestion identity: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RawEventAppendResult{}, fmt.Errorf("begin raw event: %w", err)
	}
	defer rollback(tx)
	id := newID()
	now := timestamp(time.Now())
	if _, err := tx.ExecContext(ctx, `INSERT INTO raw_event_dedup (ingestion_identity, raw_event_id, source, first_received_at, last_received_at) VALUES (?, ?, ?, ?, ?)`, ingestionIdentity, id, input.Source, now, now); err != nil {
		if !isUniqueConstraint(err) {
			return RawEventAppendResult{}, fmt.Errorf("reserve ingestion identity: %w", err)
		}
		var existingID string
		if err := tx.QueryRowContext(ctx, `SELECT raw_event_id FROM raw_event_dedup WHERE ingestion_identity = ?`, ingestionIdentity).Scan(&existingID); err != nil {
			return RawEventAppendResult{}, fmt.Errorf("read duplicate ingestion identity: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE raw_event_dedup SET last_received_at = ?, suppression_count = suppression_count + 1 WHERE ingestion_identity = ?`, now, ingestionIdentity); err != nil {
			return RawEventAppendResult{}, fmt.Errorf("record duplicate ingestion identity: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return RawEventAppendResult{}, fmt.Errorf("commit duplicate ingestion identity: %w", err)
		}
		return RawEventAppendResult{ID: existingID, SuppressionReason: "duplicate_ingestion_identity"}, nil
	}
	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT event_hash FROM raw_events WHERE source = ? AND COALESCE(session_id, '') = ? ORDER BY created_at DESC, id DESC LIMIT 1`, input.Source, input.SessionID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return RawEventAppendResult{}, fmt.Errorf("read ledger head: %w", err)
	}
	canonical := canonicalEvent(input, payload)
	event := audit.NewRecord(chainKey(input.Source, input.SessionID), canonical, previousHash)
	_, err = tx.ExecContext(ctx, `INSERT INTO raw_events (id, source, source_version, event_type, occurred_at, received_at, trace_id, span_id, parent_span_id, project_id, project_location_id, work_context_id, session_id, project_resolution_method, project_resolution_confidence, project_resolution_evidence_json, payload_json_sanitized, previous_event_hash, event_hash, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, input.Source, strings.TrimSpace(input.SourceVersion), input.EventType, timestamp(input.OccurredAt), now, input.TraceID, input.SpanID, input.ParentSpanID, nullable(input.ProjectID), nullable(input.ProjectLocationID), nullable(input.WorkContextID), nullable(input.SessionID), input.ResolutionMethod, input.ResolutionConfidence, input.EvidenceJSON, string(payload), previousHash, event.Hash, now)
	if err != nil {
		return RawEventAppendResult{}, fmt.Errorf("insert raw event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return RawEventAppendResult{}, fmt.Errorf("commit raw event: %w", err)
	}
	return RawEventAppendResult{ID: id, Accepted: true}, nil
}

// AppendAcceptanceBoundaryMarker persists a qlog-owned checkpoint in the
// append-only ledger. The marker is a control record and must never be treated
// as agent activity or model evidence.
func (s *Store) AppendAcceptanceBoundaryMarker(ctx context.Context, marker AcceptanceBoundaryMarker, occurredAt time.Time) (RawEventAppendResult, error) {
	if marker.BoundaryID == "" || !validSHA256(marker.Challenge) || !validSHA256(marker.LedgerPositionSHA256) || marker.LedgerEventCount < 0 {
		return RawEventAppendResult{}, errors.New("invalid acceptance boundary marker")
	}
	payload, err := json.Marshal(marker)
	if err != nil {
		return RawEventAppendResult{}, fmt.Errorf("encode acceptance boundary marker: %w", err)
	}
	return s.AppendRawEvent(ctx, RawEventInput{Source: acceptanceBoundarySource, EventType: acceptanceBoundaryEventType, Payload: payload, OccurredAt: occurredAt, OmitOccurredAtFromIdentity: false, acceptanceBoundaryMarker: true})
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// HasAcceptanceBoundaryMarker verifies that the exact qlog-created marker is
// present in the authoritative append-only ledger.
func (s *Store) HasAcceptanceBoundaryMarker(ctx context.Context, marker AcceptanceBoundaryMarker) (bool, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, `SELECT payload_json_sanitized FROM raw_events WHERE source = ? AND event_type = ? AND json_extract(payload_json_sanitized, '$.boundary_id') = ? ORDER BY created_at DESC, id DESC LIMIT 1`, acceptanceBoundarySource, acceptanceBoundaryEventType, marker.BoundaryID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read acceptance boundary marker: %w", err)
	}
	var stored AcceptanceBoundaryMarker
	if err := json.Unmarshal([]byte(payload), &stored); err != nil {
		return false, fmt.Errorf("decode acceptance boundary marker: %w", err)
	}
	return stored == marker, nil
}

func (s *Store) HasModelCallForRawEvent(ctx context.Context, rawEventID string) (bool, error) {
	if rawEventID == "" {
		return false, nil
	}
	var found int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM model_calls WHERE raw_event_id = ?`, rawEventID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read raw event model call: %w", err)
	}
	return true, nil
}

// RecordInteraction inserts one canonical root for an upstream prompt. A
// repeated upstream delivery returns existing root rather than another prompt.
func (s *Store) RecordInteraction(ctx context.Context, input InteractionInput) (string, bool, error) {
	if strings.TrimSpace(input.Source) == "" || strings.TrimSpace(input.UpstreamID) == "" {
		return "", false, errors.New("interaction source and upstream id are required")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if input.PromptCaptureMode == "" {
		input.PromptCaptureMode = "hash"
	}
	if input.PromptCaptureMode != "off" && input.PromptCaptureMode != "hash" && input.PromptCaptureMode != "full" {
		return "", false, errors.New("prompt capture mode must be off, hash, or full")
	}
	id := newID()
	now := timestamp(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO interactions (id, source, session_id, upstream_id, raw_event_id, primary_project_id, project_location_id, work_context_id, prompt_capture_mode, prompt_hash, prompt_redacted, occurred_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, input.Source, input.SessionID, input.UpstreamID, nullable(input.RawEventID), nullable(input.ProjectID), nullable(input.ProjectLocationID), nullable(input.WorkContextID), input.PromptCaptureMode, input.PromptHash, input.PromptRedacted, timestamp(input.OccurredAt), now)
	if err == nil {
		if err := s.backfillInteractionChildren(ctx, id, input.Source, input.SessionID, input.UpstreamID); err != nil {
			return "", false, err
		}
		return id, true, nil
	}
	if !isUniqueConstraint(err) {
		return "", false, fmt.Errorf("insert interaction: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM interactions WHERE source = ? AND session_id = ? AND upstream_id = ?`, input.Source, input.SessionID, input.UpstreamID).Scan(&id); err != nil {
		return "", false, fmt.Errorf("read duplicate interaction: %w", err)
	}
	if err := s.backfillInteractionChildren(ctx, id, input.Source, input.SessionID, input.UpstreamID); err != nil {
		return "", false, err
	}
	return id, false, nil
}

func (s *Store) backfillInteractionChildren(ctx context.Context, interactionID, _ string, sessionID, upstreamID string) error {
	if sessionID == "" || upstreamID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE model_calls SET interaction_id = ?
		WHERE interaction_id IS NULL AND session_id = ? AND interaction_upstream_id = ?`, interactionID, sessionID, upstreamID)
	if err != nil {
		return fmt.Errorf("backfill interaction model calls: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE tool_calls SET interaction_id = ?
		WHERE interaction_id IS NULL AND session_id = ? AND interaction_upstream_id = ?
		AND 1 = (SELECT COUNT(*) FROM interactions i WHERE i.session_id = ? AND i.upstream_id = ?)`, interactionID, sessionID, upstreamID, sessionID, upstreamID)
	if err != nil {
		return fmt.Errorf("backfill interaction tool calls: %w", err)
	}
	return nil
}

func (s *Store) InteractionCount(ctx context.Context, projectID string) (int64, error) {
	query, args := `SELECT COUNT(*) FROM interactions`, []any{}
	if projectID != "" {
		query += ` WHERE primary_project_id = ?`
		args = append(args, projectID)
	}
	var count int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count interactions: %w", err)
	}
	return count, nil
}

func (s *Store) InteractionByUpstream(ctx context.Context, source, sessionID, upstreamID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM interactions WHERE source = ? AND session_id = ? AND upstream_id = ?`, source, sessionID, upstreamID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find interaction: %w", err)
	}
	return id, true, nil
}

// InteractionBySessionUpstream correlates transports only when the parent
// identity is unambiguous within a session. It never guesses across sessions.
func (s *Store) InteractionBySessionUpstream(ctx context.Context, sessionID, upstreamID string) (string, bool, error) {
	if sessionID == "" || upstreamID == "" {
		return "", false, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM interactions WHERE session_id = ? AND upstream_id = ? LIMIT 2`, sessionID, upstreamID)
	if err != nil {
		return "", false, fmt.Errorf("find cross-source interaction: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", false, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	if len(ids) != 1 {
		return "", false, nil
	}
	return ids[0], true, nil
}

func (s *Store) ListInteractions(ctx context.Context, from time.Time, limit int) ([]Interaction, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, source, session_id, upstream_id, prompt_capture_mode, prompt_hash, prompt_redacted, occurred_at FROM interactions`
	args := []any{}
	if !from.IsZero() {
		query += ` WHERE julianday(occurred_at) >= julianday(?)`
		args = append(args, timestamp(from))
	}
	query += ` ORDER BY julianday(occurred_at) DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list interactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	interactions := make([]Interaction, 0)
	for rows.Next() {
		var interaction Interaction
		var occurredAt string
		if err := rows.Scan(&interaction.ID, &interaction.Source, &interaction.SessionID, &interaction.UpstreamID, &interaction.PromptCaptureMode, &interaction.PromptHash, &interaction.PromptRedacted, &occurredAt); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}
		interaction.OccurredAt = parseTimestamp(occurredAt)
		interactions = append(interactions, interaction)
	}
	return interactions, rows.Err()
}

func (s *Store) Interaction(ctx context.Context, id string) (Interaction, bool, error) {
	var interaction Interaction
	var occurredAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, source, session_id, upstream_id, prompt_capture_mode, prompt_hash, prompt_redacted, occurred_at FROM interactions WHERE id = ?`, id).Scan(&interaction.ID, &interaction.Source, &interaction.SessionID, &interaction.UpstreamID, &interaction.PromptCaptureMode, &interaction.PromptHash, &interaction.PromptRedacted, &occurredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Interaction{}, false, nil
	}
	if err != nil {
		return Interaction{}, false, fmt.Errorf("read interaction: %w", err)
	}
	interaction.OccurredAt = parseTimestamp(occurredAt)
	return interaction, true, nil
}

func (s *Store) LinkModelCallInteraction(ctx context.Context, modelCallID, interactionID string) error {
	if modelCallID == "" || interactionID == "" {
		return errors.New("model call and interaction ids are required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE model_calls SET interaction_id = ? WHERE id = ? AND (interaction_id IS NULL OR interaction_id = ?)`, interactionID, modelCallID, interactionID)
	if err != nil {
		return fmt.Errorf("link model call interaction: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("model call not found or linked to another interaction")
	}
	return nil
}

func (s *Store) RecordToolCall(ctx context.Context, input ToolCallInput) (bool, error) {
	if strings.TrimSpace(input.RawEventID) == "" || strings.TrimSpace(input.ToolName) == "" {
		return false, errors.New("tool raw event id and name are required")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if input.CaptureQuality == "" {
		input.CaptureQuality = "lifecycle_only"
	}
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO tool_calls (id, raw_event_id, interaction_id, interaction_upstream_id, primary_project_id, project_location_id, work_context_id, session_id, tool_name, tool_type, started_at, capture_quality, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, newID(), input.RawEventID, nullable(input.InteractionID), input.InteractionUpstreamID, nullable(input.ProjectID), nullable(input.LocationID), nullable(input.WorkContextID), nullable(input.SessionID), input.ToolName, input.ToolType, timestamp(input.OccurredAt), input.CaptureQuality, timestamp(time.Now()))
	if err != nil {
		return false, fmt.Errorf("record tool call: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *Store) VerifyLedger(ctx context.Context, sessionID string) error {
	query := `SELECT source, source_version, COALESCE(session_id, ''), event_type, occurred_at, project_id, project_location_id, work_context_id, project_resolution_method, project_resolution_confidence, project_resolution_evidence_json, payload_json_sanitized, previous_event_hash, event_hash FROM raw_events`
	args := []any{}
	if sessionID != "" {
		query += " WHERE session_id = ?"
		args = append(args, sessionID)
	}
	query += " ORDER BY source, COALESCE(session_id, ''), created_at, id"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	previous := make(map[string]string)
	for rows.Next() {
		var source, sourceVersion, session, eventType, occurredAt, resolutionMethod, resolutionConfidence, evidence, payload, previousHash, eventHash string
		var projectID, locationID, contextID sql.NullString
		if err := rows.Scan(&source, &sourceVersion, &session, &eventType, &occurredAt, &projectID, &locationID, &contextID, &resolutionMethod, &resolutionConfidence, &evidence, &payload, &previousHash, &eventHash); err != nil {
			return fmt.Errorf("scan ledger event: %w", err)
		}
		key := chainKey(source, session)
		if previousHash != previous[key] {
			return errors.New("ledger previous hash does not match")
		}
		canonical := canonicalEvent(RawEventInput{Source: source, SourceVersion: sourceVersion, SessionID: session, EventType: eventType, OccurredAt: parseTimestamp(occurredAt), ProjectID: projectID.String, ProjectLocationID: locationID.String, WorkContextID: contextID.String, ResolutionMethod: resolutionMethod, ResolutionConfidence: resolutionConfidence, EvidenceJSON: evidence}, []byte(payload))
		if audit.Hash(key, canonical, previousHash) != eventHash {
			return errors.New("ledger event hash does not match")
		}
		previous[key] = eventHash
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close ledger rows: %w", err)
	}
	return s.verifyAllocationRevisionChain(ctx, sessionID)
}

func (s *Store) verifyAllocationRevisionChain(ctx context.Context, sessionID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT revision_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, MAX(project_id), MAX(allocation_basis_points), MAX(allocation_method), MAX(confidence), author, source, reason, created_at, previous_revision_hash, revision_hash FROM allocation_revisions r WHERE (? = '' OR EXISTS (SELECT 1 FROM model_calls c WHERE c.id = r.subject_id AND c.session_id = ?)) GROUP BY revision_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at, previous_revision_hash, revision_hash ORDER BY subject_type, subject_id, revision_number, revision_id`, sessionID, sessionID)
	if err != nil {
		return fmt.Errorf("query allocation revisions: %w", err)
	}
	type state struct {
		previousID, previousHash string
		number                   int64
	}
	states := make(map[string]state)
	seen := make(map[string]string)
	for rows.Next() {
		var id, typ, subject, parent, key, project, method, confidence, author, source, reason, created, previousHash, hash string
		var number, bp int64
		if err := rows.Scan(&id, &typ, &subject, &number, &parent, &key, &project, &bp, &method, &confidence, &author, &source, &reason, &created, &previousHash, &hash); err != nil {
			_ = rows.Close()
			return err
		}
		if hash == "" {
			_ = rows.Close()
			return errors.New("allocation revision hash is missing")
		}
		chain := typ + "\x00" + subject
		prior := states[chain]
		if prior.number == 0 {
			if number != 1 || parent != "" || previousHash != "" {
				_ = rows.Close()
				return errors.New("allocation revision chain starts with an invalid parent")
			}
		} else if number != prior.number+1 || parent != prior.previousID || previousHash != prior.previousHash {
			_ = rows.Close()
			return errors.New("allocation revision chain is broken")
		}
		if existing, ok := seen[id]; ok && existing != hash {
			_ = rows.Close()
			return errors.New("allocation revision has inconsistent hash")
		}
		seen[id] = hash
		r := AllocationRevision{ID: id, SubjectType: typ, SubjectID: subject, RevisionNumber: number, ParentRevisionID: parent, IdempotencyKey: key, Author: author, Source: source, Reason: reason, CreatedAt: parseTimestamp(created)}
		got := allocationRevisionHash(r, []AllocationInput{{ProjectID: project, BasisPoints: bp}}, previousHash)
		// A multi-entry revision is hashed over all entries below after grouping;
		// defer content verification to the grouped pass.
		_ = got
		states[chain] = state{previousID: id, previousHash: hash, number: number}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := s.verifyAllocationRevisionContents(ctx, sessionID); err != nil {
		return err
	}
	return s.verifyAllocationRevisionHeads(ctx, sessionID)
}

func (s *Store) verifyAllocationRevisionHeads(ctx context.Context, sessionID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT h.subject_type,h.subject_id,h.revision_id,h.revision_hash FROM allocation_revision_heads h WHERE (? = '' OR EXISTS (SELECT 1 FROM model_calls c WHERE c.id = h.subject_id AND c.session_id = ?))`, sessionID, sessionID)
	if err != nil {
		return err
	}
	type head struct{ typ, subject, id, hash string }
	heads := make([]head, 0)
	for rows.Next() {
		var h head
		if err := rows.Scan(&h.typ, &h.subject, &h.id, &h.hash); err != nil {
			_ = rows.Close()
			return err
		}
		heads = append(heads, h)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, h := range heads {
		var latestID, latestHash string
		if err := s.db.QueryRowContext(ctx, `SELECT revision_id, revision_hash FROM allocation_revisions WHERE subject_type=? AND subject_id=? ORDER BY revision_number DESC, entry_id DESC LIMIT 1`, h.typ, h.subject).Scan(&latestID, &latestHash); err != nil {
			return errors.New("allocation revision head references deleted history")
		}
		if latestID != h.id || latestHash != h.hash {
			return errors.New("allocation revision head does not match terminal revision")
		}
	}
	return nil
}

func (s *Store) verifyAllocationRevisionContents(ctx context.Context, sessionID string) error {
	history, err := s.db.QueryContext(ctx, `SELECT revision_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at, previous_revision_hash, revision_hash FROM allocation_revisions r WHERE (? = '' OR EXISTS (SELECT 1 FROM model_calls c WHERE c.id = r.subject_id AND c.session_id = ?)) GROUP BY revision_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at, previous_revision_hash, revision_hash ORDER BY subject_type, subject_id, revision_number`, sessionID, sessionID)
	if err != nil {
		return err
	}
	items := make([]AllocationRevision, 0)
	for history.Next() {
		var r AllocationRevision
		var created string
		if err := history.Scan(&r.ID, &r.SubjectType, &r.SubjectID, &r.RevisionNumber, &r.ParentRevisionID, &r.IdempotencyKey, &r.Author, &r.Source, &r.Reason, &created, &r.PreviousHash, &r.RevisionHash); err != nil {
			_ = history.Close()
			return err
		}
		r.CreatedAt = parseTimestamp(created)
		items = append(items, r)
	}
	if err := history.Close(); err != nil {
		return err
	}
	if err := history.Err(); err != nil {
		return err
	}
	for _, r := range items {
		allocations, err := revisionAllocations(ctx, s.db, r.ID)
		if err != nil {
			return err
		}
		inputs := make([]AllocationInput, 0, len(allocations))
		for _, a := range allocations {
			inputs = append(inputs, AllocationInput{ProjectID: a.ProjectID, BasisPoints: a.BasisPoints, Method: a.Method, Confidence: a.Confidence})
		}
		if want := allocationRevisionHash(r, inputs, r.PreviousHash); want != r.RevisionHash {
			return errors.New("allocation revision hash does not match content")
		}
		r.Allocations = allocations
		// Keep the complete revision for projection verification.
		for i := range items {
			if items[i].ID == r.ID {
				items[i] = r
				break
			}
		}
	}
	return s.verifyAllocationProjection(ctx, items)
}

func (s *Store) verifyAllocationProjection(ctx context.Context, revisions []AllocationRevision) error {
	for _, revision := range revisions {
		var latest int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM allocation_revisions WHERE subject_type=? AND subject_id=? AND revision_number > ?`, revision.SubjectType, revision.SubjectID, revision.RevisionNumber).Scan(&latest); err != nil {
			return err
		}
		if latest > 0 {
			continue
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_allocations WHERE subject_type=? AND subject_id=?`, revision.SubjectType, revision.SubjectID).Scan(&count); err != nil {
			return err
		}
		if count != len(revision.Allocations) {
			return errors.New("allocation projection does not match latest revision")
		}
		rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(project_id,''), allocation_basis_points, allocation_method, confidence FROM usage_allocations WHERE subject_type=? AND subject_id=?`, revision.SubjectType, revision.SubjectID)
		if err != nil {
			return err
		}
		actual := make([]string, 0, count)
		for rows.Next() {
			var p, m, c string
			var bp int64
			if err := rows.Scan(&p, &bp, &m, &c); err != nil {
				if closeErr := rows.Close(); closeErr != nil {
					return err
				}
				return err
			}
			actual = append(actual, fmt.Sprintf("%s|%d|%s|%s", p, bp, m, c))
		}
		if err := rows.Close(); err != nil {
			return err
		}
		expected := make([]string, 0, len(revision.Allocations))
		for _, a := range revision.Allocations {
			expected = append(expected, fmt.Sprintf("%s|%d|%s|%s", a.ProjectID, a.BasisPoints, a.Method, a.Confidence))
		}
		sort.Strings(actual)
		sort.Strings(expected)
		if len(actual) != len(expected) {
			return errors.New("allocation projection does not match latest revision")
		}
		for i := range expected {
			if actual[i] != expected[i] {
				return errors.New("allocation projection entry does not match latest revision")
			}
		}
	}
	return nil
}

type LedgerAnchor struct {
	Source     string `json:"source"`
	SessionID  string `json:"session_id"`
	HeadHash   string `json:"head_hash"`
	Events     int64  `json:"events"`
	LastSeenAt string `json:"last_seen_at"`
}

func (s *Store) LedgerAnchors(ctx context.Context) ([]LedgerAnchor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source, COALESCE(session_id,''), event_hash, COUNT(*) OVER (PARTITION BY source, COALESCE(session_id,'')) AS event_count, MAX(created_at) OVER (PARTITION BY source, COALESCE(session_id,'')) AS last_seen FROM raw_events ORDER BY source, COALESCE(session_id,''), created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query ledger anchors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	seen := make(map[string]bool)
	var out []LedgerAnchor
	for rows.Next() {
		var source, session, head, lastSeen string
		var count int64
		if err := rows.Scan(&source, &session, &head, &count, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan anchor: %w", err)
		}
		key := source + "\x00" + session
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, LedgerAnchor{Source: source, SessionID: session, HeadHash: head, Events: count, LastSeenAt: lastSeen})
	}
	return out, rows.Err()
}

type AnchorMismatch struct {
	Source    string
	SessionID string
	Expected  string
	Actual    string
	Truncated bool
}

func (s *Store) VerifyAnchors(ctx context.Context, expected []LedgerAnchor) ([]AnchorMismatch, error) {
	current, err := s.LedgerAnchors(ctx)
	if err != nil {
		return nil, err
	}
	currentMap := make(map[string]LedgerAnchor, len(current))
	for _, a := range current {
		currentMap[a.Source+"\x00"+a.SessionID] = a
	}
	var mismatches []AnchorMismatch
	for _, exp := range expected {
		key := exp.Source + "\x00" + exp.SessionID
		got, ok := currentMap[key]
		if !ok {
			mismatches = append(mismatches, AnchorMismatch{Source: exp.Source, SessionID: exp.SessionID, Expected: exp.HeadHash, Actual: "", Truncated: true})
			continue
		}
		if got.HeadHash != exp.HeadHash {
			mismatches = append(mismatches, AnchorMismatch{Source: exp.Source, SessionID: exp.SessionID, Expected: exp.HeadHash, Actual: got.HeadHash, Truncated: got.Events < exp.Events})
		}
	}
	return mismatches, nil
}

func ValidateAllocations(allocations []AllocationInput) error {
	if len(allocations) == 0 {
		return errors.New("at least one allocation is required")
	}
	var total int64
	for _, allocation := range allocations {
		if allocation.BasisPoints < 0 || allocation.BasisPoints > 10000 {
			return errors.New("allocation basis points must be between 0 and 10000")
		}
		total += allocation.BasisPoints
	}
	if total != 10000 {
		return fmt.Errorf("allocation basis points total %d, want 10000", total)
	}
	return nil
}

func (s *Store) AddProjectTag(ctx context.Context, projectID, key, value string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return errors.New("project id, tag key, and tag value are required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO project_tags (id, project_id, tag_key, tag_value, created_at) VALUES (?, ?, ?, ?, ?)`, newID(), projectID, strings.ToLower(strings.TrimSpace(key)), strings.ToLower(strings.TrimSpace(value)), timestamp(time.Now()))
	if err != nil {
		return fmt.Errorf("add project tag: %w", err)
	}
	return nil
}

func (s *Store) StartTask(ctx context.Context, input TaskInput) (string, error) {
	if strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.TaskType) == "" {
		return "", errors.New("task project, title, and type are required")
	}
	id := newID()
	now := timestamp(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks (id, primary_project_id, title, task_type, status, started_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', ?, ?, ?)`, id, input.ProjectID, input.Title, input.TaskType, now, now, now)
	if err != nil {
		return "", fmt.Errorf("start task: %w", err)
	}
	return id, nil
}

func (s *Store) FinishTask(ctx context.Context, id, result string) error {
	var startedAt string
	if err := s.db.QueryRowContext(ctx, `SELECT started_at FROM tasks WHERE id = ? AND status = 'active'`, id).Scan(&startedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("active task %q not found", id)
		}
		return fmt.Errorf("read task: %w", err)
	}
	now := time.Now().UTC()
	duration := now.Sub(parseTimestamp(startedAt)).Milliseconds()
	if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = 'finished', result = ?, finished_at = ?, duration_ms = ?, updated_at = ? WHERE id = ?`, result, timestamp(now), duration, timestamp(now), id); err != nil {
		return fmt.Errorf("finish task: %w", err)
	}
	return nil
}

func (s *Store) TaskSummary(ctx context.Context, id string) (TaskSummary, error) {
	var summary TaskSummary
	var startedAt string
	var finishedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT t.id, COALESCE(p.slug, ''), t.title, t.task_type, t.status, t.result, t.started_at, t.finished_at,
		(SELECT COUNT(*) FROM model_calls c WHERE c.task_id = t.id),
		(SELECT COALESCE(SUM(c.total_tokens), 0) FROM model_calls c WHERE c.task_id = t.id),
		(SELECT COALESCE(SUM(c.estimated_cost_usd_micros * a.allocation_basis_points / 10000), 0) FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id WHERE c.task_id = t.id)
		FROM tasks t LEFT JOIN projects p ON p.id = t.primary_project_id WHERE t.id = ?`, id).Scan(
		&summary.ID, &summary.ProjectSlug, &summary.Title, &summary.TaskType, &summary.Status, &summary.Result, &startedAt, &finishedAt,
		&summary.ModelCallCount, &summary.ObservedTokens, &summary.AllocatedCostUSDMicros,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskSummary{}, fmt.Errorf("task %q not found", id)
	}
	if err != nil {
		return TaskSummary{}, fmt.Errorf("read task summary: %w", err)
	}
	summary.StartedAt = parseTimestamp(startedAt)
	if finishedAt.Valid {
		value := parseTimestamp(finishedAt.String)
		summary.FinishedAt = &value
	}
	summary.Measurements, err = s.modelMeasurements(ctx, "task_id", id)
	if err != nil {
		return TaskSummary{}, err
	}
	return summary, nil
}

func (s *Store) ListTasks(ctx context.Context, projectSlug string) ([]TaskRecord, error) {
	query := `SELECT t.id, p.slug, t.title, t.task_type, t.status, t.result, t.started_at, t.finished_at FROM tasks t LEFT JOIN projects p ON p.id = t.primary_project_id`
	args := []any{}
	if projectSlug != "" {
		query += ` WHERE p.slug = ?`
		args = append(args, normalizeSlug(projectSlug))
	}
	query += ` ORDER BY t.started_at DESC, t.id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	tasks := make([]TaskRecord, 0)
	for rows.Next() {
		var task TaskRecord
		var slug sql.NullString
		var startedAt string
		var finishedAt sql.NullString
		if err := rows.Scan(&task.ID, &slug, &task.Title, &task.TaskType, &task.Status, &task.Result, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		task.ProjectSlug = slug.String
		task.StartedAt = parseTimestamp(startedAt)
		if finishedAt.Valid {
			value := parseTimestamp(finishedAt.String)
			task.FinishedAt = &value
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.name, p.created_at,
		(SELECT COUNT(*) FROM project_locations l WHERE l.project_id = p.id),
		(SELECT COUNT(*) FROM project_tags t WHERE t.project_id = p.id)
		FROM projects p ORDER BY p.slug`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	projects := make([]ProjectSummary, 0)
	for rows.Next() {
		var project ProjectSummary
		var createdAt string
		if err := rows.Scan(&project.ID, &project.Slug, &project.Name, &createdAt, &project.LocationCount, &project.TagCount); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		project.CreatedAt = parseTimestamp(createdAt)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) ProjectTags(ctx context.Context, projectID string) ([]ProjectTag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag_key, tag_value FROM project_tags WHERE project_id = ? ORDER BY tag_key, tag_value`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project tags: %w", err)
	}
	defer func() { _ = rows.Close() }()
	tags := make([]ProjectTag, 0)
	for rows.Next() {
		var tag ProjectTag
		if err := rows.Scan(&tag.Key, &tag.Value); err != nil {
			return nil, fmt.Errorf("scan project tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) ProjectReport(ctx context.Context, slug string, now time.Time) (ProjectReport, error) {
	project, _, found, err := s.ProjectBySlug(ctx, slug)
	if err != nil {
		return ProjectReport{}, err
	}
	if !found {
		return ProjectReport{}, fmt.Errorf("project %q not found", slug)
	}
	projects, err := s.ListProjects(ctx)
	if err != nil {
		return ProjectReport{}, err
	}
	report := ProjectReport{Tags: make([]ProjectTag, 0), BudgetAlerts: make([]BudgetAlert, 0), Measurements: make([]MeasurementSummary, 0)}
	for _, candidate := range projects {
		if candidate.ID == project.ID {
			report.Project = candidate
			break
		}
	}
	report.Tags, err = s.ProjectTags(ctx, project.ID)
	if err != nil {
		return ProjectReport{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM tasks WHERE primary_project_id = ? AND status = 'active'),
		(SELECT COUNT(*) FROM model_calls WHERE primary_project_id = ?),
		(SELECT COALESCE(SUM(total_tokens), 0) FROM model_calls WHERE primary_project_id = ?),
		(SELECT COALESCE(SUM(c.estimated_cost_usd_micros * a.allocation_basis_points / 10000), 0) FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id WHERE a.project_id = ?)`, project.ID, project.ID, project.ID, project.ID).Scan(
		&report.ActiveTaskCount, &report.ObservedModelCallCount, &report.ObservedTokens, &report.AllocatedCostUSDMicros,
	); err != nil {
		return ProjectReport{}, fmt.Errorf("read project report: %w", err)
	}
	alerts, err := s.BudgetAlerts(ctx, now)
	if err != nil {
		return ProjectReport{}, err
	}
	tagTargets := make(map[string]struct{}, len(report.Tags))
	for _, tag := range report.Tags {
		tagTargets[tag.Key+"="+tag.Value] = struct{}{}
	}
	for _, alert := range alerts {
		_, matchesTag := tagTargets[alert.Target]
		if (alert.Scope == "project" && alert.Target == project.ID) || (alert.Scope == "tag" && matchesTag) {
			report.BudgetAlerts = append(report.BudgetAlerts, alert)
		}
	}
	report.Measurements, err = s.measurementsWithLifecycle(ctx, "primary_project_id", project.ID, "project_id", project.ID)
	if err != nil {
		return ProjectReport{}, err
	}
	return report, nil
}

// UnattributedSummary intentionally reports calls without allocations. A manual
// repair or split removes a call from this queue without rewriting raw usage.
func (s *Store) UnattributedSummary(ctx context.Context) (UnattributedSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.started_at, c.provider, c.model_id, c.total_tokens, c.estimated_cost_usd_micros
		FROM model_calls c WHERE NOT EXISTS (SELECT 1 FROM usage_allocations a WHERE a.subject_type = 'model_call' AND a.subject_id = c.id)
		ORDER BY c.started_at, c.id`)
	if err != nil {
		return UnattributedSummary{}, fmt.Errorf("list unattributed model calls: %w", err)
	}
	defer func() { _ = rows.Close() }()
	summary := UnattributedSummary{ModelCalls: make([]UnattributedModelCall, 0)}
	for rows.Next() {
		var call UnattributedModelCall
		var occurredAt string
		if err := rows.Scan(&call.ID, &occurredAt, &call.Provider, &call.Model, &call.TotalTokens, &call.EstimatedCostUSDMicros); err != nil {
			return UnattributedSummary{}, fmt.Errorf("scan unattributed model call: %w", err)
		}
		call.OccurredAt = parseTimestamp(occurredAt)
		summary.ModelCallCount++
		summary.ObservedTokens += call.TotalTokens
		summary.EstimatedCostUSDMicros += call.EstimatedCostUSDMicros
		summary.ModelCalls = append(summary.ModelCalls, call)
	}
	return summary, rows.Err()
}

func (s *Store) SetBudget(ctx context.Context, input BudgetInput) (BudgetRecord, error) {
	input.Scope = strings.ToLower(strings.TrimSpace(input.Scope))
	input.Target = strings.ToLower(strings.TrimSpace(input.Target))
	if input.Scope != "project" && input.Scope != "tag" {
		return BudgetRecord{}, errors.New("budget scope must be project or tag")
	}
	if input.Scope == "tag" {
		parts := strings.SplitN(input.Target, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return BudgetRecord{}, errors.New("tag budget target must use key=value")
		}
		input.Target = strings.ToLower(strings.TrimSpace(parts[0])) + "=" + strings.ToLower(strings.TrimSpace(parts[1]))
	}
	if input.Target == "" || input.MonthlyCostUSDMicros <= 0 {
		return BudgetRecord{}, errors.New("budget target and positive monthly cost are required")
	}
	if input.AlertPercent == 0 {
		input.AlertPercent = 80
	}
	if input.AlertPercent < 1 || input.AlertPercent > 100 {
		return BudgetRecord{}, errors.New("budget alert percent must be between 1 and 100")
	}
	record := BudgetRecord{ID: newID(), Scope: input.Scope, Target: input.Target, MonthlyCostUSDMicros: input.MonthlyCostUSDMicros, AlertPercent: input.AlertPercent}
	now := timestamp(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO budgets (id, scope, target, monthly_cost_usd_micros, alert_percent, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, target) DO UPDATE SET monthly_cost_usd_micros = excluded.monthly_cost_usd_micros, alert_percent = excluded.alert_percent, updated_at = excluded.updated_at`, record.ID, record.Scope, record.Target, record.MonthlyCostUSDMicros, record.AlertPercent, now, now)
	if err != nil {
		return BudgetRecord{}, fmt.Errorf("set budget: %w", err)
	}
	return record, nil
}

func (s *Store) BudgetAlerts(ctx context.Context, now time.Time) ([]BudgetAlert, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	monthStart := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := monthStart.AddDate(0, 1, 0)
	rows, err := s.db.QueryContext(ctx, `SELECT id, scope, target, monthly_cost_usd_micros, alert_percent FROM budgets ORDER BY scope, target`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	budgets := make([]BudgetAlert, 0)
	for rows.Next() {
		var alert BudgetAlert
		if err := rows.Scan(&alert.ID, &alert.Scope, &alert.Target, &alert.MonthlyCostUSDMicros, &alert.AlertPercent); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		budgets = append(budgets, alert)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	alerts := make([]BudgetAlert, 0, len(budgets))
	for _, alert := range budgets {
		var query string
		args := []any{timestamp(monthStart), timestamp(nextMonth)}
		if alert.Scope == "project" {
			query = `SELECT COALESCE(SUM(c.estimated_cost_usd_micros * a.allocation_basis_points / 10000), 0) FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id WHERE c.started_at >= ? AND c.started_at < ? AND a.project_id = ?`
			args = append(args, alert.Target)
		} else {
			parts := strings.SplitN(alert.Target, "=", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("stored tag budget target %q is invalid", alert.Target)
			}
			query = `SELECT COALESCE(SUM(c.estimated_cost_usd_micros * a.allocation_basis_points / 10000), 0) FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id JOIN project_tags t ON t.project_id = a.project_id WHERE c.started_at >= ? AND c.started_at < ? AND t.tag_key = ? AND t.tag_value = ?`
			args = append(args, parts[0], parts[1])
		}
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&alert.AllocatedCostUSDMicros); err != nil {
			return nil, fmt.Errorf("calculate budget usage: %w", err)
		}
		alert.ThresholdUSDMicros = alert.MonthlyCostUSDMicros * alert.AlertPercent / 100
		switch {
		case alert.AllocatedCostUSDMicros >= alert.MonthlyCostUSDMicros:
			alert.Alert = "exceeded"
		case alert.AllocatedCostUSDMicros >= alert.ThresholdUSDMicros:
			alert.Alert = "warning"
		default:
			alert.Alert = "ok"
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) RecordModelCall(ctx context.Context, input ModelCallInput) (string, error) {
	if strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.ModelID) == "" {
		return "", errors.New("model call provider and model id are required")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if input.CaptureQuality == "" {
		input.CaptureQuality = "unknown"
	}
	id := newID()
	createdAt := time.Now().UTC()
	now := timestamp(createdAt)
	total := input.InputTokens + input.OutputTokens + input.ReasoningTokens + input.CachedInputTokens + input.CacheWriteTokens
	for _, metric := range input.Metrics {
		if metric.Name == "total_tokens" && metric.Value != nil {
			total = *metric.Value
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `INSERT INTO model_calls (id, raw_event_id, interaction_id, interaction_upstream_id, primary_project_id, project_location_id, work_context_id, task_id, session_id, turn_id, started_at, finished_at, duration_ms, agent_name, provider, model_id, input_tokens, output_tokens, reasoning_tokens, cached_input_tokens, cache_write_tokens, total_tokens, estimated_cost_usd_micros, estimated_cost_eur_micros, capture_quality, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, nullable(input.RawEventID), nullable(input.InteractionID), input.InteractionUpstreamID, nullable(input.ProjectID), nullable(input.ProjectLocationID), nullable(input.WorkContextID), nullable(input.TaskID), nullable(input.SessionID), nullable(input.TurnID), timestamp(input.OccurredAt), nullableTime(input.CompletedAt), durationMilliseconds(input.DurationMS), input.AgentName, input.Provider, input.ModelID, input.InputTokens, input.OutputTokens, input.ReasoningTokens, input.CachedInputTokens, input.CacheWriteTokens, total, input.EstimatedCostUSDMicros, input.EstimatedCostEURMicros, input.CaptureQuality, now)
	if err != nil {
		if input.RawEventID != "" && isUniqueConstraint(err) {
			var existingID string
			if lookupErr := tx.QueryRowContext(ctx, `SELECT id FROM model_calls WHERE raw_event_id = ?`, input.RawEventID).Scan(&existingID); lookupErr == nil {
				return existingID, tx.Commit()
			}
		}
		return "", fmt.Errorf("insert model call: %w", err)
	}
	for _, metric := range input.Metrics {
		if metric.Value == nil || !validMetricName(metric.Name) || strings.TrimSpace(metric.Source) == "" || strings.TrimSpace(metric.RawKey) == "" || strings.TrimSpace(metric.Confidence) == "" {
			return "", errors.New("metric observations require a supported name, value, source, raw key, and confidence")
		}
		if *metric.Value < 0 {
			return "", errors.New("metric observation value must not be negative")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO model_call_metrics (model_call_id, metric_name, metric_value, source, raw_key, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, metric.Name, *metric.Value, metric.Source, metric.RawKey, metric.Confidence, now); err != nil {
			return "", fmt.Errorf("insert metric observation: %w", err)
		}
	}
	if input.ProjectID != "" {
		allocationID := newID()
		_, err = tx.ExecContext(ctx, `INSERT INTO usage_allocations (id, subject_type, subject_id, project_id, allocation_basis_points, allocation_method, confidence, created_at) VALUES (?, 'model_call', ?, ?, 10000, 'direct', 'high', ?)`, allocationID, id, input.ProjectID, now)
		if err != nil {
			return "", fmt.Errorf("insert direct allocation: %w", err)
		}
		// The initial direct allocation and its immutable history entry are one
		// transaction. This keeps every model call auditable from creation.
		direct := AllocationRevision{ID: newID(), SubjectType: "model_call", SubjectID: id, RevisionNumber: 1, IdempotencyKey: "direct:" + id, Author: "system", Source: "record_model_call", Reason: "initial direct allocation", CreatedAt: createdAt}
		direct.RevisionHash = allocationRevisionHash(direct, []AllocationInput{{ProjectID: input.ProjectID, BasisPoints: 10000, Method: "direct", Confidence: "high"}}, "")
		if _, err := tx.ExecContext(ctx, `INSERT INTO allocation_revisions (revision_id, entry_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, project_id, allocation_basis_points, allocation_method, confidence, author, source, reason, created_at, previous_revision_hash, revision_hash) VALUES (?, ?, 'model_call', ?, 1, '', ?, ?, 10000, 'direct', 'high', 'system', 'record_model_call', 'initial direct allocation', ?, '', ?)`, direct.ID, newID(), id, direct.IdempotencyKey, input.ProjectID, now, direct.RevisionHash); err != nil {
			return "", fmt.Errorf("insert direct allocation revision: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// ReconcileOTLPUsage excludes an ancestor call from consumption totals only
// when a descendant in the same persisted trace reports exactly the same
// source-native dimensions. Raw events preserve the full span graph, so this
// also works when exporters flush parent and child in separate requests.
func (s *Store) ReconcileOTLPUsage(ctx context.Context, rawEventID string) error {
	if rawEventID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `WITH RECURSIVE ancestry(ancestor_event_id, descendant_event_id) AS (
		SELECT parent.id, child.id
		FROM raw_events parent
		JOIN raw_events child ON child.trace_id = parent.trace_id AND child.parent_span_id = parent.span_id
		WHERE parent.trace_id <> '' AND parent.span_id <> ''
		UNION
		SELECT ancestry.ancestor_event_id, child.id
		FROM ancestry
		JOIN raw_events previous ON previous.id = ancestry.descendant_event_id
		JOIN raw_events child ON child.trace_id = previous.trace_id AND child.parent_span_id = previous.span_id
	)
	UPDATE model_calls
	SET usage_reconciled = 1
	WHERE raw_event_id IN (
		SELECT parent_call.raw_event_id
		FROM ancestry
		JOIN model_calls parent_call ON parent_call.raw_event_id = ancestry.ancestor_event_id
		JOIN model_calls child_call ON child_call.raw_event_id = ancestry.descendant_event_id
		WHERE ancestry.ancestor_event_id <> ancestry.descendant_event_id
			AND NOT EXISTS (
				SELECT 1 FROM ancestry reverse
				WHERE reverse.ancestor_event_id = ancestry.descendant_event_id
					AND reverse.descendant_event_id = ancestry.ancestor_event_id
			)
			AND parent_call.provider = child_call.provider
			AND parent_call.model_id = child_call.model_id
			AND parent_call.input_tokens = child_call.input_tokens
			AND parent_call.output_tokens = child_call.output_tokens
			AND parent_call.reasoning_tokens = child_call.reasoning_tokens
			AND parent_call.cached_input_tokens = child_call.cached_input_tokens
			AND parent_call.cache_write_tokens = child_call.cache_write_tokens
			AND parent_call.total_tokens = child_call.total_tokens
	)`)
	if err != nil {
		return fmt.Errorf("reconcile OTLP aggregate usage: %w", err)
	}
	return nil
}

func validMetricName(name string) bool {
	switch name {
	case "input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "total_tokens":
		return true
	default:
		return false
	}
}

func (s *Store) LinkMatchingLegacyModelCall(ctx context.Context, input ModelCallInput) (bool, error) {
	if strings.TrimSpace(input.RawEventID) == "" || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.ModelID) == "" {
		return false, nil
	}
	if input.OccurredAt.IsZero() {
		return false, nil
	}
	if input.CaptureQuality == "" {
		input.CaptureQuality = "unknown"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin legacy model call linkage: %w", err)
	}
	defer rollback(tx)
	var linkedID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM model_calls WHERE raw_event_id = ?`, input.RawEventID).Scan(&linkedID)
	if err == nil {
		return false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read raw event model call: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM model_calls
		WHERE raw_event_id IS NULL
			AND COALESCE(primary_project_id, '') = ? AND COALESCE(project_location_id, '') = ? AND COALESCE(work_context_id, '') = ?
			AND COALESCE(task_id, '') = ? AND COALESCE(session_id, '') = ? AND COALESCE(turn_id, '') = ?
			AND started_at = ? AND agent_name = ? AND provider = ? AND model_id = ?
			AND input_tokens = ? AND output_tokens = ? AND reasoning_tokens = ? AND cached_input_tokens = ? AND cache_write_tokens = ?
			AND estimated_cost_usd_micros = ? AND estimated_cost_eur_micros = ? AND capture_quality = ?
		LIMIT 2`, input.ProjectID, input.ProjectLocationID, input.WorkContextID, input.TaskID, input.SessionID, input.TurnID,
		timestamp(input.OccurredAt), input.AgentName, input.Provider, input.ModelID,
		input.InputTokens, input.OutputTokens, input.ReasoningTokens, input.CachedInputTokens, input.CacheWriteTokens,
		input.EstimatedCostUSDMicros, input.EstimatedCostEURMicros, input.CaptureQuality)
	if err != nil {
		return false, fmt.Errorf("find matching legacy model call: %w", err)
	}
	defer func() { _ = rows.Close() }()
	candidates := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return false, fmt.Errorf("scan matching legacy model call: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read matching legacy model call: %w", err)
	}
	if len(candidates) != 1 {
		return false, tx.Commit()
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_calls SET raw_event_id = ?, interaction_id = COALESCE(interaction_id, ?), interaction_upstream_id = CASE WHEN interaction_upstream_id = '' THEN ? ELSE interaction_upstream_id END WHERE id = ? AND raw_event_id IS NULL`, input.RawEventID, nullable(input.InteractionID), input.InteractionUpstreamID, candidates[0])
	if err != nil {
		return false, fmt.Errorf("link legacy model call: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("confirm legacy model call linkage: %w", err)
	}
	if affected == 1 {
		for _, metric := range input.Metrics {
			if metric.Value == nil || !validMetricName(metric.Name) || strings.TrimSpace(metric.Source) == "" || strings.TrimSpace(metric.RawKey) == "" || strings.TrimSpace(metric.Confidence) == "" || *metric.Value < 0 {
				return false, errors.New("metric observations require a supported non-negative value, source, raw key, and confidence")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO model_call_metrics (model_call_id, metric_name, metric_value, source, raw_key, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, candidates[0], metric.Name, *metric.Value, metric.Source, metric.RawKey, metric.Confidence, timestamp(time.Now())); err != nil {
				return false, fmt.Errorf("insert linked metric observation: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit legacy model call linkage: %w", err)
	}
	return affected == 1, nil
}

// UpdateLinkedModelCallTiming enriches a pre-upgrade call after its envelope
// identity has been matched. The caller must validate finish >= start.
func (s *Store) UpdateLinkedModelCallTiming(ctx context.Context, rawEventID string, startedAt, finishedAt time.Time) error {
	if rawEventID == "" || startedAt.IsZero() {
		return nil
	}
	if finishedAt.IsZero() {
		_, err := s.db.ExecContext(ctx, `UPDATE model_calls SET started_at = ? WHERE raw_event_id = ?`, timestamp(startedAt), rawEventID)
		if err != nil {
			return fmt.Errorf("update linked model call start: %w", err)
		}
		return nil
	}
	if finishedAt.Before(startedAt) {
		return nil
	}
	duration := finishedAt.Sub(startedAt).Milliseconds()
	_, err := s.db.ExecContext(ctx, `UPDATE model_calls SET started_at = ?, finished_at = ?, duration_ms = ? WHERE raw_event_id = ?`, timestamp(startedAt), timestamp(finishedAt), duration, rawEventID)
	if err != nil {
		return fmt.Errorf("update linked model call timing: %w", err)
	}
	return nil
}

func (s *Store) EnsureSession(ctx context.Context, id, agentName string, startedAt time.Time) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO sessions (id, agent_name, started_at, created_at) VALUES (?, ?, ?, ?)`, id, agentName, timestamp(startedAt), timestamp(time.Now()))
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}
	return nil
}

func (s *Store) ReplaceAllocations(ctx context.Context, subjectType, subjectID string, allocations []AllocationInput) error {
	return s.ReplaceAllocationsWithKey(ctx, subjectType, subjectID, allocations, newID())
}

// ReplaceAllocationsWithKey makes split retries idempotent at the public
// boundary while preserving the compatibility wrapper above.
func (s *Store) ReplaceAllocationsWithKey(ctx context.Context, subjectType, subjectID string, allocations []AllocationInput, idempotencyKey string) error {
	_, err := s.ReplaceAllocationsWithKeyRevision(ctx, subjectType, subjectID, allocations, idempotencyKey)
	return err
}

func (s *Store) ReplaceAllocationsWithKeyRevision(ctx context.Context, subjectType, subjectID string, allocations []AllocationInput, idempotencyKey string) (AllocationRevision, error) {
	return s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: subjectType, SubjectID: subjectID, Allocations: allocations, IdempotencyKey: idempotencyKey, Source: "split", Reason: "replace allocation", Method: "split"})
}

func (s *Store) RepairModelCallAllocation(ctx context.Context, modelCallID, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("repair project id is required")
	}
	_, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: modelCallID, Allocations: []AllocationInput{{ProjectID: projectID, BasisPoints: 10000}}, IdempotencyKey: newID(), Source: "manual", Reason: "repair allocation", Method: "manual"})
	return err
}

func (s *Store) AssignUnattributedModelCall(ctx context.Context, modelCallID, projectID string) error {
	if strings.TrimSpace(modelCallID) == "" || strings.TrimSpace(projectID) == "" {
		return errors.New("model call id and project id are required")
	}
	// The subject-scoped key makes concurrent retries converge on one
	// assignment instead of creating two independent revisions.
	_, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: modelCallID, Allocations: []AllocationInput{{ProjectID: projectID, BasisPoints: 10000}}, IdempotencyKey: "assign:" + modelCallID, Source: "manual", Reason: "assign unattributed allocation", Method: "manual", RequireUnallocated: true})
	return err
}

// AppendAllocationRevision records an allocation decision and atomically
// refreshes the current projection. Historical revision rows are never edited.
func (s *Store) AppendAllocationRevision(ctx context.Context, input AllocationRevisionInput) (AllocationRevision, error) {
	if input.SubjectType != "model_call" || strings.TrimSpace(input.SubjectID) == "" {
		return AllocationRevision{}, errors.New("only model_call allocations with a subject id are supported")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return AllocationRevision{}, errors.New("allocation idempotency key is required")
	}
	if err := ValidateAllocations(input.Allocations); err != nil {
		return AllocationRevision{}, err
	}
	if input.Method == "" {
		input.Method = "manual"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AllocationRevision{}, err
	}
	defer rollback(tx)
	var existing AllocationRevision
	if err := readAllocationRevision(ctx, tx, input.SubjectType, input.SubjectID, input.IdempotencyKey, &existing); err == nil {
		if input.RequireUnallocated && !sameAllocationInputs(existing.Allocations, input.Allocations) {
			return AllocationRevision{}, errors.New("allocation idempotency key already belongs to a different assignment")
		}
		return existing, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return AllocationRevision{}, err
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM model_calls WHERE id = ?`, input.SubjectID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AllocationRevision{}, fmt.Errorf("model call %q not found", input.SubjectID)
		}
		return AllocationRevision{}, fmt.Errorf("read model call allocation: %w", err)
	}
	if input.RequireUnallocated {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_allocations WHERE subject_type = 'model_call' AND subject_id = ?`, input.SubjectID).Scan(&count); err != nil {
			return AllocationRevision{}, fmt.Errorf("read model call allocations: %w", err)
		}
		if count > 0 {
			return AllocationRevision{}, fmt.Errorf("model call %q already has allocations; use a split to replace them", input.SubjectID)
		}
	}
	var parentID, previousHash string
	var nextNumber int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_number), 0), COALESCE((SELECT revision_id FROM allocation_revisions WHERE subject_type = ? AND subject_id = ? ORDER BY revision_number DESC, entry_id DESC LIMIT 1), ''), COALESCE((SELECT revision_hash FROM allocation_revisions WHERE subject_type = ? AND subject_id = ? ORDER BY revision_number DESC, entry_id DESC LIMIT 1), '') FROM allocation_revisions WHERE subject_type = ? AND subject_id = ?`, input.SubjectType, input.SubjectID, input.SubjectType, input.SubjectID, input.SubjectType, input.SubjectID).Scan(&nextNumber, &parentID, &previousHash); err != nil {
		return AllocationRevision{}, err
	}
	nextNumber++
	revision := AllocationRevision{ID: newID(), SubjectType: input.SubjectType, SubjectID: input.SubjectID, RevisionNumber: nextNumber, ParentRevisionID: parentID, IdempotencyKey: input.IdempotencyKey, Author: input.Author, Source: input.Source, Reason: input.Reason, CreatedAt: time.Now().UTC(), Allocations: make([]Allocation, 0, len(input.Allocations))}
	revision.PreviousHash = previousHash
	hashInputs := append([]AllocationInput(nil), input.Allocations...)
	for i := range hashInputs {
		hashInputs[i].Method, hashInputs[i].Confidence = input.Method, "high"
	}
	revision.RevisionHash = allocationRevisionHash(revision, hashInputs, previousHash)
	for _, a := range input.Allocations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO allocation_revisions (revision_id, entry_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, project_id, allocation_basis_points, allocation_method, confidence, author, source, reason, created_at, previous_revision_hash, revision_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, revision.ID, newID(), revision.SubjectType, revision.SubjectID, revision.RevisionNumber, revision.ParentRevisionID, revision.IdempotencyKey, a.ProjectID, a.BasisPoints, input.Method, "high", revision.Author, revision.Source, revision.Reason, timestamp(revision.CreatedAt), revision.PreviousHash, revision.RevisionHash); err != nil {
			return AllocationRevision{}, fmt.Errorf("append allocation revision: %w", err)
		}
		revision.Allocations = append(revision.Allocations, Allocation{ProjectID: a.ProjectID, BasisPoints: a.BasisPoints, Method: input.Method, Confidence: "high"})
	}
	if err := replaceAllocationProjection(ctx, tx, revision); err != nil {
		return AllocationRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO allocation_revision_heads(subject_type, subject_id, revision_id, revision_hash) VALUES(?,?,?,?) ON CONFLICT(subject_type, subject_id) DO UPDATE SET revision_id=excluded.revision_id, revision_hash=excluded.revision_hash`, revision.SubjectType, revision.SubjectID, revision.ID, revision.RevisionHash); err != nil {
		return AllocationRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AllocationRevision{}, err
	}
	return revision, nil
}

func sameAllocationInputs(existing []Allocation, requested []AllocationInput) bool {
	if len(existing) != len(requested) {
		return false
	}
	left, right := make([]string, 0, len(existing)), make([]string, 0, len(requested))
	for _, a := range existing {
		left = append(left, fmt.Sprintf("%s|%d", a.ProjectID, a.BasisPoints))
	}
	for _, a := range requested {
		right = append(right, fmt.Sprintf("%s|%d", a.ProjectID, a.BasisPoints))
	}
	sort.Strings(left)
	sort.Strings(right)
	return slices.Equal(left, right)
}

// AllocationHistory returns immutable revisions in deterministic order.
func (s *Store) AllocationHistory(ctx context.Context, subjectType, subjectID string) ([]AllocationRevision, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT revision_id, revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at, previous_revision_hash, revision_hash FROM allocation_revisions WHERE subject_type = ? AND subject_id = ? GROUP BY revision_id, revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at, previous_revision_hash, revision_hash ORDER BY revision_number, revision_id`, subjectType, subjectID)
	if err != nil {
		return nil, err
	}
	metadata := make([]AllocationRevision, 0)
	for rows.Next() {
		var r AllocationRevision
		var created string
		if err := rows.Scan(&r.ID, &r.RevisionNumber, &r.ParentRevisionID, &r.IdempotencyKey, &r.Author, &r.Source, &r.Reason, &created, &r.PreviousHash, &r.RevisionHash); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.SubjectType, r.SubjectID = subjectType, subjectID
		r.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		metadata = append(metadata, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]AllocationRevision, 0, len(metadata))
	for _, r := range metadata {
		r.Allocations, err = revisionAllocations(ctx, s.db, r.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// RevertAllocationRevision appends a revision restoring the selected revision's parent.
func (s *Store) RevertAllocationRevision(ctx context.Context, revisionID, idempotencyKey, reason string) (AllocationRevision, error) {
	if strings.TrimSpace(revisionID) == "" || strings.TrimSpace(reason) == "" {
		return AllocationRevision{}, errors.New("revision id and reason are required")
	}
	var subjectType, subjectID, parentID string
	if err := s.db.QueryRowContext(ctx, `SELECT subject_type, subject_id, parent_revision_id FROM allocation_revisions WHERE revision_id = ? LIMIT 1`, revisionID).Scan(&subjectType, &subjectID, &parentID); err != nil {
		return AllocationRevision{}, err
	}
	if parentID == "" {
		return AllocationRevision{}, errors.New("cannot revert the first allocation revision")
	}
	allocations, err := revisionAllocations(ctx, s.db, parentID)
	if err != nil {
		return AllocationRevision{}, err
	}
	inputs := make([]AllocationInput, 0, len(allocations))
	for _, a := range allocations {
		inputs = append(inputs, AllocationInput{ProjectID: a.ProjectID, BasisPoints: a.BasisPoints, Method: a.Method, Confidence: a.Confidence})
	}
	return s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: subjectType, SubjectID: subjectID, Allocations: inputs, IdempotencyKey: idempotencyKey, Source: "revert", Reason: reason, Method: "revert"})
}

// RebuildAllocationProjection reconstructs current state solely from history.
func (s *Store) RebuildAllocationProjection(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	rows, err := tx.QueryContext(ctx, `SELECT revision_id, subject_type, subject_id FROM allocation_revisions r WHERE revision_number = (SELECT MAX(r2.revision_number) FROM allocation_revisions r2 WHERE r2.subject_type = r.subject_type AND r2.subject_id = r.subject_id) GROUP BY subject_type, subject_id`)
	if err != nil {
		return err
	}
	type subject struct{ id, typ, sid string }
	latest := make([]subject, 0)
	for rows.Next() {
		var v subject
		if err := rows.Scan(&v.id, &v.typ, &v.sid); err != nil {
			return err
		}
		latest = append(latest, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_allocations`); err != nil {
		return err
	}
	for _, v := range latest {
		r := AllocationRevision{ID: v.id, SubjectType: v.typ, SubjectID: v.sid}
		if err := loadRevisionTx(ctx, tx, &r); err != nil {
			return err
		}
		if err := insertProjection(ctx, tx, r); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type allocationQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func allocationRevisionHash(revision AllocationRevision, allocations []AllocationInput, previous string) string {
	parts := make([]string, 0, len(allocations))
	for _, a := range allocations {
		parts = append(parts, fmt.Sprintf("%s=%d=%s=%s", a.ProjectID, a.BasisPoints, a.Method, a.Confidence))
	}
	sort.Strings(parts)
	return audit.Hash(revision.SubjectType+"\x00"+revision.SubjectID, strings.Join([]string{revision.ID, strconv.FormatInt(revision.RevisionNumber, 10), revision.ParentRevisionID, revision.IdempotencyKey, revision.Author, revision.Source, revision.Reason, timestamp(revision.CreatedAt), strings.Join(parts, ",")}, "\n"), previous)
}

func revisionAllocations(ctx context.Context, q allocationQueryer, revisionID string) (result []Allocation, err error) {
	rows, err := q.QueryContext(ctx, `SELECT r.project_id, COALESCE(p.slug, 'unattributed'), r.allocation_basis_points, r.allocation_method, r.confidence FROM allocation_revisions r LEFT JOIN projects p ON p.id = r.project_id WHERE r.revision_id = ? ORDER BY r.entry_id`, revisionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	result = make([]Allocation, 0)
	for rows.Next() {
		var a Allocation
		var projectID sql.NullString
		if err := rows.Scan(&projectID, &a.ProjectSlug, &a.BasisPoints, &a.Method, &a.Confidence); err != nil {
			return nil, err
		}
		a.ProjectID = projectID.String
		result = append(result, a)
	}
	return result, rows.Err()
}

func readAllocationRevision(ctx context.Context, q allocationQueryer, subjectType, subjectID, key string, result *AllocationRevision) error {
	var created string
	if err := q.QueryRowContext(ctx, `SELECT revision_id, revision_number, parent_revision_id, author, source, reason, created_at FROM allocation_revisions WHERE subject_type = ? AND subject_id = ? AND idempotency_key = ? LIMIT 1`, subjectType, subjectID, key).Scan(&result.ID, &result.RevisionNumber, &result.ParentRevisionID, &result.Author, &result.Source, &result.Reason, &created); err != nil {
		return err
	}
	result.SubjectType, result.SubjectID, result.IdempotencyKey = subjectType, subjectID, key
	parsed, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return err
	}
	result.CreatedAt = parsed
	result.Allocations, err = revisionAllocations(ctx, q, result.ID)
	if err != nil {
		return err
	}
	return nil
}

func replaceAllocationProjection(ctx context.Context, tx *sql.Tx, revision AllocationRevision) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_allocations WHERE subject_type = ? AND subject_id = ?`, revision.SubjectType, revision.SubjectID); err != nil {
		return fmt.Errorf("replace allocation projection: %w", err)
	}
	return insertProjection(ctx, tx, revision)
}

func loadRevisionTx(ctx context.Context, tx *sql.Tx, revision *AllocationRevision) error {
	var err error
	var created string
	if err = tx.QueryRowContext(ctx, `SELECT revision_number, parent_revision_id, idempotency_key, author, source, reason, created_at FROM allocation_revisions WHERE revision_id = ? LIMIT 1`, revision.ID).Scan(&revision.RevisionNumber, &revision.ParentRevisionID, &revision.IdempotencyKey, &revision.Author, &revision.Source, &revision.Reason, &created); err != nil {
		return err
	}
	revision.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("parse allocation revision created_at: %w", err)
	}
	revision.Allocations, err = revisionAllocations(ctx, tx, revision.ID)
	return err
}

func insertProjection(ctx context.Context, tx *sql.Tx, revision AllocationRevision) error {
	for _, a := range revision.Allocations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_allocations (id, subject_type, subject_id, project_id, allocation_basis_points, allocation_method, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, newID(), revision.SubjectType, revision.SubjectID, nullable(a.ProjectID), a.BasisPoints, a.Method, a.Confidence, timestamp(revision.CreatedAt)); err != nil {
			return fmt.Errorf("insert allocation projection: %w", err)
		}
	}
	return nil
}

func (s *Store) ModelCallAllocations(ctx context.Context, modelCallID string) ([]Allocation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.project_id, COALESCE(p.slug, 'unattributed'), a.allocation_basis_points, a.allocation_method, a.confidence
		FROM usage_allocations a LEFT JOIN projects p ON p.id = a.project_id
		WHERE a.subject_type = 'model_call' AND a.subject_id = ? ORDER BY p.slug, a.project_id`, modelCallID)
	if err != nil {
		return nil, fmt.Errorf("list model call allocations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	allocations := make([]Allocation, 0)
	for rows.Next() {
		var allocation Allocation
		if err := rows.Scan(&allocation.ProjectID, &allocation.ProjectSlug, &allocation.BasisPoints, &allocation.Method, &allocation.Confidence); err != nil {
			return nil, fmt.Errorf("scan model call allocation: %w", err)
		}
		allocations = append(allocations, allocation)
	}
	return allocations, rows.Err()
}

func (s *Store) AddPricingRule(ctx context.Context, rule pricing.Rule) (PricingRuleRecord, error) {
	encoded, err := json.Marshal(rule)
	if err != nil {
		return PricingRuleRecord{}, fmt.Errorf("encode pricing rule: %w", err)
	}
	if _, err := pricing.Load(bytes.NewReader(encoded)); err != nil {
		return PricingRuleRecord{}, err
	}
	record := PricingRuleRecord{ID: newID(), Rule: rule, CreatedAt: time.Now().UTC()}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_rules (id, provider, model_pattern, valid_from, valid_until, billing_mode, currency, unit_tokens, rule_json, catalog_version, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, rule.Provider, rule.ModelPattern, timestamp(rule.ValidFrom), nullableTimestamp(rule.ValidUntil), rule.BillingMode, rule.Currency, rule.UnitTokens, string(encoded), rule.Version, timestamp(record.CreatedAt))
	if err != nil {
		return PricingRuleRecord{}, fmt.Errorf("add pricing rule: %w", err)
	}
	return record, nil
}

func (s *Store) ListPricingRules(ctx context.Context) ([]PricingRuleRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, rule_json, created_at FROM pricing_rules ORDER BY provider, model_pattern, valid_from DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list pricing rules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	rules := make([]PricingRuleRecord, 0)
	for rows.Next() {
		var record PricingRuleRecord
		var encoded, createdAt string
		if err := rows.Scan(&record.ID, &encoded, &createdAt); err != nil {
			return nil, fmt.Errorf("scan pricing rule: %w", err)
		}
		rule, err := pricing.Load(bytes.NewReader([]byte(encoded)))
		if err != nil {
			return nil, fmt.Errorf("decode persisted pricing rule %q: %w", record.ID, err)
		}
		record.Rule = rule
		record.CreatedAt = parseTimestamp(createdAt)
		rules = append(rules, record)
	}
	return rules, rows.Err()
}

func (s *Store) RecalculateCosts(ctx context.Context, query PricingRecalculateQuery) (int, error) {
	rules, err := s.ListPricingRules(ctx)
	if err != nil {
		return 0, err
	}
	where, args := modelCallWindow(query)
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, model_id, started_at, input_tokens, cached_input_tokens, cache_write_tokens, output_tokens, reasoning_tokens FROM model_calls`+where+` ORDER BY started_at, id`, args...)
	if err != nil {
		return 0, fmt.Errorf("list model calls for recalculation: %w", err)
	}
	type modelCall struct {
		id        string
		provider  string
		modelID   string
		startedAt time.Time
		usage     pricing.Usage
	}
	calls := make([]modelCall, 0)
	for rows.Next() {
		var call modelCall
		var startedAt string
		if err := rows.Scan(&call.id, &call.provider, &call.modelID, &startedAt, &call.usage.InputTokens, &call.usage.CachedInputTokens, &call.usage.CacheWriteTokens, &call.usage.OutputTokens, &call.usage.ReasoningTokens); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan model call for recalculation: %w", err)
		}
		call.startedAt = parseTimestamp(startedAt)
		calls = append(calls, call)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)
	count := 0
	for _, call := range calls {
		record, found := matchingPricingRule(rules, call.provider, call.modelID, call.startedAt)
		if !found {
			continue
		}
		cost, err := pricing.Calculate(record.Rule, call.usage, call.startedAt)
		if err != nil {
			return 0, fmt.Errorf("calculate model call %q: %w", call.id, err)
		}
		var allocated int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(? * allocation_basis_points / 10000), 0) FROM usage_allocations WHERE subject_type = 'model_call' AND subject_id = ?`, cost, call.id).Scan(&allocated); err != nil {
			return 0, fmt.Errorf("calculate allocated cost: %w", err)
		}
		now := timestamp(time.Now())
		if _, err := tx.ExecContext(ctx, `UPDATE model_calls SET estimated_cost_usd_micros = ? WHERE id = ?`, cost, call.id); err != nil {
			return 0, fmt.Errorf("update model call cost: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cost_snapshots (id, model_call_id, pricing_rule_id, pricing_catalog_version, calculation_formula_version, calculated_at, estimated_cost_usd_micros, allocated_cost_usd_micros, created_at) VALUES (?, ?, ?, ?, 'token-v1', ?, ?, ?, ?)`, newID(), call.id, record.ID, record.Rule.Version, now, cost, allocated, now); err != nil {
			return 0, fmt.Errorf("insert cost snapshot: %w", err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ExportModelCalls(ctx context.Context, query PricingRecalculateQuery) ([]ExportRecord, error) {
	where, args := modelCallWindow(query)
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.started_at, COALESCE(p.slug, 'unattributed'), COALESCE(l.absolute_path, ''), c.provider, c.model_id, c.agent_name, c.input_tokens, c.output_tokens, c.reasoning_tokens, c.cached_input_tokens, c.cache_write_tokens, c.total_tokens, c.estimated_cost_usd_micros, c.capture_quality
		FROM model_calls c LEFT JOIN projects p ON p.id = c.primary_project_id LEFT JOIN project_locations l ON l.id = c.project_location_id`+where+` ORDER BY c.started_at, c.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("export model calls: %w", err)
	}
	records := make([]ExportRecord, 0)
	for rows.Next() {
		var record ExportRecord
		var occurredAt string
		if err := rows.Scan(&record.ID, &occurredAt, &record.ProjectSlug, &record.ProjectLocationPath, &record.Provider, &record.Model, &record.Agent, &record.InputTokens, &record.OutputTokens, &record.ReasoningTokens, &record.CachedInputTokens, &record.CacheWriteTokens, &record.TotalTokens, &record.EstimatedCostUSDMicros, &record.CaptureQuality); err != nil {
			return nil, fmt.Errorf("scan exported model call: %w", err)
		}
		record.OccurredAt = parseTimestamp(occurredAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range records {
		allocations, err := s.ModelCallAllocations(ctx, records[index].ID)
		if err != nil {
			return nil, err
		}
		records[index].Allocations = allocations
	}
	return records, nil
}

func matchingPricingRule(rules []PricingRuleRecord, provider, modelID string, occurredAt time.Time) (PricingRuleRecord, bool) {
	var match PricingRuleRecord
	for _, candidate := range rules {
		if candidate.Rule.Provider != provider || occurredAt.Before(candidate.Rule.ValidFrom) || candidate.Rule.ValidUntil != nil && !occurredAt.Before(*candidate.Rule.ValidUntil) {
			continue
		}
		matched, err := path.Match(candidate.Rule.ModelPattern, modelID)
		if err != nil || !matched {
			continue
		}
		if match.ID == "" || candidate.Rule.ValidFrom.After(match.Rule.ValidFrom) {
			match = candidate
		}
	}
	return match, match.ID != ""
}

func modelCallWindow(query PricingRecalculateQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}
	if !query.From.IsZero() {
		where += " AND started_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND started_at < ?"
		args = append(args, timestamp(query.To))
	}
	return where, args
}

func (s *Store) Usage(ctx context.Context, query UsageQuery) (UsageReport, error) {
	if err := validateGroupBy(query.GroupBy); err != nil {
		return UsageReport{}, err
	}
	var totalTokens int64
	if query.ProjectSlug == "" {
		where, args := usageWindow(query)
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_tokens), 0) FROM model_calls c`+where, args...).Scan(&totalTokens); err != nil {
			return UsageReport{}, err
		}
	}
	allocationQuery := query
	allocationQuery.ProjectSlug = ""
	where, args := usageWindow(allocationQuery)
	// Calls without an allocation are still ledger evidence. Report them once as
	// unattributed (or under their direct project when legacy data has one),
	// rather than silently omitting their tokens from rows.
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, COALESCE(a.id, ''), COALESCE(allocated.slug, direct.slug, 'unattributed'), c.agent_name, c.provider, c.model_id, c.capture_quality, c.input_tokens, c.output_tokens, c.reasoning_tokens, c.cached_input_tokens, c.cache_write_tokens, c.total_tokens, c.estimated_cost_usd_micros, COALESCE(a.allocation_basis_points, 10000) FROM model_calls c LEFT JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id LEFT JOIN projects allocated ON allocated.id = a.project_id LEFT JOIN projects direct ON direct.id = c.primary_project_id`+where+` ORDER BY c.id, a.id`, args...)
	if err != nil {
		return UsageReport{}, err
	}
	defer func() { _ = rows.Close() }()
	grouped := make(map[string]UsageRow)
	allocatedBasis := make(map[string]int64)
	var totalAllocated int64
	for rows.Next() {
		var row UsageRow
		var callID, allocationID string
		var cost, basisPoints int64
		if err := rows.Scan(&callID, &allocationID, &row.ProjectSlug, &row.AgentName, &row.Provider, &row.Model, &row.CaptureQuality, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.TotalTokens, &cost, &basisPoints); err != nil {
			return UsageReport{}, err
		}
		offset := allocatedBasis[callID]
		row.InputTokens = apportionShare(row.InputTokens, offset, basisPoints)
		row.OutputTokens = apportionShare(row.OutputTokens, offset, basisPoints)
		row.ReasoningTokens = apportionShare(row.ReasoningTokens, offset, basisPoints)
		row.CachedInputTokens = apportionShare(row.CachedInputTokens, offset, basisPoints)
		row.CacheWriteTokens = apportionShare(row.CacheWriteTokens, offset, basisPoints)
		row.TotalTokens = apportionShare(row.TotalTokens, offset, basisPoints)
		row.AllocatedCostUSDMicros = apportionShare(cost, offset, basisPoints)
		allocatedBasis[callID] += basisPoints
		if query.ProjectSlug != "" && row.ProjectSlug != normalizeSlug(query.ProjectSlug) {
			continue
		}
		totalAllocated += row.AllocatedCostUSDMicros
		key := row.ProjectSlug + "\x00" + row.AgentName + "\x00" + row.Provider + "\x00" + row.Model + "\x00" + row.CaptureQuality
		if existing, found := grouped[key]; found {
			existing.InputTokens += row.InputTokens
			existing.OutputTokens += row.OutputTokens
			existing.ReasoningTokens += row.ReasoningTokens
			existing.CachedInputTokens += row.CachedInputTokens
			existing.CacheWriteTokens += row.CacheWriteTokens
			existing.TotalTokens += row.TotalTokens
			existing.AllocatedCostUSDMicros += row.AllocatedCostUSDMicros
			grouped[key] = existing
		} else {
			grouped[key] = row
		}
	}
	if err := rows.Err(); err != nil {
		return UsageReport{}, err
	}
	if query.ProjectSlug != "" {
		totalTokens = 0
		for _, row := range grouped {
			totalTokens += row.TotalTokens
		}
	}
	report := UsageReport{GroupBy: append([]string(nil), query.GroupBy...), Rows: make([]UsageRow, 0), Measurements: make([]MeasurementSummary, 0), TotalTokens: totalTokens, AllocatedCostUSDMicros: totalAllocated}
	for _, row := range grouped {
		report.Rows = append(report.Rows, row)
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		left := report.Rows[i].ProjectSlug + report.Rows[i].AgentName + report.Rows[i].Provider + report.Rows[i].Model + report.Rows[i].CaptureQuality
		right := report.Rows[j].ProjectSlug + report.Rows[j].AgentName + report.Rows[j].Provider + report.Rows[j].Model + report.Rows[j].CaptureQuality
		return left < right
	})
	report.Measurements, err = s.usageMeasurements(ctx, query)
	if err != nil {
		return UsageReport{}, err
	}
	return report, nil
}

func (s *Store) CapabilityReport(ctx context.Context, query CapabilityQuery) (CapabilityReport, error) {
	report := CapabilityReport{From: query.From, To: query.To, ProjectSlug: normalizeSlug(query.ProjectSlug), AgentName: query.AgentName, SessionID: query.SessionID, MetricCoverage: make([]MetricCoverage, 0), Sources: make([]SourceCoverage, 0)}
	modelWhere, modelArgs := capabilityModelWhere(query)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(c.total_tokens), 0) FROM model_calls c LEFT JOIN projects p ON p.id = c.primary_project_id`+modelWhere, modelArgs...).Scan(&report.ModelCalls, &report.Tokens); err != nil {
		return CapabilityReport{}, fmt.Errorf("read capability model calls: %w", err)
	}
	interactionWhere, interactionArgs := capabilityInteractionWhere(query)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM interactions i LEFT JOIN projects p ON p.id = i.primary_project_id`+interactionWhere, interactionArgs...).Scan(&report.Interactions); err != nil {
		return CapabilityReport{}, fmt.Errorf("read capability interactions: %w", err)
	}
	report.Prompts = report.Interactions
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(c.total_tokens), 0) FROM model_calls c LEFT JOIN projects p ON p.id = c.primary_project_id`+modelWhere+` AND NOT EXISTS (SELECT 1 FROM usage_allocations a WHERE a.subject_type = 'model_call' AND a.subject_id = c.id)`, modelArgs...).Scan(&report.UnattributedModelCalls, &report.UnattributedTokens); err != nil {
		return CapabilityReport{}, fmt.Errorf("read capability unattributed calls: %w", err)
	}

	rawWhere, rawArgs := capabilityRawWhere(query)
	rows, err := s.db.QueryContext(ctx, `SELECT lower(event_type), COUNT(*) FROM raw_events r LEFT JOIN projects p ON p.id = r.project_id`+rawWhere+` GROUP BY lower(event_type)`, rawArgs...)
	if err != nil {
		return CapabilityReport{}, fmt.Errorf("read capability raw events: %w", err)
	}
	for rows.Next() {
		var eventType string
		var count int64
		if err := rows.Scan(&eventType, &count); err != nil {
			_ = rows.Close()
			return CapabilityReport{}, fmt.Errorf("scan capability raw event: %w", err)
		}
		if isLifecycleEventType(eventType) {
			report.LifecycleEvents += count
		}
		if strings.Contains(eventType, "mcp") {
			report.MCPCalls += count
		}
		if strings.Contains(eventType, "tool") {
			report.ToolCalls += count
		}
		if strings.Contains(eventType, "error") || strings.Contains(eventType, "failed") {
			report.Errors += count
		}
	}
	if err := rows.Close(); err != nil {
		return CapabilityReport{}, err
	}

	if err := s.capabilityMetrics(ctx, modelWhere, modelArgs, report.ModelCalls, &report); err != nil {
		return CapabilityReport{}, err
	}
	if err := s.capabilitySources(ctx, modelWhere, modelArgs, &report); err != nil {
		return CapabilityReport{}, err
	}
	return report, nil
}

func isLifecycleEventType(eventType string) bool {
	switch strings.ToLower(eventType) {
	case "sessionstart", "sessionend", "agentstop", "userpromptsubmit", "subagentstop", "stop", "claudecodehook":
		return true
	}
	return strings.Contains(eventType, "lifecycle") || strings.Contains(eventType, "session") || strings.HasSuffix(eventType, "stop")
}

func capabilityModelWhere(query CapabilityQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}
	if !query.From.IsZero() {
		where += " AND c.started_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND c.started_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.ProjectSlug != "" {
		where += " AND EXISTS (SELECT 1 FROM usage_allocations a JOIN projects allocated ON allocated.id = a.project_id WHERE a.subject_type = 'model_call' AND a.subject_id = c.id AND allocated.slug = ?)"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	if query.AgentName != "" {
		where += " AND c.agent_name = ?"
		args = append(args, query.AgentName)
	}
	if query.SessionID != "" {
		where += " AND c.session_id = ?"
		args = append(args, query.SessionID)
	}
	return where, args
}

func capabilityRawWhere(query CapabilityQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}
	if !query.From.IsZero() {
		where += " AND r.occurred_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND r.occurred_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.ProjectSlug != "" {
		where += " AND p.slug = ?"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	if query.AgentName != "" {
		where += " AND COALESCE(json_extract(r.payload_json_sanitized, '$.agent_name'), '') = ?"
		args = append(args, query.AgentName)
	}
	if query.SessionID != "" {
		where += " AND r.session_id = ?"
		args = append(args, query.SessionID)
	}
	return where, args
}

func capabilityInteractionWhere(query CapabilityQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}
	if !query.From.IsZero() {
		where += " AND i.occurred_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND i.occurred_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.ProjectSlug != "" {
		where += " AND p.slug = ?"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	if query.AgentName != "" {
		where += " AND COALESCE(json_extract((SELECT r.payload_json_sanitized FROM raw_events r WHERE r.id = i.raw_event_id), '$.agent_name'), '') = ?"
		args = append(args, query.AgentName)
	}
	if query.SessionID != "" {
		where += " AND i.session_id = ?"
		args = append(args, query.SessionID)
	}
	return where, args
}

func (s *Store) capabilityMetrics(ctx context.Context, where string, args []any, modelCalls int64, report *CapabilityReport) error {
	type aggregate struct{ reported, total, zero, disagreements int64 }
	values := make(map[string]aggregate)
	provenance := make(map[string][]MetricProvenance)
	rows, err := s.db.QueryContext(ctx, `SELECT m.metric_name, COUNT(*), COALESCE(SUM(m.metric_value), 0), COALESCE(SUM(CASE WHEN m.metric_value = 0 THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE m.metric_name WHEN 'input_tokens' THEN m.metric_value != c.input_tokens WHEN 'output_tokens' THEN m.metric_value != c.output_tokens WHEN 'reasoning_tokens' THEN m.metric_value != c.reasoning_tokens WHEN 'cached_input_tokens' THEN m.metric_value != c.cached_input_tokens WHEN 'cache_write_tokens' THEN m.metric_value != c.cache_write_tokens WHEN 'total_tokens' THEN m.metric_value != c.total_tokens ELSE 0 END), 0), m.source, m.raw_key, m.confidence, COUNT(*) FROM model_call_metrics m JOIN model_calls c ON c.id = m.model_call_id LEFT JOIN projects p ON p.id = c.primary_project_id`+where+` GROUP BY m.metric_name, m.source, m.raw_key, m.confidence ORDER BY m.metric_name, m.source, m.raw_key, m.confidence`, args...)
	if err != nil {
		return fmt.Errorf("read metric coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, source, rawKey, confidence string
		var count, sum, zero, disagreements, provenanceCount int64
		if err := rows.Scan(&name, &count, &sum, &zero, &disagreements, &source, &rawKey, &confidence, &provenanceCount); err != nil {
			return fmt.Errorf("scan metric coverage: %w", err)
		}
		current := values[name]
		current.reported += count
		current.total += sum
		current.zero += zero
		current.disagreements += disagreements
		values[name] = current
		provenance[name] = append(provenance[name], MetricProvenance{Source: source, RawKey: rawKey, Confidence: confidence, Count: provenanceCount})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range []string{"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "total_tokens"} {
		current := values[name]
		coverage := MetricCoverage{Name: name, ReportedCount: current.reported, MissingCount: modelCalls - current.reported, ReportedZeroCount: current.zero, Provenance: provenance[name]}
		switch {
		case current.reported == 0:
			coverage.State = "not_emitted"
		case current.reported != modelCalls || current.disagreements != 0:
			coverage.State = "unreconciled"
		default:
			coverage.State = "reported"
			value := current.total
			coverage.Value = &value
		}
		report.MetricCoverage = append(report.MetricCoverage, coverage)
	}
	return nil
}

func (s *Store) capabilitySources(ctx context.Context, where string, args []any, report *CapabilityReport) error {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(r.source, '—'), c.capture_quality, NULLIF(r.source_version, ''), COUNT(*) FROM model_calls c LEFT JOIN raw_events r ON r.id = c.raw_event_id LEFT JOIN projects p ON p.id = c.primary_project_id`+where+` GROUP BY COALESCE(r.source, '—'), c.capture_quality, NULLIF(r.source_version, '') ORDER BY 1, 2, 3`, args...)
	if err != nil {
		return fmt.Errorf("read capability sources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var source, quality string
		var version sql.NullString
		var calls int64
		if err := rows.Scan(&source, &quality, &version, &calls); err != nil {
			return fmt.Errorf("scan capability source: %w", err)
		}
		item := SourceCoverage{Source: source, Quality: quality, ModelCalls: calls}
		if version.Valid {
			value := version.String
			item.Version = &value
		}
		report.Sources = append(report.Sources, item)
	}
	return rows.Err()
}

func (s *Store) modelMeasurements(ctx context.Context, column, value string) ([]MeasurementSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT capture_quality, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(reasoning_tokens), 0), COALESCE(SUM(cached_input_tokens), 0), COALESCE(SUM(cache_write_tokens), 0), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(estimated_cost_usd_micros), 0) FROM model_calls WHERE `+column+` = ? GROUP BY capture_quality ORDER BY capture_quality`, value)
	if err != nil {
		return nil, fmt.Errorf("read measurements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	measurements := make([]MeasurementSummary, 0)
	for rows.Next() {
		var summary MeasurementSummary
		if err := rows.Scan(&summary.Quality, &summary.ModelCallCount, &summary.InputTokens, &summary.OutputTokens, &summary.ReasoningTokens, &summary.CachedInputTokens, &summary.CacheWriteTokens, &summary.TotalTokens, &summary.EstimatedCostUSDMicros); err != nil {
			return nil, fmt.Errorf("scan measurements: %w", err)
		}
		measurements = append(measurements, summary)
	}
	return measurements, rows.Err()
}

func (s *Store) measurementsWithLifecycle(ctx context.Context, modelColumn, modelValue, rawColumn, rawValue string) ([]MeasurementSummary, error) {
	measurements, err := s.modelMeasurements(ctx, modelColumn, modelValue)
	if err != nil {
		return nil, err
	}
	var lifecycleCount int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM raw_events WHERE `+rawColumn+` = ? AND lower(COALESCE(json_extract(payload_json_sanitized, '$.capture_quality'), '')) = 'lifecycle_only'`, rawValue).Scan(&lifecycleCount); err != nil {
		return nil, fmt.Errorf("read lifecycle evidence: %w", err)
	}
	return addLifecycleMeasurement(measurements, lifecycleCount), nil
}

func addLifecycleMeasurement(measurements []MeasurementSummary, rawEventCount int64) []MeasurementSummary {
	if rawEventCount == 0 {
		return measurements
	}
	for index := range measurements {
		if measurements[index].Quality == "lifecycle_only" {
			measurements[index].RawEventCount += rawEventCount
			return measurements
		}
	}
	measurements = append(measurements, MeasurementSummary{Quality: "lifecycle_only", RawEventCount: rawEventCount})
	sort.Slice(measurements, func(i, j int) bool { return measurements[i].Quality < measurements[j].Quality })
	return measurements
}

func (s *Store) usageMeasurements(ctx context.Context, query UsageQuery) ([]MeasurementSummary, error) {
	allocationQuery := query
	allocationQuery.ProjectSlug = ""
	where, args := usageWindow(allocationQuery)
	measurementQuery := `SELECT c.id, COALESCE(p.slug, 'unattributed'), c.capture_quality, c.input_tokens, c.output_tokens, c.reasoning_tokens, c.cached_input_tokens, c.cache_write_tokens, c.total_tokens, c.estimated_cost_usd_micros, COALESCE(a.allocation_basis_points, 10000) FROM model_calls c LEFT JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id LEFT JOIN projects p ON p.id = a.project_id` + where + ` ORDER BY c.id, a.id`
	rows, err := s.db.QueryContext(ctx, measurementQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("read usage measurements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byQuality := make(map[string]MeasurementSummary)
	allocatedBasis := make(map[string]int64)
	seenCalls := make(map[string]bool)
	for rows.Next() {
		var callID, projectSlug, quality string
		var inputTokens, outputTokens, reasoningTokens, cachedInputTokens, cacheWriteTokens, totalTokens, cost, basisPoints int64
		if err := rows.Scan(&callID, &projectSlug, &quality, &inputTokens, &outputTokens, &reasoningTokens, &cachedInputTokens, &cacheWriteTokens, &totalTokens, &cost, &basisPoints); err != nil {
			return nil, fmt.Errorf("scan usage measurements: %w", err)
		}
		offset := allocatedBasis[callID]
		inputTokens = apportionShare(inputTokens, offset, basisPoints)
		outputTokens = apportionShare(outputTokens, offset, basisPoints)
		reasoningTokens = apportionShare(reasoningTokens, offset, basisPoints)
		cachedInputTokens = apportionShare(cachedInputTokens, offset, basisPoints)
		cacheWriteTokens = apportionShare(cacheWriteTokens, offset, basisPoints)
		totalTokens = apportionShare(totalTokens, offset, basisPoints)
		cost = apportionShare(cost, offset, basisPoints)
		allocatedBasis[callID] += basisPoints
		if query.ProjectSlug != "" && projectSlug != normalizeSlug(query.ProjectSlug) {
			continue
		}
		summary := byQuality[quality]
		summary.Quality = quality
		if !seenCalls[callID] {
			summary.ModelCallCount++
			seenCalls[callID] = true
		}
		summary.InputTokens += inputTokens
		summary.OutputTokens += outputTokens
		summary.ReasoningTokens += reasoningTokens
		summary.CachedInputTokens += cachedInputTokens
		summary.CacheWriteTokens += cacheWriteTokens
		summary.TotalTokens += totalTokens
		summary.EstimatedCostUSDMicros += cost
		byQuality[quality] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	measurements := make([]MeasurementSummary, 0, len(byQuality))
	for _, summary := range byQuality {
		measurements = append(measurements, summary)
	}
	sort.Slice(measurements, func(i, j int) bool { return measurements[i].Quality < measurements[j].Quality })
	where = " WHERE lower(COALESCE(json_extract(r.payload_json_sanitized, '$.capture_quality'), '')) = 'lifecycle_only'"
	args = nil
	if !query.From.IsZero() {
		where += " AND r.occurred_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND r.occurred_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.ProjectSlug != "" {
		where += " AND p.slug = ?"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	var lifecycleCount int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM raw_events r LEFT JOIN projects p ON p.id = r.project_id`+where, args...).Scan(&lifecycleCount); err != nil {
		return nil, fmt.Errorf("read usage lifecycle evidence: %w", err)
	}
	return addLifecycleMeasurement(measurements, lifecycleCount), nil
}

func apportionShare(value, offset, basisPoints int64) int64 {
	return value*(offset+basisPoints)/10000 - value*offset/10000
}

func (s *Store) SessionSnapshot(ctx context.Context, sessionID string) (SessionSnapshot, error) {
	if strings.TrimSpace(sessionID) == "" {
		return SessionSnapshot{}, errors.New("session id is required")
	}
	snapshot := SessionSnapshot{SessionID: sessionID, Measurements: make([]MeasurementSummary, 0), ResolutionMethod: "unresolved", ResolutionConfidence: "unknown"}
	var sessionAgent, sessionStarted sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT agent_name, started_at FROM sessions WHERE id = ?`, sessionID).Scan(&sessionAgent, &sessionStarted); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SessionSnapshot{}, fmt.Errorf("read session: %w", err)
	}
	var rawStarted, rawLast sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(occurred_at), MAX(occurred_at), COALESCE((SELECT json_extract(payload_json_sanitized, '$.agent_name') FROM raw_events WHERE session_id = ? ORDER BY occurred_at DESC, id DESC LIMIT 1), ''), COALESCE((SELECT project_resolution_method FROM raw_events WHERE session_id = ? ORDER BY occurred_at DESC, id DESC LIMIT 1), 'unresolved'), COALESCE((SELECT project_resolution_confidence FROM raw_events WHERE session_id = ? ORDER BY occurred_at DESC, id DESC LIMIT 1), 'unknown') FROM raw_events WHERE session_id = ?`, sessionID, sessionID, sessionID, sessionID).Scan(&snapshot.RawEventCount, &rawStarted, &rawLast, &snapshot.AgentName, &snapshot.ResolutionMethod, &snapshot.ResolutionConfidence); err != nil {
		return SessionSnapshot{}, fmt.Errorf("read session evidence: %w", err)
	}
	var modelStarted, modelLast, modelAgent sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(started_at), MAX(started_at), COALESCE((SELECT agent_name FROM model_calls WHERE session_id = ? ORDER BY started_at DESC, id DESC LIMIT 1), '') FROM model_calls WHERE session_id = ?`, sessionID, sessionID).Scan(&snapshot.ModelCallCount, &modelStarted, &modelLast, &modelAgent); err != nil {
		return SessionSnapshot{}, fmt.Errorf("read session model calls: %w", err)
	}
	if snapshot.RawEventCount == 0 && snapshot.ModelCallCount == 0 && !sessionStarted.Valid {
		return SessionSnapshot{}, fmt.Errorf("session %q not found", sessionID)
	}
	if sessionStarted.Valid {
		snapshot.StartedAt = parseTimestamp(sessionStarted.String)
	} else if rawStarted.Valid {
		snapshot.StartedAt = parseTimestamp(rawStarted.String)
	} else if modelStarted.Valid {
		snapshot.StartedAt = parseTimestamp(modelStarted.String)
	}
	if rawLast.Valid {
		snapshot.LastEventAt = parseTimestamp(rawLast.String)
	}
	if modelLast.Valid && parseTimestamp(modelLast.String).After(snapshot.LastEventAt) {
		snapshot.LastEventAt = parseTimestamp(modelLast.String)
	}
	if sessionAgent.Valid && sessionAgent.String != "" {
		snapshot.AgentName = sessionAgent.String
	} else if snapshot.AgentName == "" && modelAgent.Valid {
		snapshot.AgentName = modelAgent.String
	}
	var err error
	snapshot.Measurements, err = s.measurementsWithLifecycle(ctx, "session_id", sessionID, "session_id", sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	for _, measurement := range snapshot.Measurements {
		if measurement.Quality == "lifecycle_only" {
			snapshot.LifecycleEventCount = measurement.RawEventCount
		}
	}
	return snapshot, nil
}

// SessionSnapshots returns aggregate evidence only. It does not expose raw
// event payloads, which may contain user-controlled metadata.
func (s *Store) SessionSnapshots(ctx context.Context) ([]SessionSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id FROM (
		SELECT id AS session_id FROM sessions
		UNION
		SELECT session_id FROM raw_events WHERE session_id IS NOT NULL AND session_id != ''
		UNION
		SELECT session_id FROM model_calls WHERE session_id IS NOT NULL AND session_id != ''
	) ORDER BY session_id`)
	if err != nil {
		return nil, fmt.Errorf("list session ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	sessionIDs := make([]string, 0)
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close session ids: %w", err)
	}
	snapshots := make([]SessionSnapshot, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		snapshot, err := s.SessionSnapshot(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (s *Store) HasRecentAdapterEvidence(ctx context.Context, query AdapterEvidenceQuery) (bool, error) {
	if strings.TrimSpace(query.AdapterID) == "" || strings.TrimSpace(query.Source) == "" || strings.TrimSpace(query.RequiredQuality) == "" {
		return false, errors.New("adapter id, source, and required quality are required")
	}
	where := ` WHERE r.source = ?
		AND lower(COALESCE(json_extract(r.payload_json_sanitized, '$.capture_quality'), '')) = lower(?)`
	args := []any{query.Source, query.RequiredQuality}
	if len(query.AllowedAgentNames) == 0 {
		where += ` AND lower(COALESCE(json_extract(r.payload_json_sanitized, '$.agent_name'), '')) = lower(?)`
		args = append(args, query.AdapterID)
	} else {
		placeholders := make([]string, 0, len(query.AllowedAgentNames))
		for _, agentName := range query.AllowedAgentNames {
			placeholders = append(placeholders, "lower(?)")
			args = append(args, agentName)
		}
		where += ` AND lower(COALESCE(json_extract(r.payload_json_sanitized, '$.agent_name'), '')) IN (` + strings.Join(placeholders, ", ") + `)`
	}
	if strings.TrimSpace(query.RequiredProvider) != "" {
		where += ` AND lower(COALESCE(json_extract(r.payload_json_sanitized, '$.provider'), '')) = lower(?)`
		args = append(args, query.RequiredProvider)
	}
	if !query.From.IsZero() {
		where += " AND r.occurred_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND r.occurred_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.RequireCodexResponseCompleted {
		where += ` AND json_extract(r.payload_json_sanitized, '$.codex_response_completed') = 1`
	}
	if query.RequiredQuality == "lifecycle_only" {
		if query.ProjectSlug != "" {
			where += " AND p.slug = ?"
			args = append(args, normalizeSlug(query.ProjectSlug))
		}
		var found bool
		err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM raw_events r LEFT JOIN projects p ON p.id = r.project_id`+where+`)`, args...).Scan(&found)
		return found, err
	}

	where += " AND c.capture_quality = ? AND c.total_tokens > 0"
	args = append(args, query.RequiredQuality)
	joins := ""
	if query.ProjectSlug != "" {
		joins = " JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id JOIN projects p ON p.id = a.project_id"
		where += " AND p.slug = ?"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	var found bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM raw_events r
		JOIN model_calls c ON c.raw_event_id = r.id
		`+joins+where+`
		GROUP BY r.id
		HAVING COUNT(DISTINCT c.id) = 1
	)`, args...).Scan(&found)
	return found, err
}

func validateGroupBy(groupBy []string) error {
	if len(groupBy) == 0 {
		return errors.New("at least one group-by dimension is required")
	}
	allowed := map[string]bool{"project": true, "agent": true, "provider": true, "model": true, "capture_quality": true}
	seen := make(map[string]bool)
	for _, dimension := range groupBy {
		if !allowed[dimension] || seen[dimension] {
			return fmt.Errorf("unsupported group-by dimension %q", dimension)
		}
		seen[dimension] = true
	}
	return nil
}

func usageWindow(query UsageQuery) (string, []any) {
	where := " WHERE c.usage_reconciled = 0"
	args := []any{}
	if !query.From.IsZero() {
		where += " AND c.started_at >= ?"
		args = append(args, timestamp(query.From))
	}
	if !query.To.IsZero() {
		where += " AND c.started_at < ?"
		args = append(args, timestamp(query.To))
	}
	if query.ProjectSlug != "" {
		where += " AND p.slug = ?"
		args = append(args, normalizeSlug(query.ProjectSlug))
	}
	return where, args
}

func (s *Store) RegisteredPaths(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.absolute_path, p.slug FROM project_locations l JOIN projects p ON p.id = l.project_id`)
	if err != nil {
		return nil, fmt.Errorf("query registered paths: %w", err)
	}
	defer func() { _ = rows.Close() }()
	paths := make(map[string]string)
	for rows.Next() {
		var path, slug string
		if err := rows.Scan(&path, &slug); err != nil {
			return nil, err
		}
		paths[path] = slug
	}
	return paths, rows.Err()
}

func (s *Store) ProjectBySlug(ctx context.Context, slug string) (domain.Project, domain.ProjectLocation, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, false, err
	}
	defer rollback(tx)
	project, found, err := projectBySlug(ctx, tx, normalizeSlug(slug))
	if err != nil || !found {
		return project, domain.ProjectLocation{}, found, err
	}
	var location domain.ProjectLocation
	var locationCreatedAt string
	row := tx.QueryRowContext(ctx, `SELECT id, project_id, absolute_path, path_hash, created_at FROM project_locations WHERE project_id = ? ORDER BY created_at LIMIT 1`, project.ID)
	if err := row.Scan(&location.ID, &location.ProjectID, &location.AbsolutePath, &location.PathHash, &locationCreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project, location, true, nil
		}
		return domain.Project{}, domain.ProjectLocation{}, false, err
	}
	location.CreatedAt = parseTimestamp(locationCreatedAt)
	return project, location, true, nil
}

func (s *Store) ProjectByLocation(ctx context.Context, path string) (domain.Project, domain.ProjectLocation, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.Project{}, domain.ProjectLocation{}, false, err
	}
	defer rollback(tx)
	return projectByLocation(ctx, tx, path)
}

func (s *Store) Doctor(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("sqlite integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity check failed: %s", result)
	}
	return nil
}

func (s *Store) validateSchema(ctx context.Context) error {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var applied string
		err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&applied)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("database schema has pending migration %s; run qlog init", entry.Name())
		}
		if err != nil {
			return fmt.Errorf("database schema is not initialized; run qlog init first: %w", err)
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var applied string
		err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&applied)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		sqlBytes, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", entry.Name(), timestamp(time.Now())); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	if err := s.backfillAllocationRevisionHashes(ctx); err != nil {
		return err
	}
	if err := s.backfillAllocationRevisionHeads(ctx); err != nil {
		return err
	}
	return s.backfillReconstructableIngestionIdentities(ctx)
}

func (s *Store) backfillAllocationRevisionHeads(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO allocation_revision_heads(subject_type, subject_id, revision_id, revision_hash)
		SELECT r.subject_type, r.subject_id, r.revision_id, r.revision_hash FROM allocation_revisions r
		WHERE r.revision_number=(SELECT MAX(r2.revision_number) FROM allocation_revisions r2 WHERE r2.subject_type=r.subject_type AND r2.subject_id=r.subject_id)`)
	return err
}

// backfillAllocationRevisionHashes gives migration-created revisions the same
// integrity guarantees as newly appended revisions. It is idempotent and only
// fills rows whose hash is absent; an existing non-empty hash is never changed.
func (s *Store) backfillAllocationRevisionHashes(ctx context.Context) error {
	type row struct {
		id, subjectType, subjectID, parent, key, author, source, reason, created, previous, hash string
		number                                                                                   int64
		project                                                                                  string
		bp                                                                                       int64
		method, confidence                                                                       string
	}
	rows, err := s.db.QueryContext(ctx, `SELECT revision_id, subject_type, subject_id, revision_number, parent_revision_id, idempotency_key, project_id, allocation_basis_points, allocation_method, confidence, author, source, reason, created_at, previous_revision_hash, revision_hash FROM allocation_revisions ORDER BY subject_type, subject_id, revision_number, entry_id`)
	if err != nil {
		return err
	}
	groups := make(map[string]*AllocationRevision)
	order := make([]string, 0)
	for rows.Next() {
		var v row
		if err := rows.Scan(&v.id, &v.subjectType, &v.subjectID, &v.number, &v.parent, &v.key, &v.project, &v.bp, &v.method, &v.confidence, &v.author, &v.source, &v.reason, &v.created, &v.previous, &v.hash); err != nil {
			_ = rows.Close()
			return err
		}
		r := groups[v.id]
		if r == nil {
			parsed, e := time.Parse(time.RFC3339Nano, v.created)
			if e != nil {
				_ = rows.Close()
				return e
			}
			r = &AllocationRevision{ID: v.id, SubjectType: v.subjectType, SubjectID: v.subjectID, RevisionNumber: v.number, ParentRevisionID: v.parent, IdempotencyKey: v.key, Author: v.author, Source: v.source, Reason: v.reason, CreatedAt: parsed, PreviousHash: v.previous, RevisionHash: v.hash}
			groups[v.id] = r
			order = append(order, v.id)
		}
		r.Allocations = append(r.Allocations, Allocation{ProjectID: v.project, BasisPoints: v.bp, Method: v.method, Confidence: v.confidence})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	for _, id := range order {
		r := groups[id]
		if r.RevisionHash != "" {
			continue
		}
		r.RevisionHash = allocationRevisionHash(*r, func() []AllocationInput {
			out := make([]AllocationInput, 0, len(r.Allocations))
			for _, a := range r.Allocations {
				out = append(out, AllocationInput{ProjectID: a.ProjectID, BasisPoints: a.BasisPoints, Method: a.Method, Confidence: a.Confidence})
			}
			return out
		}(), r.PreviousHash)
		if _, err := tx.ExecContext(ctx, `UPDATE allocation_revisions SET previous_revision_hash=?, revision_hash=? WHERE revision_id=? AND revision_hash=''`, r.PreviousHash, r.RevisionHash, r.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func projectByLocation(ctx context.Context, tx *sql.Tx, path string) (domain.Project, domain.ProjectLocation, bool, error) {
	var project domain.Project
	var location domain.ProjectLocation
	var projectCreatedAt, locationCreatedAt string
	path = normalizeLocationPath(path)
	err := tx.QueryRowContext(ctx, `SELECT p.id, p.slug, p.name, p.canonical_key, p.created_at, l.id, l.project_id, l.absolute_path, l.path_hash, l.created_at FROM project_locations l JOIN projects p ON p.id = l.project_id WHERE LOWER(REPLACE(l.absolute_path, '\', '/')) = ?`, path).Scan(&project.ID, &project.Slug, &project.Name, &project.CanonicalKey, &projectCreatedAt, &location.ID, &location.ProjectID, &location.AbsolutePath, &location.PathHash, &locationCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return project, location, false, nil
	}
	if err != nil {
		return project, location, false, err
	}
	project.CreatedAt = parseTimestamp(projectCreatedAt)
	location.CreatedAt = parseTimestamp(locationCreatedAt)
	return project, location, true, nil
}

func normalizeLocationPath(value string) string {
	value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	return strings.TrimSuffix(strings.ToLower(value), "/")
}

func projectBySlug(ctx context.Context, tx *sql.Tx, slug string) (domain.Project, bool, error) {
	var project domain.Project
	var createdAt string
	err := tx.QueryRowContext(ctx, `SELECT id, slug, name, canonical_key, created_at FROM projects WHERE slug = ?`, slug).Scan(&project.ID, &project.Slug, &project.Name, &project.CanonicalKey, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return project, false, nil
	}
	if err == nil {
		project.CreatedAt = parseTimestamp(createdAt)
	}
	return project, err == nil, err
}

func canonicalEvent(input RawEventInput, payload []byte) string {
	value := struct {
		Source, SourceVersion, SessionID, EventType, OccurredAt, ProjectID, ProjectLocationID, WorkContextID, ResolutionMethod, ResolutionConfidence, EvidenceJSON, Payload string
	}{input.Source, input.SourceVersion, input.SessionID, input.EventType, timestamp(input.OccurredAt), input.ProjectID, input.ProjectLocationID, input.WorkContextID, input.ResolutionMethod, input.ResolutionConfidence, input.EvidenceJSON, string(payload)}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// CanonicalIngestionIdentity identifies a sanitized event without including receive time.
func CanonicalIngestionIdentity(input RawEventInput, sanitizedPayload []byte) (string, error) {
	if identity := strings.TrimSpace(input.IngestionIdentity); identity != "" {
		encoded, err := json.Marshal(struct{ Source, SessionID, Identity string }{input.Source, input.SessionID, identity})
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(encoded)
		return "upstream-sha256:" + hex.EncodeToString(hash[:]), nil
	}
	value := struct {
		Source, SourceVersion, SessionID, EventType, OccurredAt, ProjectID, ProjectLocationID, WorkContextID, ResolutionMethod, ResolutionConfidence, EvidenceJSON, Payload string
	}{input.Source, input.SourceVersion, input.SessionID, input.EventType, identityOccurredAt(input), input.ProjectID, input.ProjectLocationID, input.WorkContextID, input.ResolutionMethod, input.ResolutionConfidence, input.EvidenceJSON, string(sanitizedPayload)}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

func identityOccurredAt(input RawEventInput) string {
	if input.OmitOccurredAtFromIdentity {
		return ""
	}
	return timestamp(input.OccurredAt.UTC())
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func sanitizePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, fmt.Errorf("parse raw event JSON: %w", err)
	}
	sanitizeValue(value)
	return json.Marshal(value)
}

func sanitizeEvidence(evidence string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(evidence), &value); err != nil {
		value = map[string]any{"sanitized": "[REDACTED]"}
	}
	sanitizeValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}", nil
	}
	return string(encoded), nil
}

func sanitizeValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if sensitiveKey(key) {
				delete(typed, key)
				continue
			}
			sanitizeValue(child)
		}
	case []any:
		for _, child := range typed {
			sanitizeValue(child)
		}
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	switch key {
	case "prompt", "response", "content", "authorization", "api_key", "access_token", "secret", "password", "tool_arguments", "tool_results", "cookie", "token", "bearer", "apikey", "private_key", "credentials":
		return true
	default:
		return false
	}
}

func ensureParent(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	return nil
}

func quiescenceLockPath(databasePath string) string { return databasePath + ".quiescence.lock" }

func writerLockPath(databasePath string) string { return databasePath + ".writer.lock" }

func purgeMarkerPath(databasePath string) string { return databasePath + ".purge.pending" }

func writerQuiescenceError(err error) error {
	if errors.Is(err, storelock.ErrContended) {
		return errors.New("quiescence lock is held by an active diagnostic; retry after it exits")
	}
	return fmt.Errorf("acquire quiescence lock: %w", err)
}

func writerLockError(err error) error {
	if errors.Is(err, storelock.ErrContended) {
		return errors.New("writer lock is held by an active qlog process; retry after it exits")
	}
	return fmt.Errorf("acquire writer lock: %w", err)
}

func readerQuiescenceError(err error) error {
	if errors.Is(err, storelock.ErrMissing) {
		return errors.New("quiescence lock is missing; run qlog init to restore it")
	}
	if errors.Is(err, storelock.ErrContended) {
		return errors.New("quiescence lock is held by an active qlog client; retry after it exits")
	}
	return fmt.Errorf("acquire quiescence lock: %w", err)
}

func readerWriterLockError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("writer lock is missing; run qlog init to restore it")
	}
	return fmt.Errorf("inspect writer lock: %w", err)
}

func maintenanceQuiescenceError(err error) error {
	if errors.Is(err, storelock.ErrMissing) {
		return errors.New("quiescence lock is missing; run qlog init to restore it")
	}
	if errors.Is(err, storelock.ErrContended) {
		return errors.New("quiescence lock is held by an active qlog client; retry after it exits")
	}
	return fmt.Errorf("acquire maintenance quiescence lock: %w", err)
}

func maintenanceWriterLockError(err error) error {
	if errors.Is(err, storelock.ErrMissing) {
		return errors.New("writer lock is missing; run qlog init to restore it")
	}
	if errors.Is(err, storelock.ErrContended) {
		return errors.New("writer lock is held by an active qlog writer; retry after it exits")
	}
	return fmt.Errorf("acquire maintenance writer lock: %w", err)
}

func rejectActiveWAL(databasePath string) error {
	info, err := os.Stat(databasePath + "-wal")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect database WAL: %w", err)
	}
	if info.Size() > 0 {
		return errors.New("database has an active WAL; close active qlog writers and retry")
	}
	return nil
}

func isolatedSHMWarning(databasePath string) []string {
	if _, err := os.Stat(databasePath + "-shm"); err == nil {
		return []string{"warning: isolated SQLite SHM file detected; diagnostics did not modify it"}
	}
	return nil
}

func rollback(tx *sql.Tx) { _ = tx.Rollback() }

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTimestamp(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
func chainKey(source, sessionID string) string { return source + "\x00" + sessionID }
func durationMilliseconds(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return timestamp(value)
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullableTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}
func hashPath(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func normalizeSlug(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
