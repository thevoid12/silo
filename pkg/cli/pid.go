package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// readPID reads the process ID from a PID file
func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read PID file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}
	return pid, nil
}

// writePID writes the process ID to a PID file
func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}

// parseDurationWithDefault parses a viper duration string, logging a warning and returning the default on failure
func parseDurationWithDefault(log *zap.Logger, key string, def time.Duration) time.Duration {
	raw := viper.GetString(key)
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Warn("invalid duration in config, using default", zap.String("key", key), zap.String("value", raw), zap.Duration("default", def))
		return def
	}
	return d
}

// isProcessRunning checks if a process with the given PID is alive
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
