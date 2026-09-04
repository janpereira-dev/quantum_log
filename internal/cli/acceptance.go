package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	acceptancecontract "github.com/janpereira-dev/quantum_log/internal/acceptance"
	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

const (
	acceptanceImplementationComplete = "IMPLEMENTATION_COMPLETE"
	acceptanceReadyForExternalE2E    = "READY_FOR_EXTERNAL_E2E"
	acceptancePass                   = "PASS"
	acceptanceFail                   = "FAIL"
	acceptancePendingExternalE2E     = "PENDING_EXTERNAL_E2E"
)

type acceptanceAgentResult struct {
	AdapterID      string `json:"adapter_id"`
	CaptureQuality string `json:"capture_quality"`
	Source         string `json:"source"`
	SourceEvidence bool   `json:"source_evidence"`
	Available      bool   `json:"available"`
	Installed      bool   `json:"installed"`
	RecentEvidence bool   `json:"recent_evidence"`
	Status         string `json:"status"`
	Readiness      string `json:"readiness"`
}

type acceptanceManifest struct {
	SchemaVersion         int                                    `json:"schema_version"`
	GeneratedAt           time.Time                              `json:"generated_at"`
	Version               string                                 `json:"version"`
	Commit                string                                 `json:"commit"`
	BuildDate             string                                 `json:"build_date"`
	Platform              string                                 `json:"platform"`
	CollectorReachable    bool                                   `json:"collector_reachable"`
	ImplementationStatus  string                                 `json:"implementation_status"`
	ReadinessStatus       string                                 `json:"readiness_status"`
	ExternalE2EStatus     string                                 `json:"external_e2e_status"`
	Agents                []acceptanceAgentResult                `json:"agents"`
	Files                 map[string]string                      `json:"files"`
	Privacy               []string                               `json:"privacy"`
	RealAgentEvidence     []acceptancecontract.RealAgentEvidence `json:"real_agent_evidence,omitempty"`
	CandidateBinarySHA256 string                                 `json:"candidate_binary_sha256"`
	CandidateAuthenticity string                                 `json:"candidate_authenticity"`
}

type acceptanceDiagnostics struct {
	LedgerStatus            string `json:"ledger_status"`
	CollectorLogSHA256      string `json:"collector_log_sha256,omitempty"`
	CollectorLogFingerprint string `json:"collector_log_fingerprint"`
}

func newAcceptanceCommand(home *string, version Version) *cobra.Command {
	acceptance := &cobra.Command{Use: "acceptance", Short: "Create privacy-safe local acceptance evidence"}
	var output string
	var boundaryIDs []string
	run := &cobra.Command{Use: "run", Short: "Write a sanitized local acceptance ZIP package", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if strings.TrimSpace(output) == "" {
			return errors.New("acceptance run requires --output <zip>")
		}
		if err := writeAcceptancePackageWithBoundaries(command.Context(), *home, version, output, boundaryIDs); err != nil {
			return err
		}
		_, err := fmt.Fprintf(command.OutOrStdout(), "acceptance evidence: %s\n", output)
		return err
	}}
	run.Flags().StringVar(&output, "output", "", "destination ZIP path")
	run.Flags().StringArrayVar(&boundaryIDs, "boundary", nil, "repeatable qlog-created pre-action boundary id")
	var agentID, agentVersion string
	begin := &cobra.Command{Use: "begin", Short: "Create a one-use pre-action real-agent boundary", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		boundary, err := createAcceptanceBoundary(command.Context(), *home, version, agentID, agentVersion)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.OutOrStdout(), boundary.ID)
		return err
	}}
	begin.Flags().StringVar(&agentID, "agent", "", "supported agent id")
	begin.Flags().StringVar(&agentVersion, "agent-version", "", "sanitized exact agent version")
	_ = begin.MarkFlagRequired("agent")
	_ = begin.MarkFlagRequired("agent-version")
	acceptance.AddCommand(begin, run, newAcceptanceInspectCommand(version))
	return acceptance
}

func writeAcceptancePackage(ctx context.Context, home string, version Version, output string) error {
	return writeAcceptancePackageWithBoundaries(ctx, home, version, output, nil)
}

func writeAcceptancePackageWithBoundaries(ctx context.Context, home string, version Version, output string, boundaryIDs []string) error {
	tag, commit, binaryHash, platform, err := acceptanceRuntimeIdentity(version)
	if err != nil {
		return err
	}
	service, err := app.OpenSnapshotReadOnly(ctx, home)
	if err != nil {
		return err
	}
	defer func() { _ = service.Close() }()

	report, err := service.Store.CapabilityReport(ctx, sqlite.CapabilityQuery{})
	if err != nil {
		return fmt.Errorf("build capability report: %w", err)
	}
	sessions, err := service.Store.SessionSnapshots(ctx)
	if err != nil {
		return fmt.Errorf("build session summary: %w", err)
	}
	report = acceptanceSafeReport(report)
	sessions = acceptanceSafeSessions(sessions)
	collector, err := collectorStatus(ctx, home, "127.0.0.1:4318", true, false, newCollectorManager())
	if err != nil {
		return fmt.Errorf("read collector status: %w", err)
	}
	agents, err := acceptanceAgents(ctx, service, collector.Reachable)
	if err != nil {
		return err
	}
	diagnostics := acceptanceDiagnostics{LedgerStatus: acceptancePass, CollectorLogFingerprint: "not_available"}
	if err := service.Store.VerifyLedger(ctx, ""); err != nil {
		diagnostics.LedgerStatus = acceptanceFail
	}
	if acceptanceOwnedPath(service.Paths.Home, collector.LogPath) {
		if fingerprint, ok := acceptanceFileSHA256(collector.LogPath); ok {
			diagnostics.CollectorLogSHA256 = fingerprint
			diagnostics.CollectorLogFingerprint = "sha256"
		}
	}
	realAgentEvidence, err := evaluateAcceptanceBoundaries(ctx, service, tag, commit, binaryHash, platform, diagnostics.LedgerStatus, boundaryIDs)
	if err != nil {
		return err
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report JSON: %w", err)
	}
	sessionsJSON, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sessions: %w", err)
	}
	diagnosticsJSON, err := json.MarshalIndent(diagnostics, "", "  ")
	if err != nil {
		return fmt.Errorf("encode diagnostics: %w", err)
	}
	collectorJSON, err := json.MarshalIndent(struct {
		Installed bool     `json:"installed"`
		Running   bool     `json:"running"`
		Reachable bool     `json:"reachable"`
		Mode      string   `json:"mode"`
		Listen    string   `json:"listen"`
		ServiceID string   `json:"service_id"`
		Endpoints []string `json:"endpoints"`
		Scope     string   `json:"scope"`
	}{collector.Installed, collector.Running, collector.Reachable, collector.Mode, collector.Listen, collector.ServiceID, collector.Endpoints, collector.Scope}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode collector status: %w", err)
	}
	reportCSV := new(bytes.Buffer)
	if err := writeCapabilityCSV(reportCSV, report); err != nil {
		return fmt.Errorf("encode report CSV: %w", err)
	}
	reportText := new(bytes.Buffer)
	if err := writeCapabilityReport(reportText, report); err != nil {
		return fmt.Errorf("encode report text: %w", err)
	}
	files := map[string][]byte{
		"collector.json":   append(collectorJSON, '\n'),
		"diagnostics.json": append(diagnosticsJSON, '\n'),
		"report.csv":       reportCSV.Bytes(),
		"report.json":      append(reportJSON, '\n'),
		"report.txt":       reportText.Bytes(),
		"sessions.json":    append(sessionsJSON, '\n'),
	}
	privacyStatus := acceptancePackagePrivacyStatus(files, realAgentEvidence)
	if privacyStatus != acceptancecontract.StatusPass {
		return errors.New("acceptance package privacy scan failed")
	}
	for index := range realAgentEvidence {
		realAgentEvidence[index].PrivacyStatus = privacyStatus
		realAgentEvidence[index], _ = acceptancecontract.EvaluateRealAgentEvidence(realAgentEvidence[index])
	}
	agents = applyRealAgentEvidence(agents, realAgentEvidence)
	if len(realAgentEvidence) > 0 {
		evidenceJSON, err := json.MarshalIndent(realAgentEvidence, "", "  ")
		if err != nil {
			return fmt.Errorf("encode real-agent evidence: %w", err)
		}
		files["real-agent-evidence.json"] = append(evidenceJSON, '\n')
	}
	externalE2EStatus := acceptanceExternalStatus(agents)
	if diagnostics.LedgerStatus == acceptanceFail {
		externalE2EStatus = acceptanceFail
	}
	manifest := acceptanceManifest{
		SchemaVersion:        1,
		GeneratedAt:          time.Now().UTC(),
		Version:              version.Version,
		Commit:               version.Commit,
		BuildDate:            version.BuildDate,
		Platform:             runtime.GOOS + "/" + runtime.GOARCH,
		CollectorReachable:   collector.Reachable,
		ImplementationStatus: acceptanceImplementationComplete,
		ReadinessStatus:      acceptanceReadiness(agents),
		ExternalE2EStatus:    externalE2EStatus,
		Agents:               agents,
		Files:                acceptanceHashes(files),
		Privacy: []string{
			"No raw events or payloads are included.",
			"Prompt, response, tool, authentication, authorization, and secret content are excluded.",
			"Diagnostics contain only status and qlog-owned log fingerprints, never log content or paths.",
			"PASS records local evidence only and never asserts external verification.",
		},
		RealAgentEvidence:     realAgentEvidence,
		CandidateAuthenticity: "PENDING_EXTERNAL_REVIEW",
	}
	manifest.Commit, manifest.CandidateBinarySHA256, manifest.Platform = commit, binaryHash, platform
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	files["manifest.json"] = append(manifestJSON, '\n')
	files["SHA256SUMS"] = acceptanceChecksumFile(files)
	if err := validateAcceptancePackageFileSizes(files); err != nil {
		return err
	}
	reservations, err := reserveAcceptanceBoundaries(service.Paths.Home, boundaryIDs)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			releaseAcceptanceReservations(reservations)
		}
	}()
	if err := writeAcceptanceZIP(output, files); err != nil {
		return err
	}
	if err := commitAcceptanceReservations(reservations, time.Now().UTC()); err != nil {
		return err
	}
	committed = true
	return nil
}

func applyRealAgentEvidence(agents []acceptanceAgentResult, evidence []acceptancecontract.RealAgentEvidence) []acceptanceAgentResult {
	byAgent := make(map[string]acceptancecontract.RealAgentEvidence, len(evidence))
	for _, item := range evidence {
		byAgent[item.AgentID] = item
	}
	for index := range agents {
		item, found := byAgent[agents[index].AdapterID]
		if !found {
			agents[index].Status = acceptancePendingExternalE2E
			agents[index].SourceEvidence = false
			continue
		}
		agents[index].Status = item.Status
		agents[index].SourceEvidence = item.SourceEvidence
		if !item.SourceEvidence {
			agents[index].SourceEvidence = false
		}
	}
	return agents
}

func acceptanceSafeReport(report sqlite.CapabilityReport) sqlite.CapabilityReport {
	report.ProjectSlug = acceptanceOpaqueID(report.ProjectSlug)
	report.AgentName = acceptanceOpaqueID(report.AgentName)
	report.SessionID = acceptanceOpaqueID(report.SessionID)
	for index := range report.Sources {
		report.Sources[index].Source = acceptanceSafeVocabulary(report.Sources[index].Source, acceptanceKnownSources)
		report.Sources[index].Quality = acceptanceSafeVocabulary(report.Sources[index].Quality, acceptanceKnownCaptureQuality)
		if report.Sources[index].Version != nil && (strings.ContainsAny(*report.Sources[index].Version, `/\\:`) || strings.ContainsAny(*report.Sources[index].Version, "\r\n\x00")) {
			redacted := acceptanceOpaqueID(*report.Sources[index].Version)
			report.Sources[index].Version = &redacted
		}
	}
	for index := range report.MetricCoverage {
		for provenanceIndex := range report.MetricCoverage[index].Provenance {
			provenance := &report.MetricCoverage[index].Provenance[provenanceIndex]
			provenance.Source = acceptanceSafeVocabulary(provenance.Source, acceptanceKnownProvenanceSources)
			provenance.RawKey = acceptanceSafeVocabulary(provenance.RawKey, acceptanceKnownRawKeys)
			provenance.Confidence = acceptanceSafeVocabulary(provenance.Confidence, acceptanceKnownConfidence)
		}
	}
	return report
}

var acceptanceKnownSources = map[string]bool{"claude-code-hook": true, "codex-app-server": true, "copilot-cli-hook": true, "opencode-plugin": true, "otlp-http": true, "qlog-plugin": true}
var acceptanceKnownProvenanceSources = map[string]bool{"otel": true, "opencode": true}
var acceptanceKnownRawKeys = map[string]bool{"input_tokens": true, "output_tokens": true, "reasoning_output_tokens": true, "cache_read_input_tokens": true, "cache_creation_input_tokens": true, "gen_ai.usage.input_tokens": true, "gen_ai.usage.output_tokens": true, "gen_ai.usage.prompt_tokens": true, "gen_ai.usage.completion_tokens": true, "gen_ai.usage.reasoning.output_tokens": true, "gen_ai.usage.reasoning_tokens": true, "gen_ai.usage.cache_read.input_tokens": true, "gen_ai.usage.cache_creation.input_tokens": true, "gen_ai.usage.total_tokens": true, "tokens.input": true, "tokens.output": true, "tokens.reasoning": true, "tokens.cache.read": true, "tokens.cache.write": true}
var acceptanceKnownConfidence = map[string]bool{"reported": true}
var acceptanceKnownCaptureQuality = map[string]bool{"agent_reported": true, "estimated": true, "lifecycle_only": true, "otel_reported": true, "unavailable": true, "unknown": true}

func acceptanceSafeVocabulary(value string, allowed map[string]bool) string {
	if value == "" || allowed[value] {
		return value
	}
	return acceptanceOpaqueID(value)
}

func acceptanceSafeSessions(sessions []sqlite.SessionSnapshot) []sqlite.SessionSnapshot {
	for index := range sessions {
		sessions[index].SessionID = acceptanceOpaqueID(sessions[index].SessionID)
		sessions[index].AgentName = acceptanceOpaqueID(sessions[index].AgentName)
	}
	return sessions
}

func acceptanceOpaqueID(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:12])
}

func acceptanceAgents(ctx context.Context, service *app.Service, collectorReachable bool) ([]acceptanceAgentResult, error) {
	registry := adapters.Default()
	items := registry.Stable()
	results := make([]acceptanceAgentResult, 0, len(items))
	for _, adapter := range items {
		status, err := adapter.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("read adapter %s status: %w", adapter.Descriptor().ID, err)
		}
		contract := evidenceContract(adapter.Descriptor().ID)
		now := time.Now().UTC()
		recentEvidence, err := hasAdapterEvidence(ctx, service.Store, adapter.Descriptor().ID, "", now.Add(-time.Hour), now, contract)
		if err != nil {
			return nil, fmt.Errorf("read adapter %s evidence: %w", adapter.Descriptor().ID, err)
		}
		result := acceptanceAgentStatus(adapter.Descriptor().ID, status.Available, status.Installed, recentEvidence)
		result.CaptureQuality = string(status.CaptureQuality)
		result.Source = contract.Source
		result.SourceEvidence = false
		if !collectorReachable {
			result.Readiness = acceptancePendingExternalE2E
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].AdapterID < results[j].AdapterID })
	return results, nil
}

func acceptanceAgentStatus(adapterID string, available, installed, recentEvidence bool) acceptanceAgentResult {
	result := acceptanceAgentResult{AdapterID: adapterID, Available: available, Installed: installed, RecentEvidence: recentEvidence, Status: acceptancePendingExternalE2E, Readiness: acceptancePendingExternalE2E}
	if available && installed {
		result.Readiness = acceptanceReadyForExternalE2E
	}
	return result
}

func acceptanceReadiness(results []acceptanceAgentResult) string {
	for _, result := range results {
		if result.Readiness != acceptanceReadyForExternalE2E {
			return acceptancePendingExternalE2E
		}
	}
	return acceptanceReadyForExternalE2E
}

func acceptanceExternalStatus(results []acceptanceAgentResult) string {
	pending := false
	for _, result := range results {
		if result.Status == acceptanceFail {
			return acceptanceFail
		}
		if result.Status != acceptancePass {
			pending = true
		}
	}
	if pending {
		return acceptancePendingExternalE2E
	}
	return acceptancePass
}

func acceptanceFileSHA256(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", false
	}
	return hex.EncodeToString(hash.Sum(nil)), true
}

func acceptanceOwnedPath(home, candidate string) bool {
	if strings.TrimSpace(home) == "" || strings.TrimSpace(candidate) == "" {
		return false
	}
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return false
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(resolvedHome, resolvedCandidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func acceptanceHashes(files map[string][]byte) map[string]string {
	hashes := make(map[string]string, len(files))
	for name, data := range files {
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	return hashes
}

func acceptanceChecksumFile(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for name := range files {
		if name != "SHA256SUMS" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	buffer := new(bytes.Buffer)
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		_, _ = fmt.Fprintf(buffer, "%x  %s\n", sum, name)
	}
	return buffer.Bytes()
}

func writeAcceptanceZIP(output string, files map[string][]byte) error {
	directory := filepath.Dir(output)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".qlog-acceptance-*.zip")
	if err != nil {
		return fmt.Errorf("create acceptance package: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	archive := zip.NewWriter(temporary)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return fmt.Errorf("create ZIP entry %s: %w", name, err)
		}
		if _, err := writer.Write(files[name]); err != nil {
			_ = archive.Close()
			_ = temporary.Close()
			return fmt.Errorf("write ZIP entry %s: %w", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("close acceptance ZIP: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close acceptance package: %w", err)
	}
	if err := os.Rename(temporaryName, output); err != nil {
		return fmt.Errorf("publish acceptance package: %w", err)
	}
	return nil
}
