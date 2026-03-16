package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"silo/pkg/logger"
)

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Silo server status",
	RunE:  runServerStatus,
}

// runServerStatus checks the PID file and logs whether the server is running
func runServerStatus(cmd *cobra.Command, args []string) error {
	log, err := logger.InitializeLogger()
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer log.Sync()

	pidFile := viper.GetString("gateway.pid_file")
	host := viper.GetString("gateway.host")
	port := viper.GetInt("gateway.port")

	pid, err := readPID(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("silo server: not running")
			return nil
		}
		log.Info("silo server: not running", zap.String("reason", "stale or missing PID file"))
		return nil
	}

	if !isProcessRunning(pid) {
		_ = os.Remove(pidFile)
		log.Info("silo server: not running", zap.String("reason", "process exited"))
		return nil
	}

	log.Info("silo server: running", zap.Int("pid", pid), zap.String("host", host), zap.Int("port", port))
	return nil
}
