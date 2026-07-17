package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// GenericJSONL imports caller-supplied normalized records. It does not infer metrics.
type GenericJSONL struct{}

func (GenericJSONL) Descriptor() Descriptor {
	return Descriptor{ID: "generic-jsonl", Name: "Generic JSONL", Version: "1", Capabilities: Capabilities{StructuredEvents: true}}
}

func (GenericJSONL) Detect(context.Context) (Detection, error) {
	return Detection{Available: true, Evidence: "built-in JSONL importer"}, nil
}

func (GenericJSONL) Install(_ context.Context, _ InstallOptions) (InstallResult, error) {
	return InstallResult{Actions: []string{"no installation required"}}, nil
}

func (GenericJSONL) Uninstall(_ context.Context, _ InstallOptions) (InstallResult, error) {
	return InstallResult{Actions: []string{"no installation required"}}, nil
}

func (GenericJSONL) HealthCheck(context.Context) error { return nil }

func (GenericJSONL) Ingest(_ context.Context, reader io.Reader) ([]RawRecord, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	records := make([]RawRecord, 0)
	for line := 1; scanner.Scan(); line++ {
		payload := append([]byte(nil), scanner.Bytes()...)
		if len(payload) == 0 {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, fmt.Errorf("parse JSONL line %d: %w", line, err)
		}
		records = append(records, RawRecord{Source: "generic-jsonl", Payload: payload})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read JSONL: %w", err)
	}
	return records, nil
}

func (GenericJSONL) Normalize(record RawRecord) (RawRecord, error) { return record, nil }

func (GenericJSONL) ExtractProjectSignals(RawRecord) ProjectSignals { return ProjectSignals{} }
