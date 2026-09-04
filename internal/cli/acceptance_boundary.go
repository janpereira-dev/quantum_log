package cli

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	acceptancecontract "github.com/janpereira-dev/quantum_log/internal/acceptance"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

func createAcceptanceBoundary(ctx context.Context, home string, version Version, agentID, agentVersion string) (acceptancecontract.RealAgentBoundary, error) {
	if !acceptancecontract.IsSupportedAgentID(agentID) {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("agent %q is unsupported or deferred for acceptance", agentID)
	}
	service, err := app.Open(ctx, home)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, err
	}
	defer func() { _ = service.Close() }()
	tag, commit, binaryHash, platform, err := acceptanceRuntimeIdentity(version)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, err
	}
	anchors, err := service.Store.LedgerAnchors(ctx)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("read pre-action ledger position: %w", err)
	}
	contract := evidenceContract(agentID)
	preActionReport, err := service.Store.CapabilityReport(ctx, sqlite.CapabilityQuery{AgentName: agentID})
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("read pre-action agent position: %w", err)
	}
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("create acceptance challenge: %w", err)
	}
	boundary := acceptancecontract.RealAgentBoundary{
		SchemaVersion: acceptancecontract.RealAgentBoundarySchemaVersion, Challenge: hex.EncodeToString(challengeBytes),
		CandidateTag: tag, CandidateCommit: commit, CandidateBinarySHA256: binaryHash, Platform: platform,
		AgentID: agentID, AgentVersion: agentVersion, StartedAt: time.Now().UTC(),
		LedgerPositionSHA256: ledgerPositionSHA256(anchors), LedgerEventCount: ledgerEventCount(anchors), AgentSourceModelCalls: sourceModelCallCount(preActionReport, contract),
	}
	boundary.ID = acceptancecontract.BoundaryID(boundary)
	if err := acceptancecontract.ValidateRealAgentBoundary(boundary, boundary.StartedAt, tag, commit, binaryHash, platform); err != nil {
		return acceptancecontract.RealAgentBoundary{}, err
	}
	marker := sqlite.AcceptanceBoundaryMarker{BoundaryID: boundary.ID, Challenge: boundary.Challenge, LedgerPositionSHA256: boundary.LedgerPositionSHA256, LedgerEventCount: boundary.LedgerEventCount}
	if _, err := service.Store.AppendAcceptanceBoundaryMarker(ctx, marker, boundary.StartedAt); err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("persist acceptance boundary in ledger: %w", err)
	}
	directory := filepath.Join(service.Paths.Home, "acceptance", "boundaries")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("create acceptance boundary directory: %w", err)
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return acceptancecontract.RealAgentBoundary{}, errors.New("acceptance boundary directory must be qlog-owned and non-symlink")
	}
	encoded, err := json.Marshal(boundary)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, err
	}
	path := filepath.Join(directory, boundary.ID+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("persist acceptance boundary: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("persist acceptance boundary: %w", err)
	}
	if err := file.Close(); err != nil {
		return acceptancecontract.RealAgentBoundary{}, err
	}
	return boundary, nil
}

func evaluateAcceptanceBoundaries(ctx context.Context, service *app.Service, version Version, ledgerStatus string, ids []string) ([]acceptancecontract.RealAgentEvidence, error) {
	results := make([]acceptancecontract.RealAgentEvidence, 0, len(ids))
	seenAgents := make(map[string]bool, len(ids))
	tag, commit, binaryHash, platform, err := acceptanceRuntimeIdentity(version)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if !isLowerHex(id, 64) {
			return nil, errors.New("acceptance boundary id must be 64 lowercase hexadecimal characters")
		}
		boundary, err := loadAcceptanceBoundary(service.Paths.Home, id)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		if err := acceptancecontract.ValidateRealAgentBoundary(boundary, now, tag, commit, binaryHash, platform); err != nil {
			return nil, err
		}
		marker := sqlite.AcceptanceBoundaryMarker{BoundaryID: boundary.ID, Challenge: boundary.Challenge, LedgerPositionSHA256: boundary.LedgerPositionSHA256, LedgerEventCount: boundary.LedgerEventCount}
		markerFound, err := service.Store.HasAcceptanceBoundaryMarker(ctx, marker)
		if err != nil {
			return nil, err
		}
		if !markerFound {
			return nil, errors.New("acceptance boundary is not authenticated by the qlog ledger")
		}
		if seenAgents[boundary.AgentID] {
			return nil, fmt.Errorf("duplicate real-agent boundary for %q", boundary.AgentID)
		}
		seenAgents[boundary.AgentID] = true
		anchors, err := service.Store.LedgerAnchors(ctx)
		if err != nil {
			return nil, err
		}
		contract := evidenceContract(boundary.AgentID)
		currentAgentReport, err := service.Store.CapabilityReport(ctx, sqlite.CapabilityQuery{AgentName: boundary.AgentID})
		if err != nil {
			return nil, err
		}
		positionAdvanced := ledgerEventCount(anchors) > boundary.LedgerEventCount && ledgerPositionSHA256(anchors) != boundary.LedgerPositionSHA256 && sourceModelCallCount(currentAgentReport, contract) > boundary.AgentSourceModelCalls
		found := false
		if positionAdvanced {
			found, err = hasAdapterEvidence(ctx, service.Store, boundary.AgentID, "", boundary.StartedAt.Add(time.Nanosecond), now, contract)
			if err != nil {
				return nil, err
			}
		}
		report, err := service.Store.CapabilityReport(ctx, sqlite.CapabilityQuery{From: boundary.StartedAt.Add(time.Nanosecond), To: now, AgentName: boundary.AgentID})
		if err != nil {
			return nil, err
		}
		metrics, quality := observedEvidence(report, contract)
		evidence := acceptancecontract.RealAgentEvidence{
			SchemaVersion: acceptancecontract.RealAgentSchemaVersion, CandidateTag: tag, CandidateCommit: commit,
			CandidateBinarySHA256: binaryHash, Platform: platform, AgentID: boundary.AgentID, AgentVersion: boundary.AgentVersion,
			BoundaryID: boundary.ID, StartedAt: boundary.StartedAt, EndedAt: now, SourceEvidence: found,
			LedgerStatus: ledgerStatus, PrivacyStatus: acceptancecontract.StatusPendingExternalE2E,
			ReplayStatus: acceptancecontract.StatusPendingExternalE2E, CaptureQuality: quality, ObservedMetrics: metrics,
		}
		evaluated, evaluateErr := acceptancecontract.EvaluateRealAgentEvidenceForCandidate(evidence, tag, commit)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		results = append(results, evaluated)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].StartedAt.Equal(results[j].StartedAt) {
			return results[i].AgentID < results[j].AgentID
		}
		return results[i].StartedAt.Before(results[j].StartedAt)
	})
	return results, nil
}

func loadAcceptanceBoundary(home, id string) (acceptancecontract.RealAgentBoundary, error) {
	path := filepath.Join(home, "acceptance", "boundaries", id+".json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 16*1024 {
		return acceptancecontract.RealAgentBoundary{}, errors.New("acceptance boundary must be a bounded regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("read acceptance boundary: %w", err)
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return acceptancecontract.RealAgentBoundary{}, fmt.Errorf("decode acceptance boundary: %w", err)
	}
	var boundary acceptancecontract.RealAgentBoundary
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&boundary); err != nil {
		return boundary, fmt.Errorf("decode acceptance boundary: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return boundary, errors.New("decode acceptance boundary: trailing JSON value")
	}
	if boundary.ID != id {
		return boundary, errors.New("acceptance boundary id does not match its persisted filename")
	}
	return boundary, nil
}

func consumeAcceptanceBoundaries(home string, ids []string, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		p := filepath.Join(home, "acceptance", "boundaries", id+".used")
		if _, err := os.Lstat(p); err == nil {
			return errors.New("acceptance boundary has already been used")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect acceptance boundary usage: %w", err)
		}
		paths = append(paths, p)
	}
	created := make([]string, 0, len(paths))
	rollback := func() {
		for _, p := range created {
			_ = os.Remove(p)
		}
	}
	for _, p := range paths {
		file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			rollback()
			return errors.New("acceptance boundary has already been used")
		}
		if err != nil {
			rollback()
			return fmt.Errorf("consume acceptance boundary: %w", err)
		}
		_, writeErr := fmt.Fprintln(file, now.Format(time.RFC3339Nano))
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			rollback()
			return fmt.Errorf("consume acceptance boundary: %w", err)
		}
		created = append(created, p)
	}
	return nil
}

func acceptanceRuntimeIdentity(version Version) (string, string, string, string, error) {
	tag := canonicalCandidateTag(version.Version)
	executable, err := os.Executable()
	if err != nil {
		return "", "", "", "", err
	}
	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", "", errors.New("qlog executable must be an exact regular non-symlink file")
	}
	hash, ok := acceptanceFileSHA256(executable)
	if !ok {
		return "", "", "", "", errors.New("hash qlog executable")
	}
	return tag, version.Commit, hash, runtime.GOOS + "/" + runtime.GOARCH, nil
}

func canonicalCandidateTag(version string) string {
	if version != "" && !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func ledgerPositionSHA256(anchors []sqlite.LedgerAnchor) string {
	encoded, _ := json.Marshal(anchors)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func ledgerEventCount(anchors []sqlite.LedgerAnchor) int64 {
	var count int64
	for _, anchor := range anchors {
		count += anchor.Events
	}
	return count
}

func observedEvidence(report sqlite.CapabilityReport, contract adapterEvidenceContract) ([]string, string) {
	metrics := make([]string, 0)
	for _, metric := range report.MetricCoverage {
		if metric.ReportedCount > 0 {
			metrics = append(metrics, metric.Name)
		}
	}
	sort.Strings(metrics)
	quality := ""
	for _, source := range report.Sources {
		if source.Source == contract.Source && source.Quality == string(contract.requiredEvidenceQuality()) && source.ModelCalls > 0 {
			quality = source.Quality
			break
		}
	}
	return metrics, quality
}

func sourceModelCallCount(report sqlite.CapabilityReport, contract adapterEvidenceContract) int64 {
	var count int64
	for _, source := range report.Sources {
		if source.Source == contract.Source && source.Quality == string(contract.requiredEvidenceQuality()) {
			count += source.ModelCalls
		}
	}
	return count
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key := keyToken.(string)
			if seen[key] {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = true
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
	}
	_, err = decoder.Token()
	return err
}

func acceptancePackagePrivacyStatus(files map[string][]byte, evidence []acceptancecontract.RealAgentEvidence) string {
	for name, data := range files {
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"must-not-export", "bearer ", "sk-", "ghp_", "github_pat_", "akia", "-----begin"} {
			if strings.Contains(lower, forbidden) {
				return acceptancecontract.StatusFail
			}
		}
		if filepath.Ext(name) == ".json" {
			if rejectDuplicateJSONKeys(data) != nil || !privacySafeJSON(data) {
				return acceptancecontract.StatusFail
			}
		}
	}
	encoded, err := json.Marshal(evidence)
	if err != nil || !privacySafeJSON(encoded) {
		return acceptancecontract.StatusFail
	}
	return acceptancecontract.StatusPass
}

func privacySafeJSON(data []byte) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return privacySafeJSONValue(value)
}

func privacySafeJSONValue(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
			forbiddenKeys := map[string]bool{"prompt": true, "promptbody": true, "prompttext": true, "promptcontent": true, "response": true, "responsebody": true, "responsetext": true, "responsecontent": true, "toolargs": true, "toolarguments": true, "toolresults": true, "authorization": true, "credential": true, "credentials": true, "environmentvalue": true, "environmentvalues": true, "filecontent": true}
			if forbiddenKeys[normalized] {
				return false
			}
			if !privacySafeJSONValue(child) {
				return false
			}
		}
	case []any:
		for _, child := range current {
			if !privacySafeJSONValue(child) {
				return false
			}
		}
	case string:
		lower := strings.ToLower(current)
		for _, forbidden := range []string{"must-not-export", "bearer ", "sk-", "ghp_", "github_pat_", "akia", "-----begin"} {
			if strings.Contains(lower, forbidden) {
				return false
			}
		}
	}
	return true
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func newAcceptanceInspectCommand(version Version) *cobra.Command {
	var packagePath string
	command := &cobra.Command{Use: "inspect", Short: "Validate an acceptance package produced by this exact qlog binary", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if err := inspectAcceptancePackage(packagePath, version); err != nil {
			return err
		}
		_, err := fmt.Fprintln(command.OutOrStdout(), "acceptance package: STRUCTURAL_VALID (authenticity: PENDING_EXTERNAL_REVIEW)")
		return err
	}}
	command.Flags().StringVar(&packagePath, "package", "", "acceptance ZIP package path")
	_ = command.MarkFlagRequired("package")
	return command
}

func inspectAcceptancePackage(path string, version Version) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
		return errors.New("acceptance package must be a non-empty regular non-symlink file")
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open acceptance package: %w", err)
	}
	defer func() { _ = archive.Close() }()
	entries := make(map[string][]byte, len(archive.File))
	var totalUncompressed uint64
	if len(archive.File) > acceptanceMaxPackageEntries {
		return fmt.Errorf("acceptance package has too many entries: %d", len(archive.File))
	}
	for _, file := range archive.File {
		if !validAcceptanceEntryName(file.Name) {
			return fmt.Errorf("invalid acceptance package entry name %q", file.Name)
		}
		if _, exists := entries[file.Name]; exists {
			return fmt.Errorf("duplicate acceptance package entry %q", file.Name)
		}
		if file.UncompressedSize64 > acceptanceMaxEntryBytes || totalUncompressed > acceptanceMaxTotalBytes-file.UncompressedSize64 {
			return fmt.Errorf("acceptance package exceeds size limits at %s", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, acceptanceMaxEntryBytes+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		if uint64(len(data)) > acceptanceMaxEntryBytes {
			return fmt.Errorf("acceptance package entry exceeds size limit: %s", file.Name)
		}
		totalUncompressed += uint64(len(data))
		entries[file.Name] = data
	}
	for name, data := range entries {
		if filepath.Ext(name) == ".json" {
			if err := rejectDuplicateJSONKeys(data); err != nil {
				return fmt.Errorf("invalid %s: %w", name, err)
			}
		}
	}
	for _, required := range []string{"manifest.json", "diagnostics.json", "collector.json", "report.json", "report.csv", "report.txt", "sessions.json", "SHA256SUMS"} {
		if _, found := entries[required]; !found {
			return fmt.Errorf("acceptance package missing %s", required)
		}
	}
	if len(entries) < acceptanceMinPackageEntries || len(entries) > acceptanceMaxPackageEntries {
		return fmt.Errorf("acceptance package has invalid entry count: %d", len(entries))
	}
	if err := verifyAcceptanceChecksums(entries); err != nil {
		return err
	}
	if err := rejectDuplicateJSONKeys(entries["manifest.json"]); err != nil {
		return err
	}
	var manifest acceptanceManifest
	decoder := json.NewDecoder(bytes.NewReader(entries["manifest.json"]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return err
	}
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported acceptance manifest schema version %d", manifest.SchemaVersion)
	}
	_, commit, binaryHash, platform, err := acceptanceRuntimeIdentity(version)
	if err != nil {
		return err
	}
	if manifest.Version != version.Version || !strings.EqualFold(manifest.Commit, commit) || !strings.EqualFold(manifest.CandidateBinarySHA256, binaryHash) || manifest.Platform != platform {
		return errors.New("acceptance package does not match the exact qlog runtime")
	}
	if manifest.CandidateAuthenticity != "PENDING_EXTERNAL_REVIEW" {
		return errors.New("acceptance package candidate authenticity is not pending external review")
	}
	if manifest.ImplementationStatus != acceptanceImplementationComplete {
		return errors.New("acceptance package implementation status is not complete")
	}
	if len(manifest.RealAgentEvidence) > 0 && len(entries) < acceptanceMinPackageEntriesWithEvidence {
		return fmt.Errorf("acceptance package with real-agent evidence has invalid entry count: %d", len(entries))
	}
	for name, expected := range manifest.Files {
		if !validAcceptanceEntryName(name) || name == "manifest.json" || name == "SHA256SUMS" {
			return fmt.Errorf("manifest references an invalid acceptance entry %q", name)
		}
		data, found := entries[name]
		if !found {
			return fmt.Errorf("manifest references missing acceptance entry %s", name)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
			return fmt.Errorf("manifest hash mismatch for %s", name)
		}
	}
	for name := range entries {
		if name == "manifest.json" || name == "SHA256SUMS" {
			continue
		}
		if _, found := manifest.Files[name]; !found {
			return fmt.Errorf("manifest omits acceptance entry %s", name)
		}
	}
	if len(manifest.RealAgentEvidence) > 0 {
		data, found := entries["real-agent-evidence.json"]
		if !found {
			return errors.New("manifest evidence is missing its package entry")
		}
		var packaged []acceptancecontract.RealAgentEvidence
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&packaged); err != nil {
			return err
		}
		manifestCanonical, _ := json.Marshal(manifest.RealAgentEvidence)
		packagedCanonical, _ := json.Marshal(packaged)
		if !bytes.Equal(manifestCanonical, packagedCanonical) {
			return errors.New("manifest and packaged real-agent evidence disagree")
		}
		seenBoundaries := map[string]bool{}
		seenAgents := map[string]bool{}
		var previousStarted, previousEnded time.Time
		for _, evidence := range packaged {
			if evidence.Platform != platform || !strings.EqualFold(evidence.CandidateBinarySHA256, binaryHash) || evidence.ReplayStatus != acceptancecontract.StatusPendingExternalE2E || seenBoundaries[evidence.BoundaryID] || seenAgents[evidence.AgentID] {
				return errors.New("packaged real-agent evidence has an invalid runtime, replay, or boundary binding")
			}
			if evidence.StartedAt.After(evidence.EndedAt) || (!previousStarted.IsZero() && evidence.StartedAt.Before(previousStarted)) || (!previousEnded.IsZero() && evidence.EndedAt.Before(previousEnded)) || evidence.EndedAt.After(manifest.GeneratedAt) || evidence.EndedAt.After(time.Now().UTC()) {
				return errors.New("packaged real-agent evidence timestamps are invalid or not chronological")
			}
			seenBoundaries[evidence.BoundaryID] = true
			seenAgents[evidence.AgentID] = true
			previousStarted, previousEnded = evidence.StartedAt, evidence.EndedAt
			evaluated, err := acceptancecontract.EvaluateRealAgentEvidenceForCandidate(evidence, canonicalCandidateTag(version.Version), commit)
			if err != nil || evaluated.Status != evidence.Status {
				return errors.New("packaged real-agent evidence status is not reproducible")
			}
		}
	}
	if err := validateAcceptanceManifestStatuses(manifest); err != nil {
		return err
	}
	var diagnostics acceptanceDiagnostics
	diagnosticsDecoder := json.NewDecoder(bytes.NewReader(entries["diagnostics.json"]))
	diagnosticsDecoder.DisallowUnknownFields()
	if err := diagnosticsDecoder.Decode(&diagnostics); err != nil {
		return errors.New("invalid acceptance diagnostics")
	}
	if (diagnostics.LedgerStatus != acceptancePass && diagnostics.LedgerStatus != acceptanceFail) || diagnostics.CollectorLogFingerprint == "" {
		return errors.New("acceptance diagnostics contains invalid status values")
	}
	if diagnostics.LedgerStatus == acceptanceFail && manifest.ExternalE2EStatus != acceptanceFail {
		return errors.New("ledger FAIL must override acceptance aggregate status")
	}
	if acceptancePackagePrivacyStatus(entries, manifest.RealAgentEvidence) != acceptancecontract.StatusPass {
		return errors.New("acceptance package privacy scan failed")
	}
	return nil
}

const (
	acceptanceMaxEntryBytes                 = 16 << 20
	acceptanceMaxTotalBytes                 = 64 << 20
	acceptanceMinPackageEntries             = 8
	acceptanceMinPackageEntriesWithEvidence = 9
	acceptanceMaxPackageEntries             = 10
)

func validAcceptanceEntryName(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.ContainsAny(name, "\x00\r\n\t") || filepath.IsAbs(name) || path.IsAbs(name) || strings.Contains(name, "..") {
		return false
	}
	allowed := map[string]bool{"collector.json": true, "diagnostics.json": true, "report.csv": true, "report.json": true, "report.txt": true, "sessions.json": true, "manifest.json": true, "SHA256SUMS": true, "real-agent-evidence.json": true}
	return allowed[name]
}

func validateAcceptanceManifestStatuses(manifest acceptanceManifest) error {
	const stableAgentCount = 5
	if len(manifest.Agents) != stableAgentCount {
		return fmt.Errorf("acceptance manifest requires exactly %d stable agents", stableAgentCount)
	}
	expected := map[string]bool{"claude-code": true, "codex": true, "copilot": true, "copilot-vscode": true, "opencode": true}
	seen := make(map[string]bool, len(manifest.Agents))
	for _, agent := range manifest.Agents {
		if !expected[agent.AdapterID] || seen[agent.AdapterID] {
			return fmt.Errorf("acceptance manifest has invalid stable agent matrix entry %q", agent.AdapterID)
		}
		seen[agent.AdapterID] = true
	}
	evidence := make([]acceptancecontract.RealAgentEvidence, len(manifest.RealAgentEvidence))
	copy(evidence, manifest.RealAgentEvidence)
	agents := applyRealAgentEvidence(append([]acceptanceAgentResult(nil), manifest.Agents...), evidence)
	for i := range agents {
		if agents[i].Status != manifest.Agents[i].Status || agents[i].Readiness != manifest.Agents[i].Readiness {
			return fmt.Errorf("acceptance agent status is not reproducible for %s", agents[i].AdapterID)
		}
	}
	if acceptanceReadiness(agents) != manifest.ReadinessStatus || acceptanceExternalStatus(agents) != manifest.ExternalE2EStatus {
		return errors.New("acceptance aggregate status is not reproducible")
	}
	return nil
}

func verifyAcceptanceChecksums(entries map[string][]byte) error {
	want := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(entries["SHA256SUMS"]))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || !isLowerHex(fields[0], 64) {
			return errors.New("invalid acceptance checksum manifest")
		}
		if !validAcceptanceEntryName(fields[1]) || fields[1] == "SHA256SUMS" {
			return fmt.Errorf("invalid acceptance checksum entry name %q", fields[1])
		}
		if _, duplicate := want[fields[1]]; duplicate {
			return errors.New("duplicate acceptance checksum entry")
		}
		want[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for name, data := range entries {
		if name == "SHA256SUMS" {
			continue
		}
		sum := sha256.Sum256(data)
		if want[name] != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("acceptance checksum mismatch for %s", name)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		return errors.New("acceptance checksum references a missing entry")
	}
	return nil
}
