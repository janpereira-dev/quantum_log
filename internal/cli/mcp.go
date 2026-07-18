package cli

import (
	"github.com/janpereira-dev/quantum_log/internal/mcpserver"
	"github.com/spf13/cobra"
)

func newMCPCommand(home *string, version Version) *cobra.Command {
	command := &cobra.Command{Use: "mcp", Short: "Run MCP integration for coding agents"}
	command.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Serve the local QUANTUM_LOG MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return mcpserver.Run(command.Context(), *home, version.Version)
		},
	})
	return command
}
