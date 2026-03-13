package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	chatModel    string
	chatProvider string
	chatNoTools  bool
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive readline-based chat session in the terminal.

The chat connects directly to the local agent (in-process, no HTTP).
Type your message and press Enter to send. The agent streams responses
token-by-token.

In-session commands:
  /help   - Show available commands
  /clear  - Clear the screen
  /exit   - Exit the chat (same as Ctrl-D)
  /new    - Start a new conversation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 8
		fmt.Println("silo chat - not yet implemented")
		return nil
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatModel, "model", "", "override the default model for this session")
	chatCmd.Flags().StringVar(&chatProvider, "provider", "", "override the default provider for this session")
	chatCmd.Flags().BoolVar(&chatNoTools, "no-tools", false, "disable tool use for this session")
}
