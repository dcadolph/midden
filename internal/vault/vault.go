// Package vault handles all read and write operations against the markdown journal directory.
package vault

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dcadolph/midden/internal/crypt"
)

// encryptToBytes returns the ciphertext bytes for plaintext using passphrase.
func encryptToBytes(passphrase string, plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := crypt.Encrypt(&buf, passphrase, plaintext); err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return buf.Bytes(), nil
}

// decryptFromBytes returns the plaintext bytes for ciphertext using passphrase.
func decryptFromBytes(passphrase string, ciphertext []byte) ([]byte, error) {
	plain, err := crypt.Decrypt(bytes.NewReader(ciphertext), passphrase)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plain, nil
}

// isEncryptedBytes reports whether data starts with the age magic header.
func isEncryptedBytes(data []byte) bool {
	return crypt.IsEncrypted(data)
}

// EnvHome is the environment variable that overrides the default vault directory.
const EnvHome = "MIDDEN_HOME"

// DefaultDir is the path used when EnvHome is unset and no override is given.
// It is resolved relative to the calling user's home directory at call time.
const DefaultDir = "midden"

// lockFile is the name of the advisory lock file inside the vault root.
const lockFile = ".midden.lock"

// encryptedMarker is the in-vault marker that signals every day file is encrypted at rest.
const encryptedMarker = ".midden.encrypted"

// Vault is a handle to a journal directory on disk.
type Vault struct {
	// Dir is the absolute path to the root of the journal directory.
	Dir string
	// Passphrase, when non-empty, is used to encrypt and decrypt day files at rest.
	// Callers set this with WithPassphrase after Open.
	Passphrase string
}

// LockPath returns the absolute path to the vault advisory lock file.
func (v *Vault) LockPath() string {
	return filepath.Join(v.Dir, lockFile)
}

// MarkerPath returns the absolute path to the encryption marker file.
func (v *Vault) MarkerPath() string {
	return filepath.Join(v.Dir, encryptedMarker)
}

// IsEncrypted reports whether the vault root carries the encryption marker.
// It does not validate the passphrase; callers must verify with TryUnlock.
func (v *Vault) IsEncrypted() bool {
	_, err := os.Stat(v.MarkerPath())
	return err == nil
}

// WithPassphrase returns a copy of the vault with the passphrase set.
// The returned value should be used for every read or write against an encrypted vault.
func (v *Vault) WithPassphrase(p string) *Vault {
	cp := *v
	cp.Passphrase = p
	return &cp
}

// SetEncrypted writes the encryption marker file, marking the vault encrypted from this point on.
// Existing day files are not transformed; callers must re-encrypt them separately.
func (v *Vault) SetEncrypted() error {
	if err := os.WriteFile(v.MarkerPath(), []byte{}, 0o600); err != nil {
		return fmt.Errorf("write encryption marker: %w", err)
	}
	return nil
}

// ClearEncrypted removes the encryption marker file.
// Existing day files are not transformed; callers must decrypt them separately.
func (v *Vault) ClearEncrypted() error {
	if err := os.Remove(v.MarkerPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove encryption marker: %w", err)
	}
	return nil
}

// Open returns a Vault rooted at the given directory.
// An empty override defers to the environment variable and then to DefaultDir under the user home.
// The directory and any required parents are created if missing.
func Open(override string) (*Vault, error) {
	dir, err := resolveDir(override)
	if err != nil {
		return nil, fmt.Errorf("resolve vault directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create vault directory: %w", err)
	}
	return &Vault{Dir: dir}, nil
}

// DayPath returns the absolute path to the markdown file for the given local date.
// The file is not guaranteed to exist.
func (v *Vault) DayPath(day time.Time) string {
	return filepath.Join(
		v.Dir,
		fmt.Sprintf("%04d", day.Year()),
		fmt.Sprintf("%02d", int(day.Month())),
		fmt.Sprintf("%02d.md", day.Day()),
	)
}

// EnsureDayFile creates the day file for the given local date if it does not exist.
// The file is initialized with a single-line date header and, when the vault is
// encrypted, the header is written through the encryption layer so the file at
// rest stays sealed.
// It returns the absolute path to the file.
func (v *Vault) EnsureDayFile(day time.Time) (string, error) {
	path := v.DayPath(day)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat day file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create day directory: %w", err)
	}
	header := fmt.Sprintf("# %s\n\n", day.Format("2006-01-02"))
	if err := v.writeDayBytes(path, []byte(header)); err != nil {
		return "", err
	}
	return path, nil
}

// WriteBytes writes contents to path through the encryption layer when the vault carries a passphrase.
// It exists so callers performing one-shot rewrites (encrypt, decrypt) can share the
// same code path as Append without reaching into vault internals.
func (v *Vault) WriteBytes(path string, contents []byte) error {
	return v.writeDayBytes(path, contents)
}

// ReadBytes returns the plaintext bytes at path, decrypting on the fly when needed.
func (v *Vault) ReadBytes(path string) ([]byte, error) {
	return v.readDayBytes(path)
}

// writeDayBytes writes contents to path. When the vault carries a passphrase the
// bytes are encrypted before being written.
func (v *Vault) writeDayBytes(path string, contents []byte) error {
	mode := os.FileMode(0o644)
	if v.Passphrase != "" {
		mode = 0o600
		buf, err := encryptToBytes(v.Passphrase, contents)
		if err != nil {
			return err
		}
		contents = buf
	}
	if err := os.WriteFile(path, contents, mode); err != nil {
		return fmt.Errorf("write day file %s: %w", path, err)
	}
	return nil
}

// readDayBytes reads path and returns the plaintext bytes.
// Encrypted files are recognized by their age magic header regardless of the
// vault marker so files in transit between encryption states still decode.
func (v *Vault) readDayBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read day file %s: %w", path, err)
	}
	if !isEncryptedBytes(data) {
		return data, nil
	}
	if v.Passphrase == "" {
		return nil, fmt.Errorf("day file %s is encrypted but no passphrase is set", path)
	}
	return decryptFromBytes(v.Passphrase, data)
}

// resolveDir picks the vault directory using override then env then default.
func resolveDir(override string) (string, error) {
	if override != "" {
		return absolute(override)
	}
	if env := os.Getenv(EnvHome); env != "" {
		return absolute(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, DefaultDir), nil
}

// absolute expands the path and converts it to an absolute path.
func absolute(p string) (string, error) {
	if len(p) > 0 && p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home: %w", err)
		}
		p = filepath.Join(home, p[1:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	return abs, nil
}
