package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/index"
	"github.com/dcadolph/midden/internal/llm"
	"github.com/dcadolph/midden/internal/vault"
)

// recallTopK caps the number of entries returned.
var recallTopK int

// recallCmd performs a semantic search over the indexed entries.
var recallCmd = &cobra.Command{
	Use:   "recall [query...]",
	Short: "Semantic search over indexed entries (run midden reindex first).",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRecall,
}

func init() {
	recallCmd.Flags().IntVarP(&recallTopK, "top", "k", 5, "Number of entries to return.")
	rootCmd.AddCommand(recallCmd)
}

// runRecall embeds the query, searches the index, and prints the top entries.
func runRecall(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	idx, err := index.Load(filepath.Join(v.Dir, index.Filename))
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("load index: %w", err))
	}
	if len(idx.Entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no index found: run `midden reindex` first"))
	}
	emb, err := llm.EmbedderFromEnv()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("pick embedder: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	q := strings.Join(args, " ")
	vecs, err := emb.Embed(ctx, []string{q})
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("embed query: %w", err))
	}
	matches := idx.Search(vecs[0], recallTopK)
	out := make([]vault.Entry, len(matches))
	for i, m := range matches {
		out[i] = vault.Entry{Time: m.Entry.Time, Tags: m.Entry.Tags, Body: m.Entry.Body}
	}
	return printEntries(cmd.OutOrStdout(), out)
}
