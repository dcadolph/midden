package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// grepCmd shells out to ripgrep or grep with the vault root pre-applied.
var grepCmd = &cobra.Command{
	Use:                "grep [args...]",
	Short:              "Run ripgrep or grep over the vault.",
	Long:               "Run ripgrep when available, falling back to grep -RHn, with the vault directory pre-applied as the search root.\n\nAll arguments are forwarded to the underlying tool. The vault path is appended last so explicit file or directory arguments still take precedence.",
	DisableFlagParsing: true,
	RunE:               runGrep,
}

func init() {
	rootCmd.AddCommand(grepCmd)
}

// runGrep dispatches the search to ripgrep when present, otherwise to plain grep.
// Encrypted vaults are refused because the on-disk day files are ciphertext.
// Exit status 1 from the search tool means no matches and maps to ErrNotFound;
// other non-zero statuses surface as plain errors.
func runGrep(_ *cobra.Command, args []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if v.IsEncrypted() {
		return errors.New("vault is encrypted: grep searches ciphertext, use `midden search` instead")
	}
	binary, base, err := pickGrepBinary()
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, append(append([]string{}, base...), append(args, v.Dir)...)...) //nolint:gosec // Grep binary resolved via exec.LookPath.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ee.ExitCode() == 1 {
				return ErrNotFound
			}
			return fmt.Errorf("%s exited with status %d", filepath.Base(binary), ee.ExitCode())
		}
		return fmt.Errorf("run %s: %w", binary, err)
	}
	return nil
}

// pickGrepBinary returns the search binary and its default arguments.
// It prefers ripgrep when available because of its speed and built-in directory recursion.
func pickGrepBinary() (string, []string, error) {
	if path, err := exec.LookPath("rg"); err == nil {
		return path, nil, nil
	}
	if path, err := exec.LookPath("grep"); err == nil {
		return path, []string{"-RHn"}, nil
	}
	return "", nil, errors.New("no search tool found: install ripgrep (rg) or grep")
}
