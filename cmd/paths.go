package cmd

import (
	"errors"
	"fmt"

	"github.com/dcadolph/midden/internal/vault"
)

// openVault returns the vault rooted at the directory chosen by the root flag.
// Encrypted vaults are unlocked using the resolved passphrase before the handle
// is returned so downstream commands do not need to repeat the unlock dance.
func openVault() (*vault.Vault, error) {
	v, err := vault.Open(vaultDir)
	if err != nil {
		return nil, errors.Join(ErrVault, fmt.Errorf("open vault: %w", err))
	}
	if !v.IsEncrypted() {
		return v, nil
	}
	pass, err := resolvePassphrase("Vault passphrase: ")
	if err != nil {
		return nil, errors.Join(ErrVault, err)
	}
	return v.WithPassphrase(pass), nil
}

// openVaultRaw returns the vault without unlocking it.
// Use it from encrypt subcommands that need to inspect or rewrite the marker
// state without prompting for a passphrase up front.
func openVaultRaw() (*vault.Vault, error) {
	v, err := vault.Open(vaultDir)
	if err != nil {
		return nil, errors.Join(ErrVault, fmt.Errorf("open vault: %w", err))
	}
	return v, nil
}
