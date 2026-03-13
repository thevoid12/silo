package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on Silo installation",
	Long: `Run a series of environment checks and report pass/warn/fail for each.

Checks performed:
  - Data directory exists and is writable
  - Config file is valid
  - Vault exists and is readable
  - Provider is configured and reachable
  - Workspace directory exists
  - Go runtime information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 4
		fmt.Println("silo doctor - not yet implemented")
		return nil
	},
}
