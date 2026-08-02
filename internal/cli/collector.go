package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	status := &cobra.Command{Use: "status", Short: "Show managed collector status", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		paths, err := config.Resolve(*home)
		if err != nil {
			return err
		}
		output, err := newCollectorManager().Status(command.Context(), listen)
		if err != nil {
			return err
		}
		output.Home = paths.Home
		output.Database = paths.Database
		output.Endpoints = []string{"/v1/traces", "/v1/logs", "/v1/events", "/healthz"}
		output.Scope = "loopback-only by default"
		output.Health = output.Message
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
		server := &http.Server{Addr: listen, Handler: newCollectorMux(*home), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "qlog collector listening on http://%s (/v1/traces and /v1/logs OTLP JSON/protobuf, /v1/events qlog JSON)\n", listen)
		if err != nil {
			return err
		}
		return server.ListenAndServe()
	}}
	serve.Flags().StringVar(&listen, "listen", "127.0.0.1:4318", "OTLP/HTTP listen address")
	serve.Flags().BoolVar(&allowNonLoopback, "allow-non-loopback", false, "allow a non-loopback listen address")
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
			return manager.Restart(home, listen)
		}, home, &listen),
		collectorLogsCommand(),
		collectorLifecycleCommand("uninstall", "Uninstall managed collector", func(manager collectorManager, _, _ string) (CollectorStatus, error) { return manager.Uninstall() }, home, &listen),
	)
	return collector
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
			_, _ = writer.Write([]byte(`{"status":"ok"}`))
		}
	})
	return mux
}

type collectorHealth struct {
	Reachable bool
	Running   bool
	Health    string
}

// CollectorStatus separates persistent-service state from collector health.
type CollectorStatus struct {
	Installed bool     `json:"installed"`
	Running   bool     `json:"running"`
	Reachable bool     `json:"reachable"`
	Listen    string   `json:"listen"`
	ServiceID string   `json:"service_id"`
	StatePath string   `json:"state_path"`
	LogPath   string   `json:"log_path"`
	Message   string   `json:"message"`
	Health    string   `json:"health,omitempty"`
	Home      string   `json:"home,omitempty"`
	Database  string   `json:"database,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
	Scope     string   `json:"scope,omitempty"`
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
	health := collectorHealth{Reachable: true, Running: true, Health: response.Status}
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		health.Health = "ok"
	}
	return health
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
		status, err := run(newCollectorManager(), *home, *listen)
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
