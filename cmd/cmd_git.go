package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

// gitCmd groups vault git helpers.
var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Backup and version the vault directory with git.",
}

// gitInitCmd initializes the vault as a git repository.
var gitInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the vault as a git repository.",
	RunE:  runGitInit,
}

// gitStatusCmd prints staged and unstaged changes inside the vault.
var gitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the vault git status.",
	RunE:  runGitStatus,
}

// gitSyncCmd stages every change, commits, and pushes to the configured remote.
var gitSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Stage every change, commit with an autosync message, and push to origin if configured.",
	RunE:  runGitSync,
}

// gitMessage is the commit subject used by sync.
var gitMessage string

func init() {
	gitSyncCmd.Flags().StringVarP(&gitMessage, "message", "m", "", "Commit message override; default is autosync with the local timestamp.")
	gitCmd.AddCommand(gitInitCmd)
	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitSyncCmd)
	rootCmd.AddCommand(gitCmd)
}

// runGitInit initializes a fresh repository in the vault root.
func runGitInit(cmd *cobra.Command, _ []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if _, err := git.PlainOpen(v.Dir); err == nil {
		return fmt.Errorf("vault already a git repository")
	}
	if _, err := git.PlainInit(v.Dir, false); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("git init: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Initialized git repository in %s\n", v.Dir)
	return nil
}

// runGitStatus prints the porcelain status of the vault repository.
func runGitStatus(cmd *cobra.Command, _ []string) error {
	repo, err := openVaultRepo()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("worktree: %w", err))
	}
	status, err := wt.Status()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("status: %w", err))
	}
	if status.IsClean() {
		fmt.Fprintln(cmd.OutOrStdout(), "clean")
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), status.String())
	return nil
}

// runGitSync stages everything, commits, and pushes to origin when present.
func runGitSync(cmd *cobra.Command, _ []string) error {
	repo, err := openVaultRepo()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("worktree: %w", err))
	}
	status, err := wt.Status()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("status: %w", err))
	}
	if status.IsClean() {
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to commit.")
		return tryPush(cmd, repo)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("git add: %w", err))
	}
	msg := gitMessage
	if msg == "" {
		msg = "autosync " + time.Now().Format("2006-01-02 15:04:05")
	}
	if _, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "midden", Email: "midden@localhost", When: time.Now()},
	}); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("git commit: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Committed: %s\n", msg)
	return tryPush(cmd, repo)
}

// tryPush pushes to origin when a remote is configured; missing remotes are not an error.
func tryPush(cmd *cobra.Command, repo *git.Repository) error {
	remotes, err := repo.Remotes()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("list remotes: %w", err))
	}
	if len(remotes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No remote configured; skipping push.")
		return nil
	}
	if err := repo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			fmt.Fprintln(cmd.OutOrStdout(), "Remote already up to date.")
			return nil
		}
		return errors.Join(ErrVault, fmt.Errorf("git push: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Pushed to origin.")
	return nil
}

// openVaultRepo opens the vault directory as a git repository.
func openVaultRepo() (*git.Repository, error) {
	v, err := openVaultRaw()
	if err != nil {
		return nil, err
	}
	repo, err := git.PlainOpen(v.Dir)
	if err != nil {
		return nil, errors.Join(ErrVault, fmt.Errorf("open vault repo at %s: %w", filepath.Clean(v.Dir), err))
	}
	return repo, nil
}
