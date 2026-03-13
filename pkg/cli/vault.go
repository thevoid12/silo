package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
	Long: `Create the encrypted vault and set the encryption password.

This is typically done as part of 'silo init', but can be run
independently to create or recreate the vault.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 2
		fmt.Println("silo vault init - not yet implemented")
		return nil
	},
}

var vaultSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Store a secret in the vault",
	Long: `Store a secret value in the encrypted vault.

The value will be prompted with no-echo input. If the key already
exists, the value will be overwritten.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 2
		fmt.Printf("silo vault set %s - not yet implemented\n", args[0])
		return nil
	},
}

var vaultGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Retrieve a secret from the vault",
	Long:  `Retrieve and display a secret value from the vault.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 2
		fmt.Printf("silo vault get %s - not yet implemented\n", args[0])
		return nil
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys in the vault",
	Long: `List all secret key names stored in the vault.

Only key names are shown, not the secret values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement in Step 2
		fmt.Println("silo vault list - not yet implemented")
		return nil
	},
}

func init() {
	vaultCmd.AddCommand(vaultInitCmd)
	vaultCmd.AddCommand(vaultSetCmd)
	vaultCmd.AddCommand(vaultGetCmd)
	vaultCmd.AddCommand(vaultListCmd)
}
