package cli

import (
	"fmt"
	"os"
	"runtime"

	"silo/pkg/config"
	"silo/pkg/vault"
	"silo/version"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on Silo installation",
	Long: `Run a series of environment checks and report pass/warn/fail for each.

Checks performed:
  - Data directory exists and is writable
  - Config file is valid
  - Vault exists
  - Provider is configured
  - Workspace directory exists`,
	RunE: runDoctor,
}

type checkResult struct {
	name   string
	status string
	detail string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Printf("Silo Doctor v%s\n", version.VERSION)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	results := []checkResult{}

	results = append(results, checkDataDir())
	results = append(results, checkConfig())
	results = append(results, checkVault())
	results = append(results, checkProvider())
	results = append(results, checkWorkspace())

	passed, warned, failed := 0, 0, 0

	for _, r := range results {
		var icon string
		switch r.status {
		case "pass":
			icon = "[OK]"
			passed++
		case "warn":
			icon = "[WARN]"
			warned++
		case "fail":
			icon = "[FAIL]"
			failed++
		}
		fmt.Printf("  %s %s", icon, r.name)
		if r.detail != "" {
			fmt.Printf(": %s", r.detail)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("Summary: %d passed, %d warnings, %d failed\n", passed, warned, failed)

	if failed > 0 {
		return fmt.Errorf("some checks failed")
	}
	return nil
}

func checkDataDir() checkResult {
	dataDir := config.DefaultDataDir()
	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		return checkResult{"Data directory", "fail", fmt.Sprintf("%s not found", dataDir)}
	}
	if err != nil {
		return checkResult{"Data directory", "fail", err.Error()}
	}
	if !info.IsDir() {
		return checkResult{"Data directory", "fail", "not a directory"}
	}
	return checkResult{"Data directory", "pass", dataDir}
}

func checkConfig() checkResult {
	configPath := config.DefaultConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return checkResult{"Config file", "warn", "not found (using defaults)"}
	}

	if err := config.Validate(); err != nil {
		return checkResult{"Config file", "fail", err.Error()}
	}
	return checkResult{"Config file", "pass", configPath}
}

func checkVault() checkResult {
	vaultPath := viper.GetString("vault.path")
	v := vault.New(vaultPath)
	if !v.Exists() {
		return checkResult{"Vault", "fail", "not found. Run 'silo init'"}
	}
	return checkResult{"Vault", "pass", vaultPath}
}

func checkProvider() checkResult {
	provider := viper.GetString("providers.default")
	if provider == "" {
		return checkResult{"Provider", "warn", "not configured"}
	}
	return checkResult{"Provider", "pass", provider}
}

func checkWorkspace() checkResult {
	workDir := viper.GetString("tools.filesystem.working_directory")
	if workDir == "" || workDir == "~" {
		home, _ := os.UserHomeDir()
		workDir = home
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return checkResult{"Workspace", "warn", fmt.Sprintf("%s not found", workDir)}
	}
	return checkResult{"Workspace", "pass", workDir}
}
