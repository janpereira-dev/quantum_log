package cli

import (
	"fmt"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/capture/wrapper"
	"github.com/spf13/cobra"
)

func newRunCommand(home *string) *cobra.Command {
	var project, agent string
	command := &cobra.Command{Use: "run -- <command> [arguments...]", Short: "Run a command and record privacy-safe process metadata", Args: cobra.MinimumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer service.Close()
		result, err := wrapper.Run(command.Context(), service, wrapper.Config{Project: project, Agent: agent, Command: args, Input: command.InOrStdin(), Output: command.Root().OutOrStdout(), Errors: command.Root().ErrOrStderr()})
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintf(command.Root().OutOrStdout(), "recorded process session %s (exit %d)\n", result.SessionID, result.ExitCode)
		return writeErr
	}}
	command.Flags().StringVar(&project, "project", "", "explicit project slug")
	command.Flags().StringVar(&agent, "agent", "", "agent name for lifecycle metadata")
	return command
}
