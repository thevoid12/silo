package cli

import (
	"fmt"
	"syscall"

	"silo/pkg/config"
	"silo/pkg/vault"
	"silo/pkg/vault/models"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage the encrypted secret vault",
	Long: `Manage the encrypted vault that stores API keys and secrets.

The vault uses XChaCha20-Poly1305 encryption with Argon2id key derivation.
Secrets are encrypted at rest and never stored in plaintext.`,
}

var vaultInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new vault with a password",
	RunE: func(cmd *cobra.Command, args []string) error {
		vaultPath := viper.GetString("vault.path")
		v := vault.New(vaultPath)

		if v.Exists() {
			return fmt.Errorf("vault already exists at %s", vaultPath)
		}

		fmt.Print("Enter vault password: ")
		pass1, err := readPassword()
		if err != nil {
			return err
		}

		fmt.Print("Confirm password: ")
		pass2, err := readPassword()
		if err != nil {
			return err
		}

		if pass1 != pass2 {
			return fmt.Errorf("passwords do not match")
		}

		if len(pass1) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}

		if err := config.EnsureDataDir(); err != nil {
			return err
		}

		if err := v.Create(pass1); err != nil {
			return err
		}

		fmt.Println("Vault created successfully")
		return nil
	},
}

var vaultSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Store a secret in the vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVault()
		if err != nil {
			return err
		}
		defer v.Close()

		key := args[0]

		fmt.Printf("Enter value for %s: ", key)
		value, err := readPassword()
		if err != nil {
			return err
		}

		if err := v.WriteSecret(key, []byte(value)); err != nil {
			return err
		}

		fmt.Printf("Secret '%s' stored\n", key)
		return nil
	},
}

var vaultGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Retrieve a secret from the vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVault()
		if err != nil {
			return err
		}
		defer v.Close()

		value, err := v.ReadSecret(args[0])
		if err != nil {
			return err
		}

		fmt.Println(string(value))
		return nil
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys in the vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVault()
		if err != nil {
			return err
		}
		defer v.Close()

		keys, err := v.ListKeys()
		if err != nil {
			return err
		}

		if len(keys) == 0 {
			fmt.Println("No secrets stored")
			return nil
		}

		for _, k := range keys {
			fmt.Println(k)
		}
		return nil
	},
}

var vaultDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret from the vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVault()
		if err != nil {
			return err
		}
		defer v.Close()

		if err := v.RemoveSecret(args[0]); err != nil {
			return err
		}

		fmt.Printf("Secret '%s' deleted\n", args[0])
		return nil
	},
}

func init() {
	vaultCmd.AddCommand(vaultInitCmd)
	vaultCmd.AddCommand(vaultSetCmd)
	vaultCmd.AddCommand(vaultGetCmd)
	vaultCmd.AddCommand(vaultListCmd)
	vaultCmd.AddCommand(vaultDeleteCmd)
}

func readPassword() (string, error) {
	pass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pass), nil
}

// openVault prompts for a password and returns an unlocked vault
func openVault() (models.SecretVault, error) {
	vaultPath := viper.GetString("vault.path")
	v := vault.New(vaultPath)

	if !v.Exists() {
		return nil, fmt.Errorf("vault not found. Run 'silo vault init' first")
	}

	fmt.Print("Enter vault password: ")
	pass, err := readPassword()
	if err != nil {
		return nil, err
	}

	if err := v.Open(pass); err != nil {
		return nil, err
	}

	return v, nil
}
