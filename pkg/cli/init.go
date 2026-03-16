package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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

var projectConfigBytes []byte

// SetProjectConfig stores the embedded config/silo.toml for use during init.
func SetProjectConfig(b []byte) {
	projectConfigBytes = b
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Silo configuration and vault",
	Long: `Initialize Silo with first-time setup wizard.

This command will:
  1. Create the data directory (~/.silo/)
  2. Set up the encrypted vault with a password
  3. Configure your LLM provider and API key
  4. Generate a gateway bearer token
  5. Write the default configuration file

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
	pass1, err := readPasswordMasked("Enter vault password (min 8 chars): ")
	if err != nil {
		return err
	}

	pass2, err := readPasswordMasked("Confirm password: ")
	if err != nil {
		return err
	}

	if pass1 != pass2 {
		fmt.Fprintln(os.Stderr, "Error: passwords do not match")
		return fmt.Errorf("passwords do not match")
	}
	if len(pass1) < 8 {
		fmt.Fprintln(os.Stderr, "Error: password must be at least 8 characters")
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

	apiKey, err := readPasswordMasked(fmt.Sprintf("Enter %s API key: ", provider))
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

	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate gateway token: %w", err)
	}
	if err := v.WriteSecret("gateway-token", []byte(token)); err != nil {
		return err
	}
	v.Close()

	configPath := config.DefaultConfigPath()
	if err := writeUserConfig(configPath); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Println()
	fmt.Println("Silo initialized successfully!")
	fmt.Printf("Config:        %s\n", configPath)
	fmt.Printf("Gateway token: %s\n", token)
	fmt.Println()
	fmt.Println("Keep your gateway token safe — you need it to authenticate API requests.")
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

	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate gateway token: %w", err)
	}
	if err := v.WriteSecret("gateway-token", []byte(token)); err != nil {
		return err
	}
	v.Close()

	configPath := config.DefaultConfigPath()
	if err := writeUserConfig(configPath); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Println("Silo initialized successfully")
	fmt.Printf("Gateway token: %s\n", token)
	return nil
}

// writeUserConfig copies the embedded project config to the user config path.
func writeUserConfig(path string) error {
	return os.WriteFile(path, projectConfigBytes, 0600)
}

// generateToken returns a 32-byte cryptographically random hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// readPasswordMasked prompts the user and reads a password showing * for each character.
func readPasswordMasked(prompt string) (string, error) {
	fmt.Print(prompt)

	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		// fallback to silent read if raw mode unavailable
		pass, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		return string(pass), err
	}
	defer term.Restore(int(syscall.Stdin), oldState)

	var buf []byte
	b := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			break
		}
		switch b[0] {
		case '\r', '\n':
			fmt.Print("\r\n")
			return string(buf), nil
		case 3: // Ctrl+C
			fmt.Print("\r\n")
			return "", fmt.Errorf("interrupted")
		case 127, 8: // backspace / DEL
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}
		default:
			if b[0] >= 32 { // printable
				buf = append(buf, b[0])
				fmt.Print("*")
			}
		}
	}
	fmt.Print("\r\n")
	return string(buf), nil
}

// readPass is kept for backward compat with start.go which calls it.
func readPass() (string, error) {
	pass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return string(pass), nil
}
