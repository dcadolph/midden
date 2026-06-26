package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/dcadolph/midden/keyring"
)

// EnvPassphrase is the environment variable that supplies the vault passphrase
// without an interactive prompt.
const EnvPassphrase = "MIDDEN_PASSPHRASE"

// passphraseFlag holds the value of --passphrase when supplied.
// Setting it on the command line is discouraged because it can be captured by
// shell history; it exists for automation contexts that already protect the
// argument list.
var passphraseFlag string

// resolvePassphrase returns the vault passphrase using the documented precedence:
// the --passphrase flag, MIDDEN_PASSPHRASE, the OS keychain when enabled in
// config, then an interactive prompt against the controlling terminal.
func resolvePassphrase(prompt string) (string, error) {
	if passphraseFlag != "" {
		return passphraseFlag, nil
	}
	if env := os.Getenv(EnvPassphrase); env != "" {
		return env, nil
	}
	if keychainEnabled() {
		if pass, err := keyring.GetVaultPassphrase(); err == nil {
			return pass, nil
		} else if !errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("read keychain: %w", err)
		}
	}
	return readPassphraseFromTerminal(prompt)
}

// resolvePassphraseWithConfirm reads a new passphrase from the controlling terminal twice
// and verifies they match. Flag and environment values bypass the confirmation step
// because callers using them are running unattended.
func resolvePassphraseWithConfirm(prompt string) (string, error) {
	if passphraseFlag != "" {
		return passphraseFlag, nil
	}
	if env := os.Getenv(EnvPassphrase); env != "" {
		return env, nil
	}
	first, err := readPassphraseFromTerminal(prompt)
	if err != nil {
		return "", err
	}
	second, err := readPassphraseFromTerminal("Confirm passphrase: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passphrases did not match")
	}
	if strings.TrimSpace(first) == "" {
		return "", errors.New("passphrase is empty")
	}
	return first, nil
}

// readPassphraseFromTerminal prompts the user on stderr and reads a passphrase
// from /dev/tty (so piped stdin does not leak into the prompt).
func readPassphraseFromTerminal(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open tty: %w", err)
	}
	defer tty.Close()
	if _, err := fmt.Fprint(tty, prompt); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	bytesIn, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return string(bytes.TrimSpace(bytesIn)), nil
}
