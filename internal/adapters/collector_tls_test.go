package adapters

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func TestOpenCodeRuntimePinsCollectorBeforeSendingToken(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required for real plugin transport")
	}
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	if _, err := newOpenCodeAdapter().Install(context.Background(), InstallOptions{Home: home}); err != nil {
		t.Fatal(err)
	}
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := config.CollectorServerTLS(paths, "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	token, err := config.CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	var validCalls, foreignCalls, plainCalls atomic.Int32
	headers := make(chan string, 8)
	valid := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validCalls.Add(1)
		headers <- r.Header.Get("X-Qlog-Token")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	valid.TLS = identity
	valid.Config.ErrorLog = log.New(io.Discard, "", 0)
	valid.StartTLS()
	defer valid.Close()
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	foreign.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer foreign.Close()
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { plainCalls.Add(1) }))
	defer plain.Close()
	redirect := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validCalls.Add(1)
		headers <- r.Header.Get("X-Qlog-Token")
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	redirect.TLS = identity.Clone()
	redirect.StartTLS()
	defer redirect.Close()
	installed := string(mustReadFile(t, newOpenCodeAdapter().pluginPath()))
	if strings.Contains(installed, "http://127.0.0.1:4318") || !strings.Contains(installed, "https://127.0.0.1:4318") {
		t.Fatal("installed plugin must pin HTTPS loopback")
	}
	for _, scenario := range []struct {
		name, url, missing string
		wantValid          int32
	}{
		{"trusted collector", valid.URL, "", 1}, {"foreign CA squatter", foreign.URL, "", 0},
		{"plain HTTP squatter", "https://" + strings.TrimPrefix(plain.URL, "http://"), "", 0},
		{"redirect not followed", redirect.URL, "", 1}, {"missing CA", valid.URL, "ca", 0}, {"missing HMAC key", valid.URL, "key", 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			before := validCalls.Load()
			source := strings.ReplaceAll(installed, "https://127.0.0.1:4318", scenario.url)
			if scenario.missing == "ca" {
				source = strings.ReplaceAll(source, strconv.Quote(config.CollectorCertificatePath(paths)), strconv.Quote(filepath.Join(home, "missing-ca.pem")))
			}
			if scenario.missing == "key" {
				source = strings.ReplaceAll(source, strconv.Quote(paths.PromptHashKey), strconv.Quote(filepath.Join(home, "missing-key")))
			}
			plugin := filepath.Join(t.TempDir(), "plugin.ts")
			if err := os.WriteFile(plugin, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			harness := filepath.Join(t.TempDir(), "run.mjs")
			script := `import {pathToFileURL} from 'node:url';
process.env.QLOG_COLLECTOR_URL='https://untrusted.invalid/events';
process.env.NODE_TLS_REJECT_UNAUTHORIZED='0';
const {QuantumLogPlugin}=await import(pathToFileURL(process.env.QLOG_TEST_PLUGIN));
const plugin=await QuantumLogPlugin({directory:'/synthetic'});
await plugin.event({event:{type:'message.updated',properties:{info:{role:'user',id:'synthetic-message',sessionID:'synthetic-session'}}}});
`
			if err := os.WriteFile(harness, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, node, "--experimental-strip-types", harness)
			command.Env = append(os.Environ(), "QLOG_TEST_PLUGIN="+plugin)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("plugin transport: %v %s", err, output)
			}
			if got := validCalls.Load() - before; got != scenario.wantValid {
				t.Fatalf("authenticated requests=%d want=%d", got, scenario.wantValid)
			}
			if scenario.wantValid > 0 {
				select {
				case got := <-headers:
					if got != token {
						t.Fatal("collector received wrong token")
					}
				default:
					t.Fatal("collector header missing")
				}
			}
			if foreignCalls.Load() != 0 || plainCalls.Load() != 0 {
				t.Fatal("untrusted receiver obtained HTTP request/token")
			}
		})
	}
}
