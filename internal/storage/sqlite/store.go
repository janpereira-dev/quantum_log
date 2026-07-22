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
	"path"
	"path/filepath"
	"sort"
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
	Source               string
	SessionID            string
	EventType            string
	Payload              []byte
	OccurredAt           time.Time
	ProjectID            string
	ProjectLocationID    string
	WorkContextID        string
	ResolutionMethod     string
	ResolutionConfidence string
	EvidenceJSON         string
}

type AllocationInput struct {
	ProjectID   string
	BasisPoints int64
}

type ModelCallInput struct {
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
	CaptureQuality         string
}

type UsageQuery struct {
	From        time.Time
	To          time.Time
	ProjectSlug string
	GroupBy     []string
}

type CopilotOTLPEvidenceQuery struct {
	From        time.Time
	To          time.Time
	ProjectSlug string
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

type UsageReport struct {
	GroupBy                []string   `json:"group_by"`
	Rows                   []UsageRow `json:"rows"`
	TotalTokens            int64      `json:"total_tokens"`
	AllocatedCostUSDMicros int64      `json:"allocated_cost_usd_micros"`
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
	ModelCallCount         int64 `json:"model_call_count"`
	ObservedTokens         int64 `json:"observed_tokens"`
	AllocatedCostUSDMicros int64 `json:"allocated_cost_usd_micros"`
}

type ProjectReport struct {
	Project                ProjectSummary `json:"project"`
	Tags                   []ProjectTag   `json:"tags"`
	ActiveTaskCount        int64          `json:"active_task_count"`
	ObservedModelCallCount int64          `json:"observed_model_call_count"`
	ObservedTokens         int64          `json:"observed_tokens"`
	AllocatedCostUSDMicros int64          `json:"allocated_cost_usd_micros"`
	BudgetAlerts           []BudgetAlert  `json:"budget_alerts"`
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
	return project, location, nil
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

func (s *Store) AppendRawEvent(ctx context.Context, input RawEventInput) (string, error) {
	if strings.TrimSpace(input.Source) == "" || strings.TrimSpace(input.EventType) == "" {
		return "", errors.New("raw event source and type are required")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	payload, err := sanitizePayload(input.Payload)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("sanitize evidence: %w", err)
	}
	input.EvidenceJSON = sanitizedEvidence
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin raw event: %w", err)
	}
	defer rollback(tx)
	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT event_hash FROM raw_events WHERE source = ? AND COALESCE(session_id, '') = ? ORDER BY created_at DESC, id DESC LIMIT 1`, input.Source, input.SessionID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read ledger head: %w", err)
	}
	canonical := canonicalEvent(input, payload)
	event := audit.NewRecord(chainKey(input.Source, input.SessionID), canonical, previousHash)
	id := newID()
	now := timestamp(time.Now())
	_, err = tx.ExecContext(ctx, `INSERT INTO raw_events (id, source, event_type, occurred_at, received_at, project_id, project_location_id, work_context_id, session_id, project_resolution_method, project_resolution_confidence, project_resolution_evidence_json, payload_json_sanitized, previous_event_hash, event_hash, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, input.Source, input.EventType, timestamp(input.OccurredAt), now, nullable(input.ProjectID), nullable(input.ProjectLocationID), nullable(input.WorkContextID), nullable(input.SessionID), input.ResolutionMethod, input.ResolutionConfidence, input.EvidenceJSON, string(payload), previousHash, event.Hash, now)
	if err != nil {
		return "", fmt.Errorf("insert raw event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit raw event: %w", err)
	}
	return id, nil
}

func (s *Store) VerifyLedger(ctx context.Context, sessionID string) error {
	query := `SELECT source, COALESCE(session_id, ''), event_type, occurred_at, project_id, project_location_id, work_context_id, project_resolution_method, project_resolution_confidence, project_resolution_evidence_json, payload_json_sanitized, previous_event_hash, event_hash FROM raw_events`
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
		var source, session, eventType, occurredAt, resolutionMethod, resolutionConfidence, evidence, payload, previousHash, eventHash string
		var projectID, locationID, contextID sql.NullString
		if err := rows.Scan(&source, &session, &eventType, &occurredAt, &projectID, &locationID, &contextID, &resolutionMethod, &resolutionConfidence, &evidence, &payload, &previousHash, &eventHash); err != nil {
			return fmt.Errorf("scan ledger event: %w", err)
		}
		key := chainKey(source, session)
		if previousHash != previous[key] {
			return errors.New("ledger previous hash does not match")
		}
		canonical := canonicalEvent(RawEventInput{Source: source, SessionID: session, EventType: eventType, OccurredAt: parseTimestamp(occurredAt), ProjectID: projectID.String, ProjectLocationID: locationID.String, WorkContextID: contextID.String, ResolutionMethod: resolutionMethod, ResolutionConfidence: resolutionConfidence, EvidenceJSON: evidence}, []byte(payload))
		if audit.Hash(key, canonical, previousHash) != eventHash {
			return errors.New("ledger event hash does not match")
		}
		previous[key] = eventHash
	}
	return rows.Err()
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
	report := ProjectReport{Tags: make([]ProjectTag, 0), BudgetAlerts: make([]BudgetAlert, 0)}
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
	now := timestamp(time.Now())
	total := input.InputTokens + input.OutputTokens + input.ReasoningTokens + input.CachedInputTokens + input.CacheWriteTokens
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `INSERT INTO model_calls (id, primary_project_id, project_location_id, work_context_id, task_id, session_id, turn_id, started_at, agent_name, provider, model_id, input_tokens, output_tokens, reasoning_tokens, cached_input_tokens, cache_write_tokens, total_tokens, estimated_cost_usd_micros, estimated_cost_eur_micros, capture_quality, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, nullable(input.ProjectID), nullable(input.ProjectLocationID), nullable(input.WorkContextID), nullable(input.TaskID), nullable(input.SessionID), nullable(input.TurnID), timestamp(input.OccurredAt), input.AgentName, input.Provider, input.ModelID, input.InputTokens, input.OutputTokens, input.ReasoningTokens, input.CachedInputTokens, input.CacheWriteTokens, total, input.EstimatedCostUSDMicros, input.EstimatedCostEURMicros, input.CaptureQuality, now)
	if err != nil {
		return "", fmt.Errorf("insert model call: %w", err)
	}
	if input.ProjectID != "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO usage_allocations (id, subject_type, subject_id, project_id, allocation_basis_points, allocation_method, confidence, created_at) VALUES (?, 'model_call', ?, ?, 10000, 'direct', 'high', ?)`, newID(), id, input.ProjectID, now)
		if err != nil {
			return "", fmt.Errorf("insert direct allocation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
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
	return s.replaceAllocations(ctx, subjectType, subjectID, allocations, "split")
}

func (s *Store) RepairModelCallAllocation(ctx context.Context, modelCallID, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("repair project id is required")
	}
	return s.replaceAllocations(ctx, "model_call", modelCallID, []AllocationInput{{ProjectID: projectID, BasisPoints: 10000}}, "manual")
}

func (s *Store) AssignUnattributedModelCall(ctx context.Context, modelCallID, projectID string) error {
	if strings.TrimSpace(modelCallID) == "" || strings.TrimSpace(projectID) == "" {
		return errors.New("model call id and project id are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM model_calls WHERE id = ?`, modelCallID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("model call %q not found", modelCallID)
		}
		return fmt.Errorf("read model call allocation: %w", err)
	}
	var allocationCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_allocations WHERE subject_type = 'model_call' AND subject_id = ?`, modelCallID).Scan(&allocationCount); err != nil {
		return fmt.Errorf("read model call allocations: %w", err)
	}
	if allocationCount > 0 {
		return fmt.Errorf("model call %q already has allocations; use a split to replace them", modelCallID)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usage_allocations (id, subject_type, subject_id, project_id, allocation_basis_points, allocation_method, confidence, created_at) VALUES (?, 'model_call', ?, ?, 10000, 'manual', 'high', ?)`, newID(), modelCallID, projectID, timestamp(time.Now())); err != nil {
		return fmt.Errorf("insert allocation: %w", err)
	}
	return tx.Commit()
}

func (s *Store) replaceAllocations(ctx context.Context, subjectType, subjectID string, allocations []AllocationInput, method string) error {
	if subjectType != "model_call" {
		return errors.New("only model_call allocations are supported")
	}
	if strings.TrimSpace(subjectID) == "" {
		return errors.New("allocation subject id is required")
	}
	if err := ValidateAllocations(allocations); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM model_calls WHERE id = ?`, subjectID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("model call %q not found", subjectID)
		}
		return fmt.Errorf("read model call allocation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_allocations WHERE subject_type = ? AND subject_id = ?`, subjectType, subjectID); err != nil {
		return fmt.Errorf("replace allocations: %w", err)
	}
	for _, allocation := range allocations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_allocations (id, subject_type, subject_id, project_id, allocation_basis_points, allocation_method, confidence, created_at) VALUES (?, ?, ?, ?, ?, ?, 'high', ?)`, newID(), subjectType, subjectID, allocation.ProjectID, allocation.BasisPoints, method, timestamp(time.Now())); err != nil {
			return fmt.Errorf("insert allocation: %w", err)
		}
	}
	return tx.Commit()
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
	where, args := usageWindow(query)
	var totalTokens int64
	totalQuery := `SELECT COALESCE(SUM(total_tokens), 0) FROM model_calls c` + where
	if query.ProjectSlug != "" {
		totalQuery = `SELECT COALESCE(SUM(c.total_tokens), 0) FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id LEFT JOIN projects p ON p.id = a.project_id` + where
	}
	if err := s.db.QueryRowContext(ctx, totalQuery, args...).Scan(&totalTokens); err != nil {
		return UsageReport{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(p.slug, 'unattributed'), c.agent_name, c.provider, c.model_id, c.capture_quality, c.input_tokens, c.output_tokens, c.reasoning_tokens, c.cached_input_tokens, c.cache_write_tokens, c.total_tokens, c.estimated_cost_usd_micros, a.allocation_basis_points FROM model_calls c JOIN usage_allocations a ON a.subject_type = 'model_call' AND a.subject_id = c.id LEFT JOIN projects p ON p.id = a.project_id`+where+` ORDER BY p.slug, c.agent_name, c.provider, c.model_id, c.capture_quality`, args...)
	if err != nil {
		return UsageReport{}, err
	}
	defer func() { _ = rows.Close() }()
	grouped := make(map[string]UsageRow)
	var totalAllocated int64
	for rows.Next() {
		var row UsageRow
		var cost, basisPoints int64
		if err := rows.Scan(&row.ProjectSlug, &row.AgentName, &row.Provider, &row.Model, &row.CaptureQuality, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CachedInputTokens, &row.CacheWriteTokens, &row.TotalTokens, &cost, &basisPoints); err != nil {
			return UsageReport{}, err
		}
		row.AllocatedCostUSDMicros = cost * basisPoints / 10000
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
	report := UsageReport{GroupBy: append([]string(nil), query.GroupBy...), Rows: make([]UsageRow, 0), TotalTokens: totalTokens, AllocatedCostUSDMicros: totalAllocated}
	for _, row := range grouped {
		report.Rows = append(report.Rows, row)
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		left := report.Rows[i].ProjectSlug + report.Rows[i].AgentName + report.Rows[i].Provider + report.Rows[i].Model + report.Rows[i].CaptureQuality
		right := report.Rows[j].ProjectSlug + report.Rows[j].AgentName + report.Rows[j].Provider + report.Rows[j].Model + report.Rows[j].CaptureQuality
		return left < right
	})
	return report, nil
}

func (s *Store) HasRecentCopilotOTLPModelCall(ctx context.Context, query CopilotOTLPEvidenceQuery) (bool, error) {
	where := " WHERE r.source = 'otlp-http' AND replace(lower(r.event_type), '_', '.') = 'model.call'"
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
	rows, err := s.db.QueryContext(ctx, `SELECT r.payload_json_sanitized FROM raw_events r LEFT JOIN projects p ON p.id = r.project_id`+where, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			return false, err
		}
		var payload struct {
			Provider       string `json:"provider"`
			AgentName      string `json:"agent_name"`
			InputTokens    int64  `json:"input_tokens"`
			OutputTokens   int64  `json:"output_tokens"`
			Reasoning      int64  `json:"reasoning_tokens"`
			CachedInput    int64  `json:"cached_input_tokens"`
			CacheWrite     int64  `json:"cache_write_tokens"`
			CaptureQuality string `json:"capture_quality"`
		}
		if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
			return false, err
		}
		if strings.EqualFold(payload.Provider, "github") && strings.Contains(strings.ToLower(payload.AgentName), "copilot") && payload.CaptureQuality == "otel_reported" && payload.InputTokens+payload.OutputTokens+payload.Reasoning+payload.CachedInput+payload.CacheWrite > 0 {
			return true, nil
		}
	}
	return false, rows.Err()
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
	return nil
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
		Source, SessionID, EventType, OccurredAt, ProjectID, ProjectLocationID, WorkContextID, ResolutionMethod, ResolutionConfidence, EvidenceJSON, Payload string
	}{input.Source, input.SessionID, input.EventType, timestamp(input.OccurredAt), input.ProjectID, input.ProjectLocationID, input.WorkContextID, input.ResolutionMethod, input.ResolutionConfidence, input.EvidenceJSON, string(payload)}
	encoded, _ := json.Marshal(value)
	return string(encoded)
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
