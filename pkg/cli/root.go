package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	dataDir string
	verbose int
	quiet   bool
	jsonOut bool
	noColor bool
)

var rootCmd = &cobra.Command{
	Use:   "silo",
	Short: "Silo - Local-first AI agent framework",
	Long: `Silo is a local-first, security-first AI agent framework.

It provides a secure environment for running AI agents with:
  - Encrypted vault for secrets (no .env files)
  - Human-in-the-loop tool approval
  - Multiple modes: Desktop, CLI, and Headless`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.silo/silo.toml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory (default: ~/.silo/)")
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "increase log verbosity (repeat for more: -vv)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress all output except errors")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(vaultCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		siloDir := filepath.Join(home, ".silo")
		viper.AddConfigPath(siloDir)
		viper.SetConfigName("silo")
		viper.SetConfigType("toml")
	}

	// Environment variable binding
	viper.SetEnvPrefix("SILO")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file found but error reading it
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		}
		// Config file not found is OK - we'll use defaults
	}
}

// GetDataDir returns the data directory path
func GetDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".silo")
}

// IsQuiet returns true if quiet mode is enabled
func IsQuiet() bool {
	return quiet
}

// IsJSON returns true if JSON output mode is enabled
func IsJSON() bool {
	return jsonOut
}

// VerboseLevel returns the verbosity level
func VerboseLevel() int {
	return verbose
}
