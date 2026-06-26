// Package crypt wraps passphrase-based file encryption with the age library.
//
// Vaults that opt in carry every day file encrypted at rest using the scrypt
// recipient supplied by age. The passphrase is the only secret material the
// user holds; the vault root stays plaintext-safe.
package crypt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
)

// MagicHeader is the in-band marker that age-encrypted streams begin with.
// Files whose first bytes match this header are treated as encrypted; others are plaintext.
const MagicHeader = "age-encryption.org/v1"

// IsEncrypted reports whether data begins with the age magic header.
func IsEncrypted(data []byte) bool {
	return bytes.HasPrefix(data, []byte(MagicHeader))
}

// Encrypt writes plaintext to w encrypted to the scrypt recipient derived from passphrase.
func Encrypt(w io.Writer, passphrase string, plaintext []byte) error {
	if strings.TrimSpace(passphrase) == "" {
		return errors.New("passphrase is empty")
	}
	rec, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return fmt.Errorf("build recipient: %w", err)
	}
	stream, err := age.Encrypt(w, rec)
	if err != nil {
		return fmt.Errorf("open encrypt stream: %w", err)
	}
	if _, err := stream.Write(plaintext); err != nil {
		stream.Close()
		return fmt.Errorf("write encrypt stream: %w", err)
	}
	if err := stream.Close(); err != nil {
		return fmt.Errorf("close encrypt stream: %w", err)
	}
	return nil
}

// Decrypt reads an age-encrypted stream from r using the scrypt identity derived from passphrase.
func Decrypt(r io.Reader, passphrase string) ([]byte, error) {
	if strings.TrimSpace(passphrase) == "" {
		return nil, errors.New("passphrase is empty")
	}
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("build identity: %w", err)
	}
	stream, err := age.Decrypt(r, id)
	if err != nil {
		return nil, fmt.Errorf("open decrypt stream: %w", err)
	}
	out, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("read decrypt stream: %w", err)
	}
	return out, nil
}
