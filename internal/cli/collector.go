package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/ingest/otlp"
	"github.com/janpereira-dev/quantum_log/internal/ingest/qlogevent"
	"github.com/spf13/cobra"
)

var collectorIngestMu sync.Mutex

func newCollectorCommand(home *string) *cobra.Command {
	collector := &cobra.Command{Use: "collector", Short: "Receive local telemetry"}
	var listen string
	var allowNonLoopback bool
	var jsonOutput bool
	var logFile string
	var fallbackState string
	status := &cobra.Command{Use: "status", Short: "Show managed collector status", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		output, err := collectorStatus(command.Context(), *home, listen, command.Flags().Changed("home"), command.Flags().Changed("listen"), newCollectorManager())
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), output)
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "collector: service=%s installed=%t running=%t reachable=%t listen=%s health=%s\n", output.ServiceID, output.Installed, output.Running, output.Reachable, output.Listen, output.Message)
		return err
	}}
	status.Flags().StringVar(&listen, "listen", "127.0.0.1:4318", "OTLP/HTTP listen address")
	status.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	serve := &cobra.Command{Use: "serve", Short: "Serve OTLP/HTTP JSON/protobuf traces and logs", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if err := validateListenAddress(listen, allowNonLoopback); err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		if err := service.Close(); err != nil {
			return err
		}
		output, closeLog, err := collectorServeOutput(command, logFile)
		if err != nil {
			return err
		}
		defer func() { _ = closeLog() }()
		if err := recordCollectorFallbackProcess(fallbackState, *home, listen, logFile); err != nil {
			return err
		}
		server := &http.Server{Addr: listen, Handler: newCollectorMux(*home), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
		_, err = fmt.Fprintf(output, "qlog collector listening on http://%s (/v1/traces and /v1/logs OTLP JSON/protobuf, /v1/events qlog JSON)\n", listen)
		if err != nil {
			return err
		}
		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			_, _ = fmt.Fprintf(output, "collector server stopped: %v\n", err)
			return err
		}
		return nil
	}}
	serve.Flags().StringVar(&listen, "listen", "127.0.0.1:4318", "OTLP/HTTP listen address")
	serve.Flags().BoolVar(&allowNonLoopback, "allow-non-loopback", false, "allow a non-loopback listen address")
	serve.Flags().StringVar(&logFile, "log-file", "", "append collector startup and error messages to this qlog-owned log file")
	serve.Flags().StringVar(&fallbackState, "fallback-state", "", "internal user fallback state path")
	_ = serve.Flags().MarkHidden("fallback-state")
	collector.AddCommand(
		status,
		serve,
		collectorLifecycleCommand("install", "Install managed collector", func(manager collectorManager, home, listen string) (CollectorStatus, error) {
			return manager.Install(home, listen)
		}, home, &listen),
		collectorLifecycleCommand("start", "Start managed collector", func(manager collectorManager, home, listen string) (CollectorStatus, error) {
			return manager.Start(home, listen)
		}, home, &listen),
		collectorLifecycleCommand("stop", "Stop managed collector", func(manager collectorManager, _, _ string) (CollectorStatus, error) { return manager.Stop() }, home, &listen),
		collectorLifecycleCommand("restart", "Restart managed collector", func(manager collectorManager, home, listen string) (CollectorStatus, error) {
			return restartCollectorAfterSchedulerDenied(manager, home, listen)
		}, home, &listen),
		collectorLogsCommand(),
		collectorLifecycleCommand("uninstall", "Uninstall managed collector", func(manager collectorManager, _, _ string) (CollectorStatus, error) { return manager.Uninstall() }, home, &listen),
	)
	return collector
}

func collectorStatus(ctx context.Context, home, listen string, homeExplicit, listenExplicit bool, manager collectorManager) (CollectorStatus, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return CollectorStatus{}, err
	}
	resolvedHome, resolvedListen := resolveManagedCollectorSettings(manager, paths.Home, listen, homeExplicit, listenExplicit)
	finalPaths, err := config.Resolve(resolvedHome)
	if err != nil {
		return CollectorStatus{}, err
	}
	output, err := manager.Status(ctx, resolvedListen)
	if err != nil {
		return CollectorStatus{}, err
	}
	output.Home = finalPaths.Home
	output.Database = finalPaths.Database
	output.Endpoints = []string{"/v1/traces", "/v1/logs", "/v1/events", "/healthz"}
	output.Scope = "loopback-only by default"
	if output.Health == "" {
		output.Health = output.Message
	}
	return output, nil
}

func newCollectorMux(home string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/v1/traces", requestScopedHandler{home: home, build: otlp.NewHandler})
	mux.Handle("/v1/logs", requestScopedHandler{home: home, build: otlp.NewHandler})
	mux.Handle("/v1/events", requestScopedHandler{home: home, build: qlogevent.NewHandler})
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "method must be GET or HEAD", http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		if request.Method == http.MethodGet {
			_, _ = writer.Write([]byte(`{"service":"quantum-log-collector","status":"ok"}`))
		}
	})
	return mux
}

type collectorHealth struct {
	Reachable bool
	Running   bool
	Managed   bool
	Health    string
}

// CollectorStatus separates persistent-service state from collector health.
type CollectorStatus struct {
	Installed     bool     `json:"installed"`
	Running       bool     `json:"running"`
	Reachable     bool     `json:"reachable"`
	Mode          string   `json:"mode"`
	Listen        string   `json:"listen"`
	ServiceID     string   `json:"service_id"`
	StatePath     string   `json:"state_path"`
	LogPath       string   `json:"log_path"`
	Message       string   `json:"message"`
	Health        string   `json:"health,omitempty"`
	Home          string   `json:"home,omitempty"`
	Database      string   `json:"database,omitempty"`
	Endpoints     []string `json:"endpoints,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	ManagedHealth bool     `json:"-"`
}

func probeCollectorHealth(ctx context.Context, listen string) collectorHealth {
	probeCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, "http://"+listen+"/healthz", nil)
	if err != nil {
		return collectorHealth{Health: err.Error()}
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return collectorHealth{Health: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return collectorHealth{Health: response.Status}
	}
	health := collectorHealth{Reachable: true, Running: true, Health: "ok"}
	var payload struct {
		Service string `json:"service"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&payload); err == nil && payload.Service == "quantum-log-collector" && payload.Status == "ok" {
		health.Managed = true
	}
	return health
}

func collectorStatusWithHealth(status CollectorStatus, health collectorHealth) CollectorStatus {
	status.Reachable = health.Reachable
	status.ManagedHealth = health.Managed
	status.Health = health.Health
	switch {
	case !status.Installed && !health.Reachable:
		status.Message = "collector is not installed; run qlog collector install or qlog setup --yes"
	case status.Installed && !health.Reachable:
		status.Message = "collector is installed but not reachable; run qlog collector restart or qlog collector logs"
	default:
		status.Message = health.Health
	}
	return status
}

type requestScopedHandler struct {
	home  string
	build func(*app.Service) http.Handler
}

func (h requestScopedHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	collectorIngestMu.Lock()
	defer collectorIngestMu.Unlock()
	service, err := app.Open(request.Context(), h.home)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = service.Close() }()
	h.build(service).ServeHTTP(writer, request)
}

type collectorManager interface {
	Install(home, listen string) (CollectorStatus, error)
	Start(home, listen string) (CollectorStatus, error)
	Stop() (CollectorStatus, error)
	Restart(home, listen string) (CollectorStatus, error)
	Status(ctx context.Context, listen string) (CollectorStatus, error)
	Logs() (string, error)
	Uninstall() (CollectorStatus, error)
}

func collectorLifecycleCommand(name, short string, run func(collectorManager, string, string) (CollectorStatus, error), home *string, listen *string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		resolvedHome, err := resolveCollectorLifecycleHome(*home)
		if err != nil {
			return err
		}
		manager := newCollectorManager()
		resolvedHome, resolvedListen := resolveManagedCollectorSettings(manager, resolvedHome, *listen, command.Flags().Changed("home"), command.Flags().Changed("listen"))
		status, err := run(manager, resolvedHome, resolvedListen)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), status)
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), status.Message)
		return err
	}}
	command.Flags().StringVar(listen, "listen", defaultCollectorListen, "OTLP/HTTP listen address")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

type managedCollectorSettingsResolver interface {
	ResolveManagedCollectorSettings(home, listen string, homeExplicit, listenExplicit bool) (string, string)
}

func resolveManagedCollectorSettings(manager collectorManager, home, listen string, homeExplicit, listenExplicit bool) (string, string) {
	resolver, ok := manager.(managedCollectorSettingsResolver)
	if !ok {
		return home, listen
	}
	return resolver.ResolveManagedCollectorSettings(home, listen, homeExplicit, listenExplicit)
}

func resolveCollectorLifecycleHome(home string) (string, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return "", err
	}
	return paths.Home, nil
}

func collectorLogsCommand() *cobra.Command {
	return &cobra.Command{Use: "logs", Short: "Show managed collector logs", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		logs, err := newCollectorManager().Logs()
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(command.Root().OutOrStdout(), logs)
		return err
	}}
}

func collectorServeOutput(command *cobra.Command, logFile string) (io.Writer, func() error, error) {
	output := command.Root().OutOrStdout()
	if logFile == "" {
		return output, func() error { return nil }, nil
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create collector log directory: %w", err)
	}
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open collector log: %w", err)
	}
	return io.MultiWriter(output, file), file.Close, nil
}

func validateListenAddress(address string, allowNonLoopback bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", address, err)
	}
	if allowNonLoopback || host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing non-loopback listener %q without --allow-non-loopback", address)
	}
	return nil
}

func validateCollectorListen(listen string) error {
	return validateListenAddress(listen, false)
}

func validateCollectorExecutable(executable string) error {
	path := strings.ToLower(strings.ReplaceAll(executable, `\`, "/"))
	if strings.HasSuffix(path, ".test") || strings.HasSuffix(path, ".test.exe") || strings.Contains(path, "/go-build") {
		return fmt.Errorf("cannot install managed collector from transient executable %q; build or install a durable qlog.exe, then run that binary to install the managed collector", executable)
	}
	return nil
}

// collectorStartupStatus preserves service state when startup exceeds the bounded
// readiness window, while making a ready collector explicit to callers.
func collectorStartupStatus(ctx context.Context, status CollectorStatus, check func(context.Context) (CollectorStatus, error)) (CollectorStatus, error) {
	ready, err := pollCollectorReadiness(ctx, check)
	if err == nil {
		ready.Message = "collector started and ready"
		return ready, nil
	}
	status.Message = "collector start requested; readiness=" + err.Error()
	return status, nil
}
