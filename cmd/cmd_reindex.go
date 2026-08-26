package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/llm"
)

// Reindex tuning.
var (
	// reindexBatch sets the number of entries embedded per provider call.
	reindexBatch int
	// reindexTimeout bounds a single provider call rather than the whole run,
	// because a backfilled vault takes far longer to embed than any one batch.
	reindexTimeout time.Duration
	// reindexFull re-embeds every entry instead of reusing cached vectors.
	reindexFull bool
)

// reindexConfig is the tuning one rebuild runs under. It is passed rather than
// read from the flag variables so the embedding logic has no global state.
type reindexConfig struct {
	// Batch is the maximum number of entries embedded per provider call.
	Batch int
	// Timeout bounds a single provider call.
	Timeout time.Duration
	// Full re-embeds every entry instead of reusing cached vectors.
	Full bool
}

// reindexCheckpointEvery is how many batches complete between index writes.
// Checkpointing means an interrupted rebuild keeps the embeddings it already
// paid for, and the next run picks up from there rather than starting over.
const reindexCheckpointEvery = 20

// reindexCmd rebuilds the vector index that backs midden recall.
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild the vector index used by midden recall.",
	Long: "Reindex embeds every entry in the vault so recall and chat can search it.\n\n" +
		"Entries whose text is already indexed reuse their existing vector, so a rebuild after adding " +
		"a day costs one provider call rather than re-embedding the whole vault. Progress is written to " +
		"the index periodically, so an interrupted rebuild resumes instead of starting over. " +
		"Use --full to re-embed everything, which is needed only after changing embedding provider.",
	RunE: runReindex,
}

func init() {
	reindexCmd.Flags().IntVar(&reindexBatch, "batch", 64, "Maximum entries embedded per provider call.")
	reindexCmd.Flags().DurationVar(&reindexTimeout, "batch-timeout", 2*time.Minute, "Time limit for a single provider call.")
	reindexCmd.Flags().BoolVar(&reindexFull, "full", false, "Re-embed every entry instead of reusing cached vectors.")
	rootCmd.AddCommand(reindexCmd)
}

// runReindex walks every entry, embeds the bodies it has no vector for, and
// writes the index to disk.
func runReindex(cmd *cobra.Command, _ []string) error {
	if reindexBatch < 1 {
		return fmt.Errorf("--batch must be at least 1")
	}
	if reindexTimeout <= 0 {
		return fmt.Errorf("--batch-timeout must be positive")
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	emb, err := llm.EmbedderFromEnv()
	if err != nil {
		return errors.Join(ErrLLM, fmt.Errorf("pick embedder: %w", err))
	}
	entries, err := allEntries(v)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	cfg := reindexConfig{Batch: reindexBatch, Timeout: reindexTimeout, Full: reindexFull}
	cached, err := cachedEmbeddings(v, emb.Name(), cfg)
	if err != nil {
		return errors.Join(ErrVault, err)
	}

	idx := &index.Index{Provider: emb.Name(), BuiltAt: time.Now()}
	idx.Entries = make([]index.Entry, 0, len(entries))
	pending := make([]int, 0, len(entries))
	for _, e := range entries {
		entry := index.Entry{Time: e.Time, Tags: e.Tags, Body: e.Body}
		if vec, ok := cached[index.ContentHash(e.Body)]; ok {
			entry.Embedding = vec
		} else {
			pending = append(pending, len(idx.Entries))
		}
		idx.Entries = append(idx.Entries, entry)
	}
	reused := len(idx.Entries) - len(pending)
	fmt.Fprintf(cmd.ErrOrStderr(), "Indexing %d entries: %d reused, %d to embed via %s\n",
		len(idx.Entries), reused, len(pending), emb.Name())

	if err := embedPending(cmd, v, emb, idx, pending, cfg); err != nil {
		return err
	}
	setDim(idx)
	if err := saveIndex(v, idx); err != nil {
		return errors.Join(ErrVault, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote index with %d entries via %s (%d reused, %d embedded)\n",
		len(idx.Entries), emb.Name(), reused, len(pending))
	return nil
}

// embedPending embeds the entries at the given positions in batches, writing a
// checkpoint as it goes. A failed batch leaves the checkpoint in place so the
// embeddings already bought are not lost.
func embedPending(
	cmd *cobra.Command,
	v *vault.Vault,
	emb llm.Embedder,
	idx *index.Index,
	pending []int,
	cfg reindexConfig,
) error {
	done := 0
	for start := 0; start < len(pending); start += cfg.Batch {
		end := min(start+cfg.Batch, len(pending))
		positions := pending[start:end]
		texts := make([]string, len(positions))
		for i, pos := range positions {
			texts[i] = idx.Entries[pos].Body
		}
		vecs, err := embedBatch(emb, texts, cfg.Timeout)
		if err != nil {
			checkpoint(cmd, v, idx)
			return errors.Join(ErrLLM, fmt.Errorf("embed entries %d-%d of %d: %w", start+1, end, len(pending), err))
		}
		if len(vecs) != len(positions) {
			checkpoint(cmd, v, idx)
			return errors.Join(ErrLLM, fmt.Errorf("embedder returned %d vectors for %d entries", len(vecs), len(positions)))
		}
		for i, pos := range positions {
			idx.Entries[pos].Embedding = vecs[i]
		}
		done = end
		fmt.Fprintf(cmd.ErrOrStderr(), "Embedded %d/%d\n", done, len(pending))
		if batchNum := (start / cfg.Batch) + 1; batchNum%reindexCheckpointEvery == 0 {
			checkpoint(cmd, v, idx)
		}
	}
	return nil
}

// embedBatch runs one provider call under its own deadline.
func embedBatch(emb llm.Embedder, texts []string, timeout time.Duration) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return emb.Embed(ctx, texts)
}

// checkpoint writes the index as it currently stands, dropping entries with no
// vector yet so what lands on disk is a smaller but valid index. A failure to
// write is reported and otherwise ignored, since the caller is either mid-run
// or already returning a more useful error.
func checkpoint(cmd *cobra.Command, v *vault.Vault, idx *index.Index) {
	partial := &index.Index{Provider: idx.Provider, BuiltAt: idx.BuiltAt}
	for _, e := range idx.Entries {
		if len(e.Embedding) > 0 {
			partial.Entries = append(partial.Entries, e)
		}
	}
	if len(partial.Entries) == 0 {
		return
	}
	setDim(partial)
	if err := saveIndex(v, partial); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not checkpoint the index: %v\n", err)
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Checkpointed %d embedded entries.\n", len(partial.Entries))
}

// cachedEmbeddings returns the vectors already on disk for reuse, or nothing
// when a full rebuild was asked for or the stored index came from a different
// embedder. Vectors from another provider have their own geometry and
// dimension, so mixing them would silently corrupt every later search.
func cachedEmbeddings(v *vault.Vault, provider string, cfg reindexConfig) (map[string][]float32, error) {
	if cfg.Full {
		return nil, nil
	}
	idx, err := loadIndex(v)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}
	if idx.Provider != provider {
		return nil, nil
	}
	return idx.EmbeddingsByContent(), nil
}

// setDim records the embedding width from the first vector present.
func setDim(idx *index.Index) {
	for _, e := range idx.Entries {
		if len(e.Embedding) > 0 {
			idx.Dim = len(e.Embedding)
			return
		}
	}
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
