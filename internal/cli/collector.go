package cli

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/otlp"
	"github.com/spf13/cobra"
)

func newCollectorCommand(home *string) *cobra.Command {
	collector := &cobra.Command{Use: "collector", Short: "Receive local telemetry"}
	var listen string
	var allowNonLoopback bool
	serve := &cobra.Command{Use: "serve", Short: "Serve OTLP/HTTP JSON traces", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if err := validateListenAddress(listen, allowNonLoopback); err != nil {
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer service.Close()
		server := &http.Server{Addr: listen, Handler: otlp.NewHandler(service), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "OTLP/HTTP JSON receiver listening on http://%s/v1/traces\n", listen)
		if err != nil {
			return err
		}
		return server.ListenAndServe()
	}}
	serve.Flags().StringVar(&listen, "listen", "127.0.0.1:4318", "OTLP/HTTP listen address")
	serve.Flags().BoolVar(&allowNonLoopback, "allow-non-loopback", false, "allow a non-loopback listen address")
	collector.AddCommand(serve)
	return collector
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
