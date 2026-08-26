package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/util"
	"github.com/dcadolph/midden/internal/vault"
)

// commitPrefix marks the line recording which commit an entry came from, so
// repeated ingests recognize what the vault already holds.
const commitPrefix = "GIT-COMMIT: "

// ingestGitCmd ingests commit history from local repositories.
var ingestGitCmd = &cobra.Command{
	Use:   "git [repo...]",
	Short: "Append commit history from local git repositories as entries.",
	Long: "Ingest git reads commit history and appends one entry per commit.\n\n" +
		"Commit history is a record of what you were working on and when, which calendar exports do not " +
		"carry. Author dates are used rather than commit dates, so rebased or cherry-picked work still " +
		"lands on the day it was written. Merge commits are skipped unless asked for, and commits already " +
		"in the vault are skipped, so ingesting the same repository twice is safe.",
	Args: cobra.MinimumNArgs(1),
	RunE: runIngestGit,
}

// Git ingestion options.
var (
	ingestGitSince  string
	ingestGitUntil  string
	ingestGitAuthor []string
	ingestGitTag    []string
	ingestGitRev    string
	ingestGitMerges bool
	ingestGitStat   bool
)

func init() {
	ingestGitCmd.Flags().StringVar(&ingestGitSince, "since", "", "Only ingest commits authored on or after this date.")
	ingestGitCmd.Flags().StringVar(&ingestGitUntil, "until", "", "Only ingest commits authored on or before this date.")
	ingestGitCmd.Flags().StringSliceVar(&ingestGitAuthor, "author", nil,
		"Only ingest commits whose author name or email contains one of these values.")
	ingestGitCmd.Flags().StringSliceVarP(&ingestGitTag, "tag", "t", []string{"git"}, "Tags to attach to every ingested commit.")
	ingestGitCmd.Flags().StringVar(&ingestGitRev, "rev", "", "Revision to walk (default: the repository HEAD).")
	ingestGitCmd.Flags().BoolVar(&ingestGitMerges, "merges", false, "Include merge commits.")
	ingestGitCmd.Flags().BoolVar(&ingestGitStat, "stat", false, "Include changed-file and line counts, at the cost of a diff per commit.")
	ingestCmd.AddCommand(ingestGitCmd)
}

// gitFilter is the set of conditions a commit must satisfy to be ingested.
type gitFilter struct {
	// Span bounds the author dates accepted.
	Span dateRange
	// Authors matches against the commit author name and email; empty accepts any.
	Authors []string
	// Rev is the revision to walk, or empty for HEAD.
	Rev string
	// Merges includes merge commits when set.
	Merges bool
	// Stat includes changed-file and line counts when set.
	Stat bool
}

// runIngestGit walks each repository and appends the commits the vault does not
// already hold.
func runIngestGit(cmd *cobra.Command, args []string) error {
	span, err := resolveDateRange(ingestGitSince, ingestGitUntil)
	if err != nil {
		return err
	}
	filter := gitFilter{
		Span:    span,
		Authors: ingestGitAuthor,
		Rev:     ingestGitRev,
		Merges:  ingestGitMerges,
		Stat:    ingestGitStat,
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	seen, err := existingCommits(v, span)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault for existing commits: %w", err))
	}
	tags := entryTags(ingestGitTag)

	var entries []vault.Entry
	dupes, failed := 0, 0
	for _, repo := range args {
		commits, err := readCommits(repo, filter)
		if err != nil {
			// One unreadable repository must not cost the user the other thirty-nine.
			// An empty repository has no HEAD at all, and a backfill sweeping a source
			// directory will meet those routinely.
			failed++
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: skipped (%v)\n", repoName(repo), err)
			continue
		}
		kept := 0
		for _, c := range commits {
			if seen[c.SHA] {
				dupes++
				continue
			}
			seen[c.SHA] = true
			entries = append(entries, vault.Entry{Time: c.When, Tags: tags, Body: c.Body})
			kept++
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %d commit(s) in range, %d new\n", repoName(repo), len(commits), kept)
	}
	if failed == len(args) {
		return errors.Join(ErrGit, fmt.Errorf("every repository failed to read (%d of %d)", failed, len(args)))
	}
	if err := v.AppendAll(entries); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append commits: %w", err))
	}
	if dupes > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d commit(s) already in the vault.\n", dupes)
	}
	if failed > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d unreadable repository/repositories.\n", failed)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d commit(s) into the vault (%s).\n", len(entries), span.Label())
	return nil
}

// gitCommit is one commit rendered for the vault.
type gitCommit struct {
	// SHA is the full commit hash, which identifies the commit for deduplication.
	SHA string
	// When is the author timestamp, which is when the work was actually written.
	When time.Time
	// Body is the rendered entry text.
	Body string
}

// readCommits walks the repository and returns the commits passing the filter,
// ordered oldest first.
func readCommits(path string, filter gitFilter) ([]gitCommit, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	start, err := resolveRev(repo, filter.Rev)
	if err != nil {
		return nil, err
	}
	iter, err := repo.Log(&git.LogOptions{From: start, Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	defer iter.Close()
	name := repoName(path)
	var out []gitCommit
	err = iter.ForEach(func(c *object.Commit) error {
		when := c.Author.When.Local()
		if !filter.accepts(c, when) {
			return nil
		}
		out = append(out, gitCommit{SHA: c.Hash.String(), When: when, Body: formatCommit(c, name, filter.Stat)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk log: %w", err)
	}
	// The log walks newest first; the vault reads better oldest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// accepts reports whether the commit passes every condition in the filter.
func (f gitFilter) accepts(c *object.Commit, when time.Time) bool {
	if !f.Merges && c.NumParents() > 1 {
		return false
	}
	if !f.Span.From.IsZero() && when.Before(f.Span.From) {
		return false
	}
	if !f.Span.To.IsZero() && when.After(f.Span.To) {
		return false
	}
	if len(f.Authors) == 0 {
		return true
	}
	who := c.Author.Name + " <" + c.Author.Email + ">"
	for _, want := range f.Authors {
		if util.ContainsFold(who, want) {
			return true
		}
	}
	return false
}

// resolveRev returns the hash to start the log walk from.
func resolveRev(repo *git.Repository, rev string) (plumbing.Hash, error) {
	if rev == "" {
		head, err := repo.Head()
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("resolve HEAD: %w", err)
		}
		return head.Hash(), nil
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("resolve %q: %w", rev, err)
	}
	return *hash, nil
}

// formatCommit renders the entry body for one commit. The subject leads so the
// entry reads as a line of history, and the trailing hash line makes the commit
// recognizable on later runs.
func formatCommit(c *object.Commit, repo string, withStat bool) string {
	message := strings.TrimSpace(c.Message)
	subject, rest, _ := strings.Cut(message, "\n")
	var b strings.Builder
	b.WriteString(strings.TrimSpace(subject))
	fmt.Fprintf(&b, "\nRepo: %s", repo)
	fmt.Fprintf(&b, "\nAuthor: %s", c.Author.Name)
	if withStat {
		if summary := statSummary(c); summary != "" {
			fmt.Fprintf(&b, "\nChanges: %s", summary)
		}
	}
	if body := strings.TrimSpace(rest); body != "" {
		fmt.Fprintf(&b, "\n\n%s", body)
	}
	fmt.Fprintf(&b, "\n%s%s", commitPrefix, c.Hash.String())
	return b.String()
}

// statSummary renders the changed-file and line counts for a commit, or empty
// when the diff cannot be computed.
func statSummary(c *object.Commit) string {
	stats, err := c.Stats()
	if err != nil {
		return ""
	}
	added, deleted := 0, 0
	for _, s := range stats {
		added += s.Addition
		deleted += s.Deletion
	}
	return fmt.Sprintf("%d file(s), +%d/-%d", len(stats), added, deleted)
}

// repoName returns the directory name identifying the repository in an entry.
func repoName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(filepath.Clean(abs))
}

// existingCommits collects the commit hashes already recorded in the vault
// across the window.
func existingCommits(v *vault.Vault, span dateRange) (map[string]bool, error) {
	seen := map[string]bool{}
	err := forEachEntryInRange(v, span, func(e vault.Entry) {
		if sha := bodyMarker(e.Body, commitPrefix); sha != "" {
			seen[sha] = true
		}
	})
	if err != nil {
		return nil, err
	}
	return seen, nil
}
