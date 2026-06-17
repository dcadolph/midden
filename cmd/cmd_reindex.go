package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/index"
	"github.com/dcadolph/midden/internal/llm"
	"github.com/dcadolph/midden/internal/vault"
)

// reindexBatch sets the number of entries embedded per provider call.
var reindexBatch int

// reindexCmd rebuilds the vector index that backs midden recall.
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild the vector index used by midden recall.",
	RunE:  runReindex,
}

func init() {
	reindexCmd.Flags().IntVar(&reindexBatch, "batch", 64, "Maximum entries embedded per provider call.")
	rootCmd.AddCommand(reindexCmd)
}

// runReindex walks every entry, embeds the body, and writes the index to disk.
func runReindex(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	emb, err := llm.EmbedderFromEnv()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("pick embedder: %w", err))
	}
	entries, err := allEntries(v)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	idx := &index.Index{Provider: emb.Name(), BuiltAt: time.Now()}
	for start := 0; start < len(entries); start += reindexBatch {
		end := start + reindexBatch
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[start:end]
		texts := make([]string, len(batch))
		for i, e := range batch {
			texts[i] = e.Body
		}
		vecs, err := emb.Embed(ctx, texts)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("embed batch: %w", err))
		}
		for i, vec := range vecs {
			idx.Entries = append(idx.Entries, index.Entry{
				Time:      batch[i].Time,
				Tags:      batch[i].Tags,
				Body:      batch[i].Body,
				Embedding: vec,
			})
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Embedded %d/%d\n", end, len(entries))
	}
	if len(idx.Entries) > 0 {
		idx.Dim = len(idx.Entries[0].Embedding)
	}
	if err := idx.Save(filepath.Join(v.Dir, index.Filename)); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote index with %d entries via %s\n", len(idx.Entries), emb.Name())
	return nil
}

// allEntries returns every entry across the vault in chronological order.
func allEntries(v *vault.Vault) ([]vault.Entry, error) {
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []vault.Entry
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	return out, nil
}
