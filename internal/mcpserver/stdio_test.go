package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
)

func TestMCPStdio(t *testing.T) {
	const homeEnv = "QLOG_TEST_MCP_STDIO_HOME"
	if home := os.Getenv(homeEnv); home != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := Run(ctx, home, "stdio-test"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0) // Keep the Go test runner's output off the protocol stream.
	}

	home := t.TempDir()
	service, err := app.Initialize(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestMCPStdio$")
	cmd.Env = append(os.Environ(), homeEnv+"="+home)
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdout.Close() }()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		cancel()
		if !waited {
			_ = cmd.Wait()
		}
		if t.Failed() {
			t.Logf("MCP server stderr: %s", &stderr)
		}
	}()
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(stdout)
	send := func(message string) {
		t.Helper()
		if err := encoder.Encode(json.RawMessage(message)); err != nil {
			t.Fatal(err)
		}
	}
	receive := func(id int, result any) {
		t.Helper()
		for {
			var response struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      *int            `json:"id"`
				Error   json.RawMessage `json:"error"`
				Result  json.RawMessage `json:"result"`
			}
			if err := decoder.Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.ID == nil {
				continue
			}
			if response.JSONRPC != "2.0" || *response.ID != id || len(response.Error) != 0 {
				t.Fatalf("unexpected JSON-RPC response: %+v", response)
			}
			if err := json.Unmarshal(response.Result, result); err != nil {
				t.Fatal(err)
			}
			return
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"raw-stdio-test","version":"1"}}}`)
	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	receive(1, &initialized)
	if initialized.ProtocolVersion != "2025-11-25" || initialized.ServerInfo.Name != "quantum-log" || initialized.ServerInfo.Version != "stdio-test" {
		t.Fatalf("unexpected initialization: %+v", initialized)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	receive(2, &listed)
	expected := map[string]bool{
		"register_project": false, "get_current_project": false, "switch_project": false,
		"add_project_tag": false, "start_task": false, "finish_task": false,
		"get_project_summary": false, "get_unattributed_summary": false, "assign_usage": false,
		"split_usage": false, "allocation_history": false, "revert_allocation": false,
	}
	for _, tool := range listed.Tools {
		if found, known := expected[tool.Name]; !known || found {
			t.Fatalf("unexpected or duplicate tool %q", tool.Name)
		}
		expected[tool.Name] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing tool %q", name)
		}
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("MCP server did not exit cleanly on EOF: %v", err)
	}
}
