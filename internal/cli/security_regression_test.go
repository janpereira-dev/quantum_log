package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/spf13/cobra"
)

func TestSecurityHookForwardingNeverSendsPromptContent(t *testing.T) {
	for _, mode := range []string{"off", "hash", "full"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			paths, _ := config.Resolve(home)
			if err := config.SetPromptCaptureMode(paths, mode); err != nil {
				t.Fatal(err)
			}
			var received []byte
			var receivedToken string
			server := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received, _ = io.ReadAll(r.Body)
				receivedToken = r.Header.Get("X-Qlog-Token")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
			event, err := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic","timestamp":1760000000000,"prompt":"MOCK_PRIVATE_PROMPT"}`), "userPromptSubmitted")
			if err != nil {
				t.Fatal(err)
			}
			command := &cobra.Command{}
			command.SetContext(context.Background())
			if err := ingestOrForwardHook(command, &home, event); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(received), "MOCK_PRIVATE_PROMPT") || strings.Contains(string(received), `"prompt":`) || strings.Contains(string(received), "interaction_redacted") {
				t.Fatalf("prompt content left producer in mode %s: %s", mode, received)
			}
			wantToken, err := config.CollectorToken(paths)
			if err != nil || receivedToken != wantToken {
				t.Fatalf("forwarding authentication missing: err=%v", err)
			}
		})
	}
}

func TestSecurityCopilotFallbackIdentityDoesNotFingerprintPrompt(t *testing.T) {
	first, err := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic","timestamp":1760000000000,"prompt":"first private prompt"}`), "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	second, err := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic","timestamp":1760000000000,"prompt":"second private prompt"}`), "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	if first.UpstreamEventID != second.UpstreamEventID {
		t.Fatal("fallback event identity depends on prompt content")
	}
	digest := sha256.Sum256([]byte("synthetic\x001760000000000\x00first private prompt"))
	if strings.Contains(first.UpstreamEventID, hex.EncodeToString(digest[:])) {
		t.Fatal("public prompt fingerprint retained")
	}
}

func TestSecurityClaudeFallbackIdentityDoesNotFingerprintPrompt(t *testing.T) {
	first, err := claudeCodeHookEvent([]byte(`{"session_id":"synthetic","hook_event_name":"UserPromptSubmit","timestamp":"2026-11-01T12:00:00Z","prompt":"first private prompt"}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := claudeCodeHookEvent([]byte(`{"session_id":"synthetic","hook_event_name":"UserPromptSubmit","timestamp":"2026-11-01T12:00:00Z","prompt":"second private prompt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if first.UpstreamEventID != second.UpstreamEventID {
		t.Fatal("Claude fallback event identity depends on prompt content")
	}
}

func TestSecurityCopilotMissingTimestampDoesNotCollapseSession(t *testing.T) {
	input := []byte(`{"sessionId":"synthetic","prompt":"same private prompt"}`)
	first, err := copilotCLIHookEvent(input, "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	second, err := copilotCLIHookEvent(input, "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	if first.UpstreamEventID == second.UpstreamEventID {
		t.Fatal("all timestamp-less session prompts share one fallback identity")
	}
}

func TestSecurityCollectorRejectsUnauthenticatedIngestionBeforeOpeningLedger(t *testing.T) {
	for _, endpoint := range []string{"/v1/events", "/v1/traces", "/v1/logs"} {
		t.Run(endpoint, func(t *testing.T) {
			home := t.TempDir()
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1"+endpoint, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			newCollectorMux(home).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			if _, err := os.Stat(filepath.Join(home, "qlog.db")); !os.IsNotExist(err) {
				t.Fatal("unauthenticated request opened ledger")
			}
		})
	}
}

func TestSecurityHookDoesNotFollowRedirect(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	if err := config.Ensure(paths); err != nil {
		t.Fatal(err)
	}
	called := false
	target := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
	defer target.Close()
	redirect := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	t.Setenv("QLOG_COLLECTOR_URL", redirect.URL)
	command := &cobra.Command{}
	command.SetContext(context.Background())
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic"}`), "agentStop")
	err := ingestOrForwardHook(command, &home, event)
	if err == nil || called {
		t.Fatalf("redirect followed: called=%t err=%v", called, err)
	}
}

func TestSecurityHookRejectsNonLoopbackDestination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("QLOG_COLLECTOR_URL", "http://192.0.2.1:4318/v1/events")
	original := http.DefaultTransport
	http.DefaultTransport = &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("regression test blocks all network dialing")
	}}
	defer func() { http.DefaultTransport = original }()
	command := &cobra.Command{}
	command.SetContext(context.Background())
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic"}`), "agentStop")
	err := ingestOrForwardHook(command, &home, event)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("destination not rejected before network: %v", err)
	}
}

func TestSecurityCollectorAuthenticationAndOrigin(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	token, err := config.CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	otherPaths, _ := config.Resolve(t.TempDir())
	otherToken, err := config.CollectorToken(otherPaths)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, host, origin, token, listen string
		status                            int
	}{
		{name: "missing", host: "127.0.0.1:4318", status: http.StatusUnauthorized},
		{name: "wrong", host: "127.0.0.1:4318", token: "wrong", status: http.StatusUnauthorized},
		{name: "other home", host: "127.0.0.1:4318", token: otherToken, status: http.StatusUnauthorized},
		{name: "valid", host: "127.0.0.1:4318", token: token, status: http.StatusNoContent},
		{name: "foreign Host", host: "attacker.example:4318", token: token, status: http.StatusForbidden},
		{name: "foreign Origin", host: "127.0.0.1:4318", origin: "http://attacker.example:4318", token: token, status: http.StatusForbidden},
		{name: "same Origin", host: "127.0.0.1:4318", origin: "http://127.0.0.1:4318", token: token, status: http.StatusNoContent},
		{name: "explicit remote listener", host: "192.0.2.10:4318", listen: "192.0.2.10:4318", token: token, status: http.StatusNoContent},
		{name: "wildcard IP listener", host: "192.0.2.10:4318", listen: "0.0.0.0:4318", token: token, status: http.StatusNoContent},
		{name: "wildcard foreign Host", host: "attacker.example:4318", listen: "0.0.0.0:4318", token: token, status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://"+test.host+"/v1/events", strings.NewReader("{}"))
			request.Header.Set("X-Qlog-Token", test.token)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			handler := authenticatedCollector(home, test.listen, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func authorizeCollectorRequest(t *testing.T, request *http.Request, home string) {
	t.Helper()
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	token, err := config.CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Qlog-Token", token)
}

func TestSecurityHookAuthenticatesCustomHomeCollector(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runQLog(t, home, "config", "set", "prompt-capture", "off"); err != nil {
		t.Fatal(err)
	}
	server := newCollectorTLSTestServer(t, home, newCollectorMux(home))
	defer server.Close()
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
	input := `{"sessionId":"custom-home-session","timestamp":1760000000000,"prompt":"MOCK_PRIVATE_PROMPT"}`
	output, err := runQLogWithInput(t, home, strings.NewReader(input), "hook", "copilot-cli", "--event", "userPromptSubmitted")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "hook ignored") {
		t.Fatalf("authenticated capture was silently discarded: %s", output)
	}
	server.Close() // Wait for the handler's deferred checkpoint before independent inspection.
	reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(home, "qlog.db"))+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var mode, fingerprint, content, upstream string
	if err := reader.QueryRow(`SELECT prompt_capture_mode, prompt_hash, prompt_redacted, upstream_id FROM interactions WHERE session_id='custom-home-session'`).Scan(&mode, &fingerprint, &content, &upstream); err != nil {
		t.Fatal(err)
	}
	if mode != "off" || fingerprint != "" || content != "" || upstream == "" {
		t.Fatalf("off capture was not preserved: mode=%s hash=%q content=%q upstream=%q", mode, fingerprint, content, upstream)
	}
}

func TestSecurityHookUsesExistingFullBudgetForTrustedTLSReceiver(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	if err := config.SetPromptCaptureMode(paths, "off"); err != nil {
		t.Fatal(err)
	}
	received := make(chan string, 1)
	server := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
		time.Sleep(650 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
	command := &cobra.Command{}
	command.SetContext(context.Background())
	var output bytes.Buffer
	command.SetErr(&output)
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic-slow-receiver","timestamp":1760000000000,"prompt":"MOCK_PRIVATE_PROMPT"}`), "userPromptSubmitted")
	if err := bestEffortHook(command, &home, event); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "hook ignored") {
		t.Fatalf("valid receiver was cut off before the existing two-second budget: %s", output.String())
	}
	body := <-received
	if strings.Contains(body, "MOCK_PRIVATE_PROMPT") || strings.Contains(body, `"interaction_hash"`) || !strings.Contains(body, `"prompt_capture_mode":"off"`) {
		t.Fatalf("off policy changed during delayed delivery: %s", body)
	}
}

func TestSecurityHookRespectsEarlierCallerDeadline(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	if err := config.SetPromptCaptureMode(paths, "off"); err != nil {
		t.Fatal(err)
	}
	server := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	command := &cobra.Command{}
	command.SetContext(ctx)
	var output bytes.Buffer
	command.SetErr(&output)
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic-cancel"}`), "agentStop")
	if err := bestEffortHook(command, &home, event); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || !strings.Contains(output.String(), "hook ignored") {
		t.Fatalf("earlier caller deadline not honored: ctx=%v output=%s", ctx.Err(), output.String())
	}
}
