package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
)

// editCmd opens a specific day file in the editor.
var editCmd = &cobra.Command{
	Use:   "edit [date]",
	Short: "Open a day file in the editor (default today).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

// runEdit resolves the date, ensures the day file exists, and shells out to the user's editor.
// Encrypted vaults decrypt to a secured temp file, edit there, and re-encrypt on save.
func runEdit(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	day := time.Now()
	if len(args) == 1 {
		t, err := dateutil.Parse(args[0])
		if err != nil {
			return err
		}
		day = t
	}
	path, err := v.EnsureDayFile(day)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("ensure day: %w", err))
	}
	editor := chooseEditor()
	if v.Passphrase == "" {
		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return errors.Join(ErrEditor, fmt.Errorf("editor %s exited: %w", filepath.Base(editor), err))
		}
		return nil
	}
	return editEncryptedDay(v, path, editor)
}

// editEncryptedDay decrypts the day file to a temp file inside the vault, runs the editor,
// re-encrypts the result back into the day file, and removes the temp file.
func editEncryptedDay(v vaultEditor, path, editor string) error {
	plain, err := v.ReadBytes(path)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("decrypt %s: %w", path, err))
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".midden-edit-*.md")
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(plain); err != nil {
		tmp.Close()
		return errors.Join(ErrVault, fmt.Errorf("write temp file: %w", err))
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return errors.Join(ErrVault, fmt.Errorf("chmod temp file: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("close temp file: %w", err))
	}
	c := exec.Command(editor, tmpPath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return errors.Join(ErrEditor, fmt.Errorf("editor %s exited: %w", filepath.Base(editor), err))
	}
	updated, err := os.ReadFile(tmpPath)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read temp file: %w", err))
	}
	if err := v.WriteBytes(path, updated); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("write %s: %w", path, err))
	}
	return nil
}

// vaultEditor isolates the vault methods edit uses so tests can swap in a fake.
type vaultEditor interface {
	ReadBytes(path string) ([]byte, error)
	WriteBytes(path string, contents []byte) error
}
