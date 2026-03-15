package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DefaultDataDir returns the default data directory path
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".silo")
}

// DefaultConfigPath returns the default config file path
func DefaultConfigPath() string {
	return filepath.Join(DefaultDataDir(), "silo.toml")
}

// SetDefaults sets default values for all configuration
func SetDefaults() {
	// Gateway defaults
	viper.SetDefault("gateway.host", "127.0.0.1")
	viper.SetDefault("gateway.port", 5110)
	viper.SetDefault("gateway.brain_enabled", true)
	viper.SetDefault("gateway.max_request_size", "10mb")
	viper.SetDefault("gateway.cors_origins", []string{"*"})
	viper.SetDefault("gateway.auth.enabled", true)
	viper.SetDefault("gateway.timeouts.read", "30s")
	viper.SetDefault("gateway.timeouts.write", "60s")
	viper.SetDefault("gateway.timeouts.idle", "120s")

	// Provider defaults
	viper.SetDefault("providers.default", "gemini")
	viper.SetDefault("providers.gemini.model", "gemini-2.0-flash")
	viper.SetDefault("providers.openai.model", "gpt-4o")

	// Tool approval defaults
	viper.SetDefault("tools.approval.mode", "per-tool")
	viper.SetDefault("tools.approval.timeout", 30)
	viper.SetDefault("tools.approval.rules.bash", "always")

	// Shell defaults — only read-only observation commands are pre-approved.
	// Everything that can modify files, run scripts, or make network calls requires approval.
	viper.SetDefault("tools.shell.allowed_commands", []string{
		"ls", "cat", "head", "tail", "wc", "grep", "find",
		"date", "pwd", "whoami", "uname", "df", "du", "ps", "env",
	})
	viper.SetDefault("tools.shell.permissions_file", filepath.Join(DefaultDataDir(), "allowed_permissions.md"))
	viper.SetDefault("tools.shell.blocked_patterns", []string{
		"rm -rf /", "dd if=/dev/zero", "sudo", "su", "eval", "exec",
		"nc", "ncat", "netcat", "ssh", "scp", "chmod 777",
	})
	viper.SetDefault("tools.shell.max_output_bytes", 1048576)
	viper.SetDefault("tools.shell.timeout_secs", 30)

	// Filesystem defaults
	viper.SetDefault("tools.filesystem.working_directory", "~")
	viper.SetDefault("tools.filesystem.allowed_paths", []string{"~/", "/tmp"})
	viper.SetDefault("tools.filesystem.blocked_paths", []string{
		"~/.ssh", "~/.gnupg", "~/.silo/vault.enc",
		"~/.aws", "~/.kube", "~/.config/gcloud",
	})
	viper.SetDefault("tools.filesystem.max_read_size", "10mb")
	viper.SetDefault("tools.filesystem.max_write_size", "50mb")

	// Environment defaults
	viper.SetDefault("tools.environment.passthrough_all", false)

	// Sandbox defaults
	viper.SetDefault("tools.sandbox.enabled", true)
	viper.SetDefault("tools.sandbox.direct_commands", []string{
		"ls", "cat", "head", "tail", "grep", "find",
		"wc", "date", "pwd", "whoami", "uname",
	})
	viper.SetDefault("tools.sandbox.max_output_size", "50mb")
	viper.SetDefault("tools.sandbox.cleanup_orphans_after", "1h")

	// Session defaults
	viper.SetDefault("session.db_path", filepath.Join(DefaultDataDir(), "sessions.db"))
	viper.SetDefault("session.retention_days", 30)

	// Vault defaults
	viper.SetDefault("vault.path", filepath.Join(DefaultDataDir(), "vault.enc"))
	viper.SetDefault("vault.argon2.memory_mib", 8)
	viper.SetDefault("vault.argon2.iterations", 8)
	viper.SetDefault("vault.argon2.parallelism", 2)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "auto")
	viper.SetDefault("logging.log_dir", filepath.Join(DefaultDataDir(), "logs"))

	// Agent defaults
	viper.SetDefault("agent.max_iterations", 25)
	viper.SetDefault("agent.system_prompt_path", "")
	viper.SetDefault("agent.retry_on_error", 1)
	viper.SetDefault("agent.retry_backoff_ms", 1000)
}

// Validate validates the current configuration
func Validate() error {
	port := viper.GetInt("gateway.port")
	if port < 1 || port > 65535 {
		return fmt.Errorf("gateway.port must be 1-65535, got %d", port)
	}

	memoryMiB := viper.GetInt("vault.argon2.memory_mib")
	if memoryMiB < 1 {
		return fmt.Errorf("vault.argon2.memory_mib must be >= 1")
	}

	timeout := viper.GetInt("tools.approval.timeout")
	if timeout < 0 {
		return fmt.Errorf("tools.approval.timeout must be >= 0")
	}

	maxIter := viper.GetInt("agent.max_iterations")
	if maxIter < 1 {
		return fmt.Errorf("agent.max_iterations must be >= 1")
	}

	return nil
}

// EnsureDataDir creates the data directory if it doesn't exist
func EnsureDataDir() error {
	dataDir := DefaultDataDir()
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"logs", "workspace"}
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0700); err != nil {
			return fmt.Errorf("create %s directory: %w", sub, err)
		}
	}

	return nil
}
