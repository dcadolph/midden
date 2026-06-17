// Package keyring wraps the OS keychain for storing the vault passphrase.
//
// On macOS the keychain backend is the system Keychain.
// On Linux the backend is the Secret Service or KWallet.
// On Windows the backend is the Credential Manager.
package keyring

import (
	"errors"
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

// Service is the keychain service name midden uses for every secret.
const Service = "midden"

// AccountVaultPassphrase is the keychain account name under which the vault passphrase is stored.
const AccountVaultPassphrase = "vault-passphrase"

// SetVaultPassphrase persists the passphrase in the OS keychain under the midden service.
func SetVaultPassphrase(passphrase string) error {
	if err := gokeyring.Set(Service, AccountVaultPassphrase, passphrase); err != nil {
		return fmt.Errorf("set keychain entry: %w", err)
	}
	return nil
}

// GetVaultPassphrase returns the previously stored passphrase.
// ErrNotFound is returned when no passphrase has been stored.
func GetVaultPassphrase() (string, error) {
	v, err := gokeyring.Get(Service, AccountVaultPassphrase)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get keychain entry: %w", err)
	}
	return v, nil
}

// DeleteVaultPassphrase removes the stored passphrase from the OS keychain.
// A missing entry is treated as success.
func DeleteVaultPassphrase() error {
	if err := gokeyring.Delete(Service, AccountVaultPassphrase); err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("delete keychain entry: %w", err)
	}
	return nil
}

// ErrNotFound indicates no passphrase has been stored in the keychain.
var ErrNotFound = errors.New("passphrase not found in keychain")
