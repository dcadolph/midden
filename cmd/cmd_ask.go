package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/interview"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/internal/weave"
)

// askedPrefix marks the line recording which question an entry answers, so a
// question is never put twice.
const askedPrefix = "MIDDEN-ASKED: "

// askedPrefixLabel introduces the question an answer was given to, kept out of
// the headline so the entry reads as the person's own words.
const askedPrefixLabel = "In answer to: "

// Ask options.
var (
	askAnswer      string
	askCount       int
	askTags        []string
	askInteractive bool
	askList        bool
	askSkip        bool
)

// askCmd puts a question the record cannot answer about itself.
var askCmd = &cobra.Command{
	Use:   "ask",
	Short: "Answer a question about a gap in your own record.",
	Long: "Ask puts one question drawn from what the record proves is missing.\n\n" +
		"Imported history reconstructs where you were and what you produced, because calendars and " +
		"commit logs already exist. It cannot reconstruct what you thought, because nothing recorded " +
		"that at the time, and no further import will fix it. Ask closes that gap the only way it can " +
		"be closed: a commitment held for years stopped and nobody wrote down why, so it asks.\n\n" +
		"Questions come from arithmetic over the record, never from a model, so nothing is ever asked " +
		"about something that did not happen. Answers are ordinary entries and a question already " +
		"answered is not asked again.",
	RunE: runAsk,
}

func init() {
	askCmd.Flags().StringVarP(&askAnswer, "answer", "a", "", "Answer the next question and file it as an entry.")
	askCmd.Flags().IntVarP(&askCount, "count", "n", 1, "Number of questions to show.")
	askCmd.Flags().StringSliceVarP(&askTags, "tag", "t", []string{"answer"}, "Tags to attach to the answer.")
	askCmd.Flags().BoolVarP(&askInteractive, "interactive", "i", false, "Ask, then read the answer from stdin.")
	askCmd.Flags().BoolVar(&askList, "list", false, "List pending questions without answering.")
	askCmd.Flags().BoolVar(&askSkip, "skip", false,
		"Dismiss the next question without answering it, so it is never asked again.")
	rootCmd.AddCommand(askCmd)
}

// runAsk generates outstanding questions and either shows them or captures an
// answer to the first.
func runAsk(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := entriesInRange(v, dateRange{})
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	questions := pendingQuestions(entries)
	if len(questions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to ask: the record has no unexplained gaps yet.")
		return nil
	}

	if askSkip {
		q := questions[0]
		// A dismissal is recorded the same way an answer is, because a queue that
		// keeps returning a question the person has already rejected trains them
		// to stop reading it at all.
		entry := vault.Entry{
			Time: time.Now(),
			Tags: entryTags([]string{"skipped"}),
			Body: fmt.Sprintf("Not worth recording.\n%s%s\n%s%s",
				askedPrefixLabel, q.Prompt, askedPrefix, q.ID),
		}
		if err := v.Append(entry); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("record dismissal: %w", err))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Dismissed: %s\n", q.Prompt)
		fmt.Fprintf(cmd.ErrOrStderr(), "%d question(s) still outstanding.\n", len(questions)-1)
		return nil
	}

	if askList || (askAnswer == "" && !askInteractive) {
		writeQuestions(cmd.OutOrStdout(), questions, askCount)
		return nil
	}

	q := questions[0]
	answer := askAnswer
	if answer == "" {
		writeQuestion(cmd.ErrOrStderr(), q)
		answer, err = readAnswer(cmd.InOrStdin())
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(answer) == "" {
		return errors.Join(ErrNotFound, errors.New("no answer given"))
	}
	entry := vault.Entry{
		Time: time.Now(),
		Tags: entryTags(askTags),
		// The answer leads and the question trails as attribution. An entry's
		// headline is what every other command shows and what weave groups on, so
		// putting the prompt first would make the record display midden's
		// questions back instead of the person's own words.
		Body: fmt.Sprintf("%s\n\n%s%s\n%s%s",
			strings.TrimSpace(answer), askedPrefixLabel, q.Prompt, askedPrefix, q.ID),
	}
	if err := v.Append(entry); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append answer: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Answered: %s\n", q.Prompt)
	fmt.Fprintf(cmd.ErrOrStderr(), "%d question(s) still outstanding.\n", len(questions)-1)
	return nil
}

// pendingQuestions derives the outstanding questions from the record.
func pendingQuestions(entries []vault.Entry) []interview.Question {
	now := time.Now()
	answered := map[string]bool{}
	for _, e := range entries {
		if id := bodyMarker(e.Body, askedPrefix); id != "" {
			answered[id] = true
		}
	}
	// Answers are prose rather than events, so they must not themselves become
	// threads the interviewer then asks about.
	source := make([]vault.Entry, 0, len(entries))
	for _, e := range entries {
		if bodyMarker(e.Body, askedPrefix) == "" {
			source = append(source, e)
		}
	}
	threads := weave.Threads(source, weave.DefaultOptions(now))
	crossings := weave.Overlaps(source, []string{"calendar", "git"}, now)
	if len(crossings) > 3 {
		crossings = crossings[:3]
	}
	gaps := weave.Gaps(source, weave.DefaultGapOptions(now))
	return interview.Generate(threads, crossings, gaps, answered, interview.DefaultOptions(now))
}

// writeQuestions renders up to n outstanding questions.
func writeQuestions(w io.Writer, questions []interview.Question, n int) {
	if n < 1 {
		n = 1
	}
	if n > len(questions) {
		n = len(questions)
	}
	for _, q := range questions[:n] {
		writeQuestion(w, q)
	}
	if remaining := len(questions) - n; remaining > 0 {
		fmt.Fprintf(w, "%d more outstanding. Answer with: midden ask -i\n", remaining)
	}
}

// writeQuestion renders one question with the evidence behind it.
func writeQuestion(w io.Writer, q interview.Question) {
	fmt.Fprintf(w, "\n%s\n", q.Prompt)
	for line := range strings.SplitSeq(strings.TrimSpace(q.Context), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintln(w)
}

// readAnswer reads a free-text answer, ending at a blank line or end of input,
// so a spoken-length reply can run to several lines without ceremony.
func readAnswer(r io.Reader) (string, error) {
	fmt.Print("> ")
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" && len(lines) > 0 {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read answer: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}
