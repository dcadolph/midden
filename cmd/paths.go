package cmd

import (
	"errors"
	"fmt"

	"github.com/dcadolph/midden/internal/vault"
)

// openVault returns the vault rooted at the directory chosen by the root flag.
// Failures are wrapped with ErrVault so they map to the vault exit code.
func openVault() (*vault.Vault, error) {
	v, err := vault.Open(vaultDir)
	if err != nil {
		return nil, errors.Join(ErrVault, fmt.Errorf("open vault: %w", err))
	}
	return v, nil
}
