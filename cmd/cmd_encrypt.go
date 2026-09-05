package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/crypt"
	"github.com/dcadolph/midden/flock"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/keyring"
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

// encryptVerifyCmd checks the passphrase against the newest encrypted day file
// and reports any files still sitting in plaintext.
var encryptVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the passphrase against the newest encrypted day file and report plaintext stragglers.",
	RunE:  runEncryptVerify,
}

// encryptStoreCmd writes the vault passphrase to the OS keychain.
var encryptStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Store the vault passphrase in the OS keychain.",
	RunE:  runEncryptStore,
}

// encryptForgetCmd removes the stored passphrase from the OS keychain.
var encryptForgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Remove the stored passphrase from the OS keychain.",
	RunE:  runEncryptForget,
}

func init() {
	encryptCmd.AddCommand(encryptStatusCmd)
	encryptCmd.AddCommand(encryptEnableCmd)
	encryptCmd.AddCommand(encryptDisableCmd)
	encryptCmd.AddCommand(encryptVerifyCmd)
	encryptCmd.AddCommand(encryptStoreCmd)
	encryptCmd.AddCommand(encryptForgetCmd)
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

// runEncryptEnable ensures every day file is encrypted and the marker is set.
// The whole conversion runs under the vault lock so concurrent appends cannot
// interleave, and files that already carry the age header are skipped, so the
// command is safe to rerun after an interruption or to seal plaintext
// stragglers in an already-encrypted vault.
func runEncryptEnable(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	var pass string
	if v.IsEncrypted() {
		pass, err = resolvePassphrase("Vault passphrase: ")
	} else {
		pass, err = resolvePassphraseWithConfirm("New vault passphrase: ")
	}
	if err != nil {
		return err
	}
	lock, err := flock.Acquire(v.LockPath())
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("acquire vault lock: %w", err))
	}
	defer func() { _ = lock.Close() }()
	encrypted := v.WithPassphrase(pass)
	if v.IsEncrypted() {
		if err := encrypted.VerifyPassphrase(); err != nil {
			return errors.Join(ErrVault, err)
		}
	}
	files, err := sealablePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	sealed, skipped := 0, 0
	for _, p := range files {
		data, err := os.ReadFile(p) //nolint:gosec // Day paths enumerated from the vault directory.
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read %s: %w", p, err))
		}
		if crypt.IsEncrypted(data) {
			skipped++
			continue
		}
		if err := encrypted.WriteBytes(p, data); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("encrypt %s: %w", p, err))
		}
		if _, err := encrypted.ReadBytes(p); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("round-trip verify %s: %w", p, err))
		}
		sealed++
	}
	if err := v.SetEncrypted(); err != nil {
		return errors.Join(ErrVault, err)
	}
	if skipped > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Encrypted %d file(s), %d already encrypted.\n", sealed, skipped)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Encrypted %d file(s).\n", sealed)
	return nil
}

// runEncryptDisable transforms every day file from ciphertext to plaintext and removes the marker.
// The conversion runs under the vault lock; already-plaintext files are skipped
// so a rerun after an interrupted disable finishes the job.
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
	lock, err := flock.Acquire(v.LockPath())
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("acquire vault lock: %w", err))
	}
	defer func() { _ = lock.Close() }()
	encrypted := v.WithPassphrase(pass)
	files, err := sealablePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	opened := 0
	for _, p := range files {
		raw, err := os.ReadFile(p) //nolint:gosec // Day paths enumerated from the vault directory.
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read %s: %w", p, err))
		}
		if !crypt.IsEncrypted(raw) {
			continue
		}
		data, err := encrypted.ReadBytes(p)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("decrypt %s: %w", p, err))
		}
		if err := v.WriteBytes(p, data); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("write %s: %w", p, err))
		}
		opened++
	}
	if err := v.ClearEncrypted(); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Decrypted %d file(s).\n", opened)
	return nil
}

// runEncryptVerify confirms the passphrase decrypts the newest encrypted day
// file and counts files that still lack the age header.
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
	files, err := sealablePaths(v)
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "ok (no files to verify)")
		return nil
	}
	plaintext := 0
	newestEncrypted := ""
	for _, p := range files {
		raw, err := os.ReadFile(p) //nolint:gosec // Day paths enumerated from the vault directory.
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read %s: %w", p, err))
		}
		if crypt.IsEncrypted(raw) {
			newestEncrypted = p
		} else {
			plaintext++
		}
	}
	if newestEncrypted != "" {
		if _, err := encrypted.ReadBytes(newestEncrypted); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("verify %s: %w", newestEncrypted, err))
		}
	}
	if plaintext > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "ok, but %d file(s) are plaintext; run `midden encrypt enable` to seal them\n", plaintext)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "ok")
	return nil
}

// runEncryptStore reads the passphrase and stores it in the OS keychain.
// The passphrase is verified against the newest day file when one exists so a
// typo cannot silently land in the keychain.
func runEncryptStore(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if !v.IsEncrypted() {
		return fmt.Errorf("vault is not encrypted: run `midden encrypt enable` first")
	}
	pass, err := resolvePassphrase("Vault passphrase: ")
	if err != nil {
		return err
	}
	latest := latestStoredDate(v)
	if !latest.IsZero() {
		if _, err := v.WithPassphrase(pass).ReadDay(latest); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("verify passphrase before storing: %w", err))
		}
	}
	if err := keyring.SetVaultPassphrase(pass); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Stored passphrase in keychain. Set `keychain: true` in your config to use it automatically.")
	return nil
}

// runEncryptForget removes the stored passphrase from the OS keychain.
func runEncryptForget(cmd *cobra.Command, _ []string) error {
	if err := keyring.DeleteVaultPassphrase(); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Removed stored passphrase from keychain.")
	return nil
}

// latestStoredDate returns the most recent date with a day file in the vault, or the zero time when empty.
func latestStoredDate(v *vault.Vault) time.Time {
	days, err := v.ListDays()
	if err != nil || len(days) == 0 {
		return time.Time{}
	}
	return days[len(days)-1]
}

// sealablePaths returns every vault file whose encryption state must track the
// vault marker: the day files, plus the recall index when one has been built.
//
// The index stores entry bodies verbatim so recall can quote them back, so
// leaving it in plaintext beside an encrypted vault would publish the journal
// it is meant to protect.
func sealablePaths(v *vault.Vault) ([]string, error) {
	paths, err := dayFilePaths(v)
	if err != nil {
		return nil, err
	}
	idx := indexPath(v)
	if _, err := os.Stat(idx); err == nil {
		paths = append(paths, idx)
	}
	return paths, nil
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
