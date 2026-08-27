package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/classify"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/internal/weave"
	"github.com/dcadolph/midden/llm"
)

// Identity markers. A verdict the person accepts or rejects is recorded as an
// ordinary entry carrying one of these lines, which is what makes the decision
// durable, auditable, and greppable like everything else in the vault.
const (
	samePrefix = vault.SameMarker
	diffPrefix = vault.DiffMarker
)

// Classify options.
var (
	classifyApply   bool
	classifyLimit   int
	classifyTimeout time.Duration
)

// classifyCmd proposes identity merges for a person to accept or reject.
var classifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Propose identity merges the arithmetic cannot make, for you to accept or reject.",
	Long: "Classify finds pairs that might be the same thing under two names: Will and William, or one " +
		"activity recorded under different titles across eras.\n\n" +
		"The boundary is strict. Arithmetic generates the candidate pairs; a model answers only yes or " +
		"no about each pair under a fixed schema, and an unparseable answer counts as no answer rather " +
		"than a guess. Every verdict is a proposal shown with its evidence, and nothing changes until " +
		"you accept it. An accepted merge is recorded as an ordinary entry, which weave and people then " +
		"read; a rejected one is recorded too, so the same pair is never proposed again. The model " +
		"never writes to the vault and the tool works fully without one.",
	RunE: runClassify,
}

func init() {
	classifyCmd.Flags().BoolVar(&classifyApply, "apply", false,
		"Review each proposal interactively and record accepts and rejects.")
	classifyCmd.Flags().IntVar(&classifyLimit, "top", 10, "Maximum pairs to judge per kind.")
	classifyCmd.Flags().DurationVar(&classifyTimeout, "call-timeout", 2*time.Minute, "Time limit per model call.")
	rootCmd.AddCommand(classifyCmd)
}

// runClassify generates candidates, judges the undecided ones, and either
// lists the proposals or walks them interactively.
func runClassify(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := entriesInRange(v, dateRange{})
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	decided := decidedPairs(entries)
	now := time.Now()

	threadOpts := weave.DefaultOptions(now)
	threadOpts.SameKeys, _ = loadEquivalences(entries)
	threads := weave.Threads(entries, threadOpts)
	peopleOpts := weave.DefaultPeopleOptions(now)
	_, peopleOpts.SameNames = loadEquivalences(entries)
	people := weave.People(entries, peopleOpts)

	var candidates []classify.Candidate
	for _, c := range append(classify.ThreadPairs(threads, classifyLimit), classify.PersonPairs(people, classifyLimit)...) {
		if !decided[c.MarkerKey()] {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to classify: no undecided pairs.")
		return nil
	}

	chat, err := llm.ChatterFromEnv()
	if err != nil {
		return errors.Join(ErrLLM, fmt.Errorf("pick chatter: %w", err))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Judging %d pair(s) with %s.\n", len(candidates), chat.Name())
	verdicts := classify.Judge(context.Background(), chat, candidates, classifyTimeout)

	if !classifyApply {
		writeVerdicts(cmd.OutOrStdout(), verdicts)
		fmt.Fprintln(cmd.ErrOrStderr(), "Proposals only; nothing was changed. Review with: midden classify --apply")
		return nil
	}
	return applyVerdicts(cmd, v, verdicts)
}

// writeVerdicts renders proposals with their evidence.
func writeVerdicts(w io.Writer, verdicts []classify.Verdict) {
	for _, vd := range verdicts {
		call := "no answer"
		switch {
		case vd.Same != nil && *vd.Same:
			call = "SAME"
		case vd.Same != nil:
			call = "different"
		}
		fmt.Fprintf(w, "\n[%s] %s  <->  %s\n", call, vd.Candidate.A, vd.Candidate.B)
		for line := range strings.SplitSeq(vd.Candidate.Evidence, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

// applyVerdicts walks proposals interactively. Accepting records a SAME entry,
// declining records a DIFF entry, and skipping records nothing so the pair
// returns next run. The model's own answer is shown but decides nothing.
func applyVerdicts(cmd *cobra.Command, v *vault.Vault, verdicts []classify.Verdict) error {
	scanner := bufio.NewScanner(cmd.InOrStdin())
	accepted, rejected := 0, 0
	for _, vd := range verdicts {
		writeVerdicts(cmd.ErrOrStderr(), []classify.Verdict{vd})
		fmt.Fprint(cmd.ErrOrStderr(), "Same thing? [y]es / [n]o / [s]kip: ")
		if !scanner.Scan() {
			break
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		var body string
		switch answer {
		case "y", "yes":
			body = fmt.Sprintf("%s and %s are the same.\n%s%s",
				vd.Candidate.A, vd.Candidate.B, samePrefix, vd.Candidate.MarkerKey())
			accepted++
		case "n", "no":
			body = fmt.Sprintf("%s and %s are different.\n%s%s",
				vd.Candidate.A, vd.Candidate.B, diffPrefix, vd.Candidate.MarkerKey())
			rejected++
		default:
			continue
		}
		entry := vault.Entry{Time: time.Now(), Tags: entryTags([]string{"identity"}), Body: body}
		if err := v.Append(entry); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("record verdict: %w", err))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read decisions: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Recorded %d merge(s) and %d distinction(s).\n", accepted, rejected)
	return nil
}

// decidedPairs collects the pairs already accepted or rejected, so a pair is
// proposed at most once ever.
func decidedPairs(entries []vault.Entry) map[string]bool {
	out := map[string]bool{}
	for _, e := range entries {
		if k := bodyMarker(e.Body, samePrefix); k != "" {
			out[k] = true
		}
		if k := bodyMarker(e.Body, diffPrefix); k != "" {
			out[k] = true
		}
	}
	return out
}

// loadEquivalences reads accepted identity merges into the maps weave and
// people consume. The second key of a marker maps to the first, so every
// accepted pair counts under one canonical form.
func loadEquivalences(entries []vault.Entry) (threadKeys map[string]string, personNames map[string]string) {
	threadKeys = map[string]string{}
	personNames = map[string]string{}
	for _, e := range entries {
		marker := bodyMarker(e.Body, samePrefix)
		if marker == "" {
			continue
		}
		parts := strings.SplitN(marker, "|", 3)
		if len(parts) != 3 {
			continue
		}
		kind, a, b := parts[0], parts[1], parts[2]
		switch classify.Kind(kind) {
		case classify.KindThread:
			threadKeys[b] = a
		case classify.KindPerson:
			personNames[b] = a
		}
	}
	return threadKeys, personNames
}
