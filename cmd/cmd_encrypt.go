package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// encryptCmd groups vault encryption subcommands.
var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Manage vault-at-rest encryption.",
}

// encryptStatusCmd reports whether the vault is encrypted.
var encryptStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print whether the vault is encrypted at rest.",
	RunE:  runEncryptStatus,
}

// encryptEnableCmd encrypts every existing day file and writes the marker.
var encryptEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Encrypt every day file in the vault and lock it behind a passphrase.",
	RunE:  runEncryptEnable,
}

// encryptDisableCmd decrypts every day file and removes the marker.
var encryptDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Decrypt every day file and remove the encryption marker.",
	RunE:  runEncryptDisable,
}

// encryptVerifyCmd checks that the supplied passphrase decrypts the most recent day file.
var encryptVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify that the supplied passphrase unlocks the vault.",
	RunE:  runEncryptVerify,
}

func init() {
	encryptCmd.AddCommand(encryptStatusCmd)
	encryptCmd.AddCommand(encryptEnableCmd)
	encryptCmd.AddCommand(encryptDisableCmd)
	encryptCmd.AddCommand(encryptVerifyCmd)
	rootCmd.AddCommand(encryptCmd)
}

// runEncryptStatus prints whether the vault is encrypted at rest.
func runEncryptStatus(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if v.IsEncrypted() {
		fmt.Fprintln(cmd.OutOrStdout(), "encrypted")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "plaintext")
	return nil
}

// runEncryptEnable transforms every day file from plaintext to ciphertext and writes the marker.
func runEncryptEnable(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if v.IsEncrypted() {
		return fmt.Errorf("vault already encrypted")
	}
	pass, err := resolvePassphraseWithConfirm("New vault passphrase: ")
	if err != nil {
		return err
	}
	encrypted := v.WithPassphrase(pass)
	files, err := dayFilePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read %s: %w", p, err))
		}
		if err := encrypted.WriteBytes(p, data); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("encrypt %s: %w", p, err))
		}
	}
	if err := v.SetEncrypted(); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Encrypted %d file(s).\n", len(files))
	return nil
}

// runEncryptDisable transforms every day file from ciphertext to plaintext and removes the marker.
func runEncryptDisable(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if !v.IsEncrypted() {
		return fmt.Errorf("vault is not encrypted")
	}
	pass, err := resolvePassphrase("Vault passphrase: ")
	if err != nil {
		return err
	}
	encrypted := v.WithPassphrase(pass)
	files, err := dayFilePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	for _, p := range files {
		data, err := encrypted.ReadBytes(p)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("decrypt %s: %w", p, err))
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("write %s: %w", p, err))
		}
	}
	if err := v.ClearEncrypted(); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Decrypted %d file(s).\n", len(files))
	return nil
}

// runEncryptVerify confirms that the supplied passphrase decrypts the most recent day file.
func runEncryptVerify(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if !v.IsEncrypted() {
		return fmt.Errorf("vault is not encrypted")
	}
	pass, err := resolvePassphrase("Vault passphrase: ")
	if err != nil {
		return err
	}
	encrypted := v.WithPassphrase(pass)
	files, err := dayFilePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "ok (no files to verify)")
		return nil
	}
	last := files[len(files)-1]
	if _, err := encrypted.ReadBytes(last); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("verify %s: %w", last, err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "ok")
	return nil
}

// dayFilePaths returns the absolute paths of every YYYY/MM/DD.md file under the vault.
// The slice is ordered ascending by date so callers can use the last element as the newest.
func dayFilePaths(v *vault.Vault) ([]string, error) {
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(days))
	for _, d := range days {
		path := v.DayPath(d)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, path)
	}
	return out, nil
}
