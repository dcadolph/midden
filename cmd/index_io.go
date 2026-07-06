package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/llm"
)

// indexPath returns the on-disk location of the recall index for the vault.
func indexPath(v *vault.Vault) string {
	return filepath.Join(v.Dir, index.Filename)
}

// loadIndex reads the recall index through the vault encryption layer so an
// encrypted vault keeps its index sealed at rest. A missing index returns an
// empty Index and no error.
func loadIndex(v *vault.Vault) (*index.Index, error) {
	path := indexPath(v)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &index.Index{}, nil
		}
		return nil, fmt.Errorf("stat index: %w", err)
	}
	data, err := v.ReadBytes(path)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	return index.Decode(data)
}

// saveIndex writes the recall index through the vault encryption layer.
func saveIndex(v *vault.Vault, idx *index.Index) error {
	data, err := idx.Encode()
	if err != nil {
		return err
	}
	if err := v.WriteBytes(indexPath(v), data); err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	return nil
}

// recallContext bundles everything a recall-backed command needs to search the index.
type recallContext struct {
	// Index is the loaded recall index.
	Index *index.Index
	// Embedder produced the query vector and identifies the provider.
	Embedder llm.Embedder
	// Query is the embedded query vector.
	Query []float32
}

// embedRecallQuery loads the index, checks it against the active embedder,
// warns when the index is older than the newest day file, and embeds the query.
func embedRecallQuery(cmd *cobra.Command, v *vault.Vault, query string, timeout time.Duration) (*recallContext, error) {
	idx, err := loadIndex(v)
	if err != nil {
		return nil, errors.Join(ErrVault, fmt.Errorf("load index: %w", err))
	}
	if len(idx.Entries) == 0 {
		return nil, errors.Join(ErrNotFound, fmt.Errorf("no index found: run `midden reindex` first"))
	}
	emb, err := llm.EmbedderFromEnv()
	if err != nil {
		return nil, errors.Join(ErrLLM, fmt.Errorf("pick embedder: %w", err))
	}
	if idx.Provider != emb.Name() {
		return nil, errors.Join(ErrLLM, fmt.Errorf(
			"index was built with %s but the current embedder is %s: run `midden reindex`", idx.Provider, emb.Name()))
	}
	warnStaleIndex(cmd, v, idx)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	vecs, err := emb.Embed(ctx, []string{query})
	if err != nil {
		return nil, errors.Join(ErrLLM, fmt.Errorf("embed query: %w", err))
	}
	if len(vecs) == 0 || (idx.Dim != 0 && len(vecs[0]) != idx.Dim) {
		return nil, errors.Join(ErrLLM, errors.New(
			"query embedding dimension does not match the index: run `midden reindex`"))
	}
	return &recallContext{Index: idx, Embedder: emb, Query: vecs[0]}, nil
}

// warnStaleIndex prints a reindex hint when a day file is newer than the index build time.
func warnStaleIndex(cmd *cobra.Command, v *vault.Vault, idx *index.Index) {
	days, err := v.ListDays()
	if err != nil || len(days) == 0 || idx.BuiltAt.IsZero() {
		return
	}
	fi, err := os.Stat(v.DayPath(days[len(days)-1]))
	if err != nil {
		return
	}
	if fi.ModTime().After(idx.BuiltAt) {
		fmt.Fprintln(cmd.ErrOrStderr(), "note: entries changed since the last reindex; run `midden reindex` to refresh recall")
	}
}
