package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	headless    bool
	startPort   int
	desktopMode bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Silo (desktop app or headless server)",
	Long: `Start Silo in either desktop or headless mode.

Desktop mode (default):
  Launches the Electron desktop app with the Go backend as a sidecar.

Headless mode (--headless):
  Starts the HTTP/SSE gateway server for external adapters.
  Perfect for servers, Raspberry Pi, Docker, or CI/CD environments.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 9
		if headless {
			fmt.Println("silo start --headless - not yet implemented")
		} else {
			fmt.Println("silo start (desktop) - not yet implemented")
		}
		return nil
	},
}

func init() {
	startCmd.Flags().BoolVar(&headless, "headless", false, "start in headless mode (HTTP server only)")
	startCmd.Flags().IntVar(&startPort, "port", 5110, "port for headless mode server")
	startCmd.Flags().BoolVar(&desktopMode, "desktop-mode", false, "internal flag for desktop app integration")
	startCmd.Flags().MarkHidden("desktop-mode")
}
