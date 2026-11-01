package cli

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/spf13/cobra"
)

func TestTLSPlaintextPortSquatterReceivesNoCredentialsOrBody(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	if err := config.Ensure(paths); err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureCollectorTLS(paths, "127.0.0.1:4318"); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	squatter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer squatter.Close()
	for _, scheme := range []string{"http://", "https://"} {
		t.Run(scheme, func(t *testing.T) {
			t.Setenv("QLOG_COLLECTOR_URL", scheme+strings.TrimPrefix(squatter.URL, "http://")+"/v1/events")
			command := &cobra.Command{}
			command.SetContext(context.Background())
			event, err := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic","timestamp":1760000000000,"prompt":"MOCK_PRIVATE_PROMPT"}`), "userPromptSubmitted")
			if err != nil {
				t.Fatal(err)
			}
			err = ingestOrForwardHook(command, &home, event)
			if requests.Load() != 0 || err == nil {
				t.Fatalf("plaintext squatter received %d requests; rejection=%v", requests.Load(), err)
			}
		})
	}
}

func newCollectorTLSTestServer(t *testing.T, home string, handler http.Handler) *httptest.Server {
	t.Helper()
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := config.CollectorServerTLS(paths, "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = identity
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func TestTLSWrongHomeOrAmbientTrustedCertificateReceivesNoHTTPRequest(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	if err := config.EnsureCollectorTLS(paths, "127.0.0.1:4318"); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	attacker := newCollectorTLSTestServer(t, t.TempDir(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	// A process-wide transport that trusts the impostor must not override the pin.
	previousTransport := http.DefaultTransport
	http.DefaultTransport = attacker.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	t.Setenv("QLOG_COLLECTOR_URL", attacker.URL+"/v1/events")
	command := &cobra.Command{}
	command.SetContext(context.Background())
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic","timestamp":1760000000000,"prompt":"MOCK_PRIVATE_PROMPT"}`), "userPromptSubmitted")
	if err := ingestOrForwardHook(command, &home, event); err == nil {
		t.Fatal("foreign installation certificate accepted")
	}
	if requests.Load() != 0 {
		t.Fatal("impostor received HTTP credentials or body")
	}
	health := probeCollectorHealth(context.Background(), home, strings.TrimPrefix(attacker.URL, "https://"))
	if health.Reachable || health.Managed {
		t.Fatalf("health trusted foreign certificate: %+v", health)
	}
	if reachable, _ := verifyCollectorReachability(context.Background(), home); reachable {
		t.Fatal("adapter health trusted foreign certificate")
	}
	if requests.Load() != 0 {
		t.Fatal("health sent HTTP to foreign identity")
	}
}

func TestTLSHookKeepsNormalHostnameVerification(t *testing.T) {
	home := t.TempDir()
	var requests atomic.Int64
	server := newCollectorTLSTestServer(t, home, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNoContent) }))
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	previousTransport := http.DefaultTransport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previousTransport; transport.CloseIdleConnections() }()
	t.Setenv("QLOG_COLLECTOR_URL", "https://127.0.0.2:"+port+"/v1/events")
	command := &cobra.Command{}
	command.SetContext(context.Background())
	event, _ := copilotCLIHookEvent([]byte(`{"sessionId":"synthetic"}`), "agentStop")
	if err := ingestOrForwardHook(command, &home, event); err == nil {
		t.Fatal("certificate hostname mismatch accepted")
	}
	if requests.Load() != 0 {
		t.Fatal("hostname mismatch received HTTP credentials/body")
	}
}

func TestTLSCollectorRejectsV1CredentialOverValidPinnedTLS(t *testing.T) {
	home := t.TempDir()
	paths, _ := config.Resolve(home)
	server := newCollectorTLSTestServer(t, home, newCollectorMux(home))
	key, err := config.PromptHashKey(paths)
	if err != nil {
		t.Fatal(err)
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte("qlog-collector-auth-v1"))
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/events", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Qlog-Token", hex.EncodeToString(digest.Sum(nil)))
	request.Header.Set("Content-Type", "application/json")
	client, err := collectorHTTPSClient(home, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old credential status=%d", response.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(home, "qlog.db")); !os.IsNotExist(err) {
		t.Fatal("rejected old credential opened ledger")
	}
}
