package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// voiceTags are the tags applied to the spoken entry in addition to the
// always-present "voice" tag merged in at append time.
var voiceTags []string

// voiceDuration optionally caps the recording length.
var voiceDuration time.Duration

// voiceKeepAudio retains the recorded WAV in the vault and links it from the entry.
var voiceKeepAudio bool

// voiceCloud forces the OpenAI Whisper backend for this run.
var voiceCloud bool

// voiceCmd records speech, transcribes it locally, and appends the transcript
// as a plain entry to today's day file.
var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Speak an entry and append the transcript to today's day file.",
	Long: "Speak an entry and append the transcript to today's day file.\n\n" +
		"Recording stops on Ctrl-C or when --duration elapses. Transcription runs\n" +
		"locally through whisper.cpp by default; the model downloads on first use.\n" +
		"The audio itself is discarded unless --keep-audio is set.",
	RunE: runVoice,
}

func init() {
	voiceCmd.Flags().StringSliceVarP(&voiceTags, "tag", "t", nil, "Tags to attach to the entry (the voice tag is always added).")
	voiceCmd.Flags().DurationVar(&voiceDuration, "duration", 0, "Cap the recording length (0 = record until Ctrl-C).")
	voiceCmd.Flags().BoolVar(&voiceKeepAudio, "keep-audio", false, "Keep the WAV in the vault and link it from the entry.")
	voiceCmd.Flags().BoolVar(&voiceCloud, "cloud", false, "Use the OpenAI Whisper API instead of local whisper.cpp.")
	rootCmd.AddCommand(voiceCmd)
}

// runVoice records a spoken entry, transcribes it, and appends the transcript.
// The transcriber is resolved before the microphone opens so a missing model
// downloads up front rather than after the user has spoken.
func runVoice(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	transcribe, err := resolveTranscriber(voiceCloud, cmd.ErrOrStderr())
	if err != nil {
		return errors.Join(ErrTranscribe, err)
	}
	when := time.Now()
	path, cleanup, err := voiceRecordingPath(v, when)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := recordAudio(cmd, path, recordOptions{Duration: voiceDuration, Mono16k: true}); err != nil {
		return err
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		return errors.Join(ErrTranscribe, fmt.Errorf("recording produced no audio at %s", path))
	}
	text, err := transcribe(path)
	if err != nil {
		return errors.Join(ErrTranscribe, err)
	}
	transcript := trimTranscript(text)
	if transcript == "" {
		return errors.Join(ErrTranscribe, errors.New("no speech detected"))
	}
	audioPath := ""
	if voiceKeepAudio {
		audioPath = path
	}
	entry := voiceEntry(when, entryTags(append(voiceTags, "voice")), transcript, audioPath)
	if err := v.Append(entry); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append voice entry: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), transcript)
	fmt.Fprintf(cmd.OutOrStdout(), "Appended entry at %s\n", entry.Time.Format(layoutDateTime))
	return nil
}

// voiceRecordingPath returns the WAV path for a spoken entry and a cleanup
// function. With --keep-audio the file lands in the vault audio tree and
// cleanup is a no-op; otherwise it lands in a temp directory that cleanup
// removes.
func voiceRecordingPath(v *vault.Vault, when time.Time) (string, func(), error) {
	if voiceKeepAudio {
		dir := filepath.Join(v.Dir, "audio", when.Format("2006"), when.Format("01"), when.Format("02"))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", nil, errors.Join(ErrVault, fmt.Errorf("create audio directory: %w", err))
		}
		return filepath.Join(dir, when.Format("15-04-05")+".wav"), func() {}, nil
	}
	dir, err := os.MkdirTemp("", "midden-voice-*")
	if err != nil {
		return "", nil, errors.Join(ErrVault, fmt.Errorf("create temp directory: %w", err))
	}
	return filepath.Join(dir, "entry.wav"), func() { _ = os.RemoveAll(dir) }, nil
}

// voiceEntry builds the journal entry for a spoken transcript.
// tags must already be merged and normalized. A non-empty audioPath is linked
// below the transcript, matching the audio command's entry shape.
func voiceEntry(when time.Time, tags []string, transcript, audioPath string) vault.Entry {
	body := transcript
	if audioPath != "" {
		body += "\n\nAudio: `" + audioPath + "`"
	}
	return vault.Entry{Time: when, Tags: tags, Body: body}
}
