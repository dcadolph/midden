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
		return runEditor(editor, path)
	}
	return editEncryptedDay(v, path, editor)
}

// runEditor opens path in the given editor wired to the current terminal.
func runEditor(editor, path string) error {
	c := exec.Command(editor, path) //nolint:gosec // Editor comes from user config or environment.
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return errors.Join(ErrEditor, fmt.Errorf("editor %s exited: %w", filepath.Base(editor), err))
	}
	return nil
}

// editEncryptedDay decrypts the day file to a private temp file outside the
// vault, runs the editor, re-encrypts the result back into the day file, and
// removes the temp file. When the day file changes on disk while the editor is
// open (for example a concurrent append), the re-encrypt is refused and the
// edited plaintext is kept for manual recovery instead of clobbering the change.
func editEncryptedDay(v vaultEditor, path, editor string) error {
	plain, err := v.ReadBytes(path)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("decrypt %s: %w", path, err))
	}
	before, err := os.Stat(path)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("stat %s: %w", path, err))
	}
	tmp, err := os.CreateTemp("", ".midden-edit-*.md")
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(plain); err != nil {
		_ = tmp.Close()
		return errors.Join(ErrVault, fmt.Errorf("write temp file: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("close temp file: %w", err))
	}
	if err := runEditor(editor, tmpPath); err != nil {
		return err
	}
	updated, err := os.ReadFile(tmpPath) //nolint:gosec // Temp file created above.
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read temp file: %w", err))
	}
	after, err := os.Stat(path)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("stat %s: %w", path, err))
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		keepTmp = true
		return errors.Join(ErrVault, fmt.Errorf(
			"day file %s changed while the editor was open; your edit is preserved at %s", path, tmpPath))
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
