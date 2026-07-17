package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAdapterCommandsExposeCapabilitiesAndSafeDryRun(t *testing.T) {
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	output, err := run("adapter", "list", "--json")
	if err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("adapter list = %q, %v", output, err)
	}
	output, err = run("adapter", "install", "opencode", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("adapter install dry run: %v", err)
	}
	var result struct {
		Changed bool `json:"changed"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil || result.Changed {
		t.Fatalf("dry-run result = %q, %#v, %v", output, result, err)
	}
}

func TestCollectorRejectsPublicBindingWithoutExplicitOptIn(t *testing.T) {
	if err := validateListenAddress("0.0.0.0:4318", false); err == nil {
		t.Fatal("public binding was accepted")
	}
	if err := validateListenAddress("127.0.0.1:4318", false); err != nil {
		t.Fatalf("loopback binding rejected: %v", err)
	}
}
