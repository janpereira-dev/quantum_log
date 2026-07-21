package cli

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/otlp"
	"github.com/janpereira-dev/quantum_log/internal/ingest/qlogevent"
	"github.com/spf13/cobra"
)

func newCollectorCommand(home *string) *cobra.Command {
	collector := &cobra.Command{Use: "collector", Short: "Receive local telemetry"}
	var listen string
	var allowNonLoopback bool
	var jsonOutput bool
	status := &cobra.Command{Use: "status", Short: "Show local collector endpoints", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		output := map[string]any{
			"listen":    listen,
			"endpoints": []string{"/v1/traces", "/v1/events"},
			"scope":     "loopback-only by default",
		}
		if jsonOutput {
			return writeJSON(command.Root().OutOrStdout(), output)
		}
		_, err := fmt.Fprintf(command.Root().OutOrStdout(), "collector: http://%s (/v1/traces OTLP JSON, /v1/events qlog JSON)\n", listen)
		return err
	}}
	status.Flags().StringVar(&listen, "listen", "127.0.0.1:4318", "OTLP/HTTP listen address")
	status.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	serve := &cobra.Command{Use: "serve", Short: "Serve OTLP/HTTP JSON traces", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if err := validateListenAddress(listen, allowNonLoopback); err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		mux := http.NewServeMux()
		mux.Handle("/v1/traces", otlp.NewHandler(service))
		mux.Handle("/v1/events", qlogevent.NewHandler(service))
		server := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "qlog collector listening on http://%s (/v1/traces OTLP JSON, /v1/events qlog JSON)\n", listen)
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
		collectorLifecycleCommand("install", "Install managed collector", func(manager collectorManager, home, listen string) (string, error) {
			return manager.Install(home, listen)
		}, home, &listen),
		collectorLifecycleCommand("start", "Start managed collector", func(manager collectorManager, home, listen string) (string, error) {
			return manager.Start(home, listen)
		}, home, &listen),
		collectorLifecycleCommand("stop", "Stop managed collector", func(manager collectorManager, _, _ string) (string, error) { return manager.Stop() }, home, &listen),
		collectorLifecycleCommand("restart", "Restart managed collector", func(manager collectorManager, home, listen string) (string, error) {
			return manager.Restart(home, listen)
		}, home, &listen),
		collectorLifecycleCommand("logs", "Show managed collector logs", func(manager collectorManager, _, _ string) (string, error) { return manager.Logs() }, home, &listen),
		collectorLifecycleCommand("uninstall", "Uninstall managed collector", func(manager collectorManager, _, _ string) (string, error) { return manager.Uninstall() }, home, &listen),
	)
	return collector
}

type collectorManager interface {
	Install(home, listen string) (string, error)
	Start(home, listen string) (string, error)
	Stop() (string, error)
	Restart(home, listen string) (string, error)
	Logs() (string, error)
	Uninstall() (string, error)
}

func collectorLifecycleCommand(name, short string, run func(collectorManager, string, string) (string, error), home *string, listen *string) *cobra.Command {
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		message, err := run(newCollectorManager(), *home, *listen)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), message)
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
