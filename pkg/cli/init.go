package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"silo/pkg/config"
	"silo/pkg/vault"
	"silo/pkg/vault/models"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
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
  SILO_VAULT_PASSWORD, SILO_PROVIDER, SILO_API_KEY`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "run without prompts (requires env vars)")
}

func runInit(cmd *cobra.Command, args []string) error {
	vaultPath := viper.GetString("vault.path")
	v := vault.New(vaultPath)

	if v.Exists() {
		return fmt.Errorf("silo already initialized. Vault exists at %s", vaultPath)
	}

	if nonInteractive {
		return runNonInteractive(v)
	}

	return runInteractive(v)
}

func runInteractive(v models.SecretVault) error {
	fmt.Println("Welcome to Silo!")
	fmt.Println()

	if err := config.EnsureDataDir(); err != nil {
		return err
	}

	fmt.Println("Step 1: Create encrypted vault")
	fmt.Print("Enter vault password (min 8 chars): ")
	pass1, err := readPass()
	if err != nil {
		return err
	}

	fmt.Print("Confirm password: ")
	pass2, err := readPass()
	if err != nil {
		return err
	}

	if pass1 != pass2 {
		return fmt.Errorf("passwords do not match")
	}
	if len(pass1) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	if err := v.Create(pass1); err != nil {
		return err
	}
	fmt.Println("Vault created.")
	fmt.Println()

	fmt.Println("Step 2: Configure LLM provider")
	fmt.Println("Available providers: gemini, openai, anthropic")
	fmt.Print("Select provider [gemini]: ")

	reader := bufio.NewReader(os.Stdin)
	provider, _ := reader.ReadString('\n')
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "gemini"
	}

	fmt.Printf("Enter %s API key: ", provider)
	apiKey, err := readPass()
	if err != nil {
		return err
	}

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	keyName := fmt.Sprintf("%s_api_key", provider)
	if err := v.WriteSecret(keyName, []byte(apiKey)); err != nil {
		return err
	}
	v.Close()

	configPath := config.DefaultConfigPath()
	if err := writeMinimalConfig(configPath, provider); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Println()
	fmt.Println("Silo initialized successfully!")
	fmt.Printf("Config: %s\n", configPath)
	return nil
}

func runNonInteractive(v models.SecretVault) error {
	password := os.Getenv("SILO_VAULT_PASSWORD")
	provider := os.Getenv("SILO_PROVIDER")
	apiKey := os.Getenv("SILO_API_KEY")

	if password == "" {
		return fmt.Errorf("SILO_VAULT_PASSWORD required for non-interactive mode")
	}
	if apiKey == "" {
		return fmt.Errorf("SILO_API_KEY required for non-interactive mode")
	}
	if provider == "" {
		provider = "gemini"
	}

	if len(password) < 8 {
		return fmt.Errorf("SILO_VAULT_PASSWORD must be at least 8 characters")
	}

	if err := config.EnsureDataDir(); err != nil {
		return err
	}

	if err := v.Create(password); err != nil {
		return err
	}

	keyName := fmt.Sprintf("%s_api_key", provider)
	if err := v.WriteSecret(keyName, []byte(apiKey)); err != nil {
		return err
	}
	v.Close()

	configPath := config.DefaultConfigPath()
	if err := writeMinimalConfig(configPath, provider); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Println("Silo initialized successfully")
	return nil
}

// writeMinimalConfig writes only user-specific settings to the personal config file.
// All other values come from config/silo.toml (project defaults) or code defaults.
func writeMinimalConfig(path, provider string) error {
	content := fmt.Sprintf("[providers]\ndefault = %q\n", provider)
	return os.WriteFile(path, []byte(content), 0600)
}

func readPass() (string, error) {
	pass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return string(pass), nil
}
