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
const EnvPassphrase = "MIDDEN_PASSPHRASE" //nolint:gosec // Environment variable name, not a credential.

// passphraseFlag holds the value of --passphrase when supplied.
// Setting it on the command line is discouraged because it can be captured by
// shell history; it exists for automation contexts that already protect the
// argument list.
var passphraseFlag string

// resolvePassphrase returns the vault passphrase using the documented precedence:
// the --passphrase flag, MIDDEN_PASSPHRASE, the OS keychain when enabled in
// config, then an interactive prompt against the controlling terminal.
// Flag, environment, and keychain values are trimmed and must be non-empty.
// The flag value is zeroed once read to shorten its stay in memory.
func resolvePassphrase(prompt string) (string, error) {
	if passphraseFlag != "" {
		pass, err := trimmedPassphrase("flag", passphraseFlag)
		passphraseFlag = ""
		return pass, err
	}
	if env := os.Getenv(EnvPassphrase); env != "" {
		return trimmedPassphrase("environment", env)
	}
	if keychainEnabled() {
		if pass, err := keyring.GetVaultPassphrase(); err == nil {
			return trimmedPassphrase("keychain", pass)
		} else if !errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("read keychain: %w", err)
		}
	}
	return readPassphraseFromTerminal(prompt)
}

// resolvePassphraseWithConfirm reads a new passphrase from the controlling terminal twice
// and verifies they match. Flag and environment values bypass the confirmation step
// because callers using them are running unattended; both are trimmed and must
// be non-empty, and the flag value is zeroed once read.
func resolvePassphraseWithConfirm(prompt string) (string, error) {
	if passphraseFlag != "" {
		pass, err := trimmedPassphrase("flag", passphraseFlag)
		passphraseFlag = ""
		return pass, err
	}
	if env := os.Getenv(EnvPassphrase); env != "" {
		return trimmedPassphrase("environment", env)
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

// trimmedPassphrase trims surrounding whitespace from a non-interactive
// passphrase source and rejects values that trim to nothing.
func trimmedPassphrase(source, pass string) (string, error) {
	pass = strings.TrimSpace(pass)
	if pass == "" {
		return "", fmt.Errorf("%s passphrase is empty", source)
	}
	return pass, nil
}

// readPassphraseFromTerminal prompts the user on stderr and reads a passphrase
// from /dev/tty (so piped stdin does not leak into the prompt).
func readPassphraseFromTerminal(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open tty: %w", err)
	}
	defer func() { _ = tty.Close() }()
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
