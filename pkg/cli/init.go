package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var nonInteractive bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Silo configuration and vault",
	Long: `Initialize Silo with first-time setup wizard.

This command will:
  1. Create the data directory (~/.silo/)
  2. Set up the encrypted vault with a password
  3. Configure your LLM provider and API key
  4. Write the default configuration file

For automated/headless setup, use --non-interactive with environment variables:
  SILO_VAULT_PASSWORD, SILO_PROVIDER_NAME, SILO_PROVIDER_API_KEY`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 3
		fmt.Println("silo init - not yet implemented")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "run without prompts (requires env vars)")
}
