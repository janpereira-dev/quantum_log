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
	SchemaVersion        int                     `json:"schema_version"`
	GeneratedAt          time.Time               `json:"generated_at"`
	Version              string                  `json:"version"`
	Commit               string                  `json:"commit"`
	BuildDate            string                  `json:"build_date"`
	Platform             string                  `json:"platform"`
	ImplementationStatus string                  `json:"implementation_status"`
	ReadinessStatus      string                  `json:"readiness_status"`
	ExternalE2EStatus    string                  `json:"external_e2e_status"`
	Agents               []acceptanceAgentResult `json:"agents"`
	Files                map[string]string       `json:"files"`
	Privacy              []string                `json:"privacy"`
}

type acceptanceDiagnostics struct {
	LedgerStatus            string `json:"ledger_status"`
	CollectorLogSHA256      string `json:"collector_log_sha256,omitempty"`
	CollectorLogFingerprint string `json:"collector_log_fingerprint"`
}

func newAcceptanceCommand(home *string, version Version) *cobra.Command {
	acceptance := &cobra.Command{Use: "acceptance", Short: "Create privacy-safe local acceptance evidence"}
	var output string
	run := &cobra.Command{Use: "run", Short: "Write a sanitized local acceptance ZIP package", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if strings.TrimSpace(output) == "" {
			return errors.New("acceptance run requires --output <zip>")
		}
		if err := writeAcceptancePackage(command.Context(), *home, version, output); err != nil {
			return err
		}
		_, err := fmt.Fprintf(command.OutOrStdout(), "acceptance evidence: %s\n", output)
		return err
	}}
	run.Flags().StringVar(&output, "output", "", "destination ZIP path")
	acceptance.AddCommand(run)
	return acceptance
}

func writeAcceptancePackage(ctx context.Context, home string, version Version, output string) error {
	service, err := app.OpenReadOnly(ctx, home)
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
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	files["manifest.json"] = append(manifestJSON, '\n')
	files["SHA256SUMS"] = acceptanceChecksumFile(files)
	return writeAcceptanceZIP(output, files)
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
		recentEvidence, err := service.Store.HasRecentAdapterEvidence(ctx, sqlite.AdapterEvidenceQuery{
			AdapterID: adapter.Descriptor().ID, AllowedAgentNames: contract.AllowedAgentNames, Source: contract.Source,
			From: time.Now().UTC().Add(-time.Hour), To: time.Now().UTC(), RequiredQuality: string(contract.Quality),
			RequiredProvider: contract.RequiredProvider, RequireCodexResponseCompleted: contract.RequireCodexResponseCompleted,
		})
		if err != nil {
			return nil, fmt.Errorf("read adapter %s evidence: %w", adapter.Descriptor().ID, err)
		}
		result := acceptanceAgentStatus(adapter.Descriptor().ID, status.Available, status.Installed, recentEvidence)
		result.CaptureQuality = string(status.CaptureQuality)
		result.Source = contract.Source
		result.SourceEvidence = contract.SourceEvidence
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
	if recentEvidence {
		result.Status = acceptancePass
	}
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
	for _, result := range results {
		if result.Status != acceptancePass {
			return acceptancePendingExternalE2E
		}
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
