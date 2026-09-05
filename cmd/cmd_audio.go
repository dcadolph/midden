package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// audioDuration optionally caps the recording length.
var audioDuration time.Duration

// audioTranscribe enables Whisper transcription after recording.
var audioTranscribe bool

// audioTags attach to the entry written for the audio file in addition to the
// always-present "audio" tag merged in at append time.
var audioTags []string

// audioCmd captures a voice memo and writes a linking entry to today's day file.
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Record a voice memo into the vault and optionally transcribe it.",
	RunE:  runAudio,
}

func init() {
	audioCmd.Flags().DurationVar(&audioDuration, "duration", 0, "Cap the recording length (0 = record until Ctrl-C).")
	audioCmd.Flags().BoolVar(&audioTranscribe, "transcribe", false, "Transcribe after recording (local whisper.cpp by default; whisper_backend config selects openai).")
	audioCmd.Flags().StringSliceVarP(&audioTags, "tag", "t", nil, "Tags to attach to the entry (the audio tag is always added).")
	rootCmd.AddCommand(audioCmd)
}

// runAudio detects an available recording binary, records to the vault audio
// subdirectory, optionally transcribes, and appends an entry linking to both.
func runAudio(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	var transcribe transcribeFunc
	if audioTranscribe {
		// Resolve before the microphone opens so a first-use model download
		// happens up front, and degrade to a plain memo on failure.
		t, err := resolveTranscriber(false, cmd.ErrOrStderr())
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "transcription skipped: %v\n", err)
		} else {
			transcribe = t
		}
	}
	when := time.Now()
	dir := filepath.Join(v.Dir, "audio", when.Format("2006"), when.Format("01"), when.Format("02"))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("create audio directory: %w", err))
	}
	path := filepath.Join(dir, when.Format("15-04-05")+".wav")
	if err := recordAudio(cmd, path, recordOptions{Duration: audioDuration}); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("recording produced no file: %w", err))
	}
	if info.Size() == 0 {
		return errors.Join(ErrVault, fmt.Errorf("recording produced empty file %s", path))
	}
	body := fmt.Sprintf("Voice memo at `%s`.", path)
	if transcribe != nil {
		text, err := transcribe(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "transcription skipped: %v\n", err)
		} else if cleaned := trimTranscript(text); cleaned != "" {
			body = cleaned + "\n\nAudio: `" + path + "`"
		}
	}
	if err := v.Append(vault.Entry{Time: when, Tags: entryTags(append(audioTags, "audio")), Body: body}); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append audio entry: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Saved audio to %s\n", path)
	return nil
}

// recordOptions control how recordAudio captures the WAV.
type recordOptions struct {
	// Duration caps the recording length; zero records until Ctrl-C.
	Duration time.Duration
	// Mono16k records 16 kHz mono, the native Whisper input format.
	Mono16k bool
}

// recordAudio shells out to the first available recording tool and writes a WAV to path.
func recordAudio(cmd *cobra.Command, path string, opts recordOptions) error {
	binary, args := pickRecorder(path, opts.Mono16k)
	if binary == "" {
		return errors.Join(ErrVault, errors.New("no recorder found: install sox, rec, or ffmpeg"))
	}
	if opts.Duration > 0 {
		args = appendDurationArg(binary, args, opts.Duration)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Recording with %s. Press Ctrl-C to stop.\n", binary)
	c := exec.Command(binary, args...) //nolint:gosec // Recorder binary resolved via exec.LookPath.
	c.Stdin = os.Stdin
	c.Stdout = cmd.ErrOrStderr()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// Ctrl-C exits the recorder non-zero; the caller validates the
			// output file instead of trusting the exit code.
			return nil
		}
		return errors.Join(ErrVault, fmt.Errorf("run %s: %w", binary, err))
	}
	return nil
}

// pickRecorder returns the first available recorder binary along with its required args.
// Returned empty binary means none was found.
func pickRecorder(out string, mono16k bool) (string, []string) {
	if p, err := exec.LookPath("sox"); err == nil {
		return p, recorderArgs("sox", out, mono16k)
	}
	if p, err := exec.LookPath("rec"); err == nil {
		return p, recorderArgs("rec", out, mono16k)
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, recorderArgs("ffmpeg", out, mono16k)
	}
	return "", nil
}

// recorderArgs builds the capture arguments for a recorder binary name.
// With mono16k, sox and rec downsample through effects after the output file
// and ffmpeg through output options before it.
func recorderArgs(name, out string, mono16k bool) []string {
	switch name {
	case "sox":
		args := []string{"-d", out}
		if mono16k {
			args = append(args, "rate", "16000", "channels", "1")
		}
		return args
	case "rec":
		args := []string{out}
		if mono16k {
			args = append(args, "rate", "16000", "channels", "1")
		}
		return args
	case "ffmpeg":
		args := append([]string{}, ffmpegInputArgs()...)
		if mono16k {
			args = append(args, "-ar", "16000", "-ac", "1")
		}
		return append(args, "-y", out)
	}
	return nil
}

// ffmpegInputArgs returns the per-OS ffmpeg input arguments for default microphone capture.
func ffmpegInputArgs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"-f", "avfoundation", "-i", ":0"}
	case "linux":
		return []string{"-f", "alsa", "-i", "default"}
	case "windows":
		return []string{"-f", "dshow", "-i", "audio=Microphone"}
	}
	return []string{"-i", "default"}
}

// appendDurationArg attaches the right duration flag for the chosen recorder.
func appendDurationArg(binary string, args []string, d time.Duration) []string {
	secs := int(d.Seconds())
	switch filepath.Base(binary) {
	case "sox", "rec":
		return append(args, "trim", "0", fmt.Sprintf("%d", secs))
	case "ffmpeg":
		return append([]string{"-t", fmt.Sprintf("%d", secs)}, args...)
	}
	return args
}
