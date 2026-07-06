package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// initCmd creates the vault directory and writes a starter README inside it.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the vault directory and seed a README.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// runInit creates the vault, seeds an in-vault README and a gitignore.
func runInit(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	if err := writeVaultReadme(v); err != nil {
		return errors.Join(ErrVault, err)
	}
	if err := writeVaultGitignore(v); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Vault ready at %s\n", v.Dir)
	return nil
}

// writeVaultReadme drops a brief in-vault README so anyone browsing the directory understands the layout.
func writeVaultReadme(v *vault.Vault) error {
	path := filepath.Join(v.Dir, "README.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	body := "# midden vault\n\n" +
		"Daily journal files live under YYYY/MM/DD.md.\n" +
		"Each entry begins with a level-two header carrying a timestamp\n" +
		"and optional inline hashtags, followed by a free markdown body.\n\n" +
		"Manage entries with the `midden` CLI.\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write vault README: %w", err)
	}
	return nil
}

// writeVaultGitignore ignores common editor artifacts and the advisory lock file
// so the vault is safe to track with git.
func writeVaultGitignore(v *vault.Vault) error {
	path := filepath.Join(v.Dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	body := ".midden.lock\n.DS_Store\n*.swp\n*.swo\n.idea/\n.vscode/\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write vault gitignore: %w", err)
	}
	return nil
}
