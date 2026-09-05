package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// addTags are the tags applied to the new entry.
var addTags []string

// addCmd appends a new entry to today's day file.
var addCmd = &cobra.Command{
	Use:   "add [text]",
	Short: "Append an entry to today's day file.",
	Long: "Append an entry to today's day file.\n\n" +
		"With no arguments and a TTY, opens $VISUAL or $EDITOR for the body.\n" +
		"With no arguments and a piped stdin, reads the body from stdin.",
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Tag to attach to the entry (may be repeated or comma-separated).")
	rootCmd.AddCommand(addCmd)
}

// runAdd executes the add subcommand.
func runAdd(cmd *cobra.Command, args []string) error {
	body, err := resolveAddBody(args)
	if err != nil {
		return err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("entry body is empty")
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entry := vault.Entry{
		Time: time.Now(),
		Tags: entryTags(addTags),
		Body: body,
	}
	if err := v.Append(entry); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append entry: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Appended entry at %s\n", entry.Time.Format(layoutDateTime))
	return nil
}

// resolveAddBody returns the body text to attach to the new entry.
// Positional arguments win; otherwise piped stdin is consumed; otherwise the editor is invoked.
func resolveAddBody(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	info, err := os.Stdin.Stat()
	if err == nil && info.Mode()&os.ModeCharDevice == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
	return readBodyFromEditor()
}

// readBodyFromEditor opens a temp file in the user's editor and returns the trimmed contents.
func readBodyFromEditor() (string, error) {
	editor := chooseEditor()
	tmp, err := os.CreateTemp("", "midden-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.WriteString("\n"); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	if err := runEditor(editor, path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path) //nolint:gosec // Temp file created above.
	if err != nil {
		return "", fmt.Errorf("read temp file: %w", err)
	}
	return string(data), nil
}

// chooseEditor picks the user's preferred editor, falling back to vi.
// The config file editor wins, then MIDDEN_EDITOR, VISUAL, and EDITOR.
func chooseEditor() string {
	if editor := resolveEditorOverride(); editor != "" {
		return editor
	}
	for _, env := range []string{"MIDDEN_EDITOR", "VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return "vi"
}
