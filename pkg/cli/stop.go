package cli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	siloerrors "silo/pkg/errors"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Silo server",
	RunE:  runStop,
}

// runStop sends SIGTERM to the server process and removes the PID file
func runStop(cmd *cobra.Command, args []string) error {
	pidFile := viper.GetString("gateway.pid_file")

	pid, err := readPID(pidFile)
	if err != nil {
		return siloerrors.ErrServerNotRunning
	}

	if !isProcessRunning(pid) {
		_ = os.Remove(pidFile)
		return siloerrors.ErrServerNotRunning
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to PID %d: %w", pid, err)
	}

	_ = os.Remove(pidFile)
	fmt.Printf("silo server stopped (PID %d)\n", pid)
	return nil
}
