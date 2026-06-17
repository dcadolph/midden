package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// audioDuration optionally caps the recording length.
var audioDuration time.Duration

// audioTranscribe enables Whisper transcription after recording.
var audioTranscribe bool

// audioTags attach to the entry written for the audio file.
var audioTags []string

// audioCmd captures a voice memo and writes a linking entry to today's day file.
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Record a voice memo into the vault and optionally transcribe it.",
	RunE:  runAudio,
}

func init() {
	audioCmd.Flags().DurationVarP(&audioDuration, "duration", "d", 0, "Cap the recording length (0 = record until Ctrl-C).")
	audioCmd.Flags().BoolVar(&audioTranscribe, "transcribe", false, "Run OpenAI Whisper transcription after recording (needs $OPENAI_API_KEY).")
	audioCmd.Flags().StringSliceVarP(&audioTags, "tag", "t", []string{"audio"}, "Tags to attach to the entry.")
	rootCmd.AddCommand(audioCmd)
}

// runAudio detects an available recording binary, records to the vault audio
// subdirectory, optionally transcribes, and appends an entry linking to both.
func runAudio(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	when := time.Now()
	dir := filepath.Join(v.Dir, "audio", when.Format("2006"), when.Format("01"), when.Format("02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("create audio directory: %w", err))
	}
	path := filepath.Join(dir, when.Format("15-04-05")+".wav")
	if err := recordAudio(cmd, path); err != nil {
		return err
	}
	body := fmt.Sprintf("Voice memo at `%s`.", path)
	if audioTranscribe {
		text, err := transcribeWhisper(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "transcription skipped: %v\n", err)
		} else {
			body = strings.TrimSpace(text) + "\n\nAudio: `" + path + "`"
		}
	}
	if err := v.Append(vault.Entry{Time: when, Tags: normalizeTags(audioTags), Body: body}); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append audio entry: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Saved audio to %s\n", path)
	return nil
}

// recordAudio shells out to the first available recording tool and writes a WAV to path.
func recordAudio(cmd *cobra.Command, path string) error {
	binary, args := pickRecorder(path)
	if binary == "" {
		return errors.Join(ErrVault, errors.New("no recorder found: install sox, rec, or ffmpeg"))
	}
	if audioDuration > 0 {
		args = appendDurationArg(binary, args, audioDuration)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Recording with %s. Press Ctrl-C to stop.\n", binary)
	c := exec.Command(binary, args...)
	c.Stdin = os.Stdin
	c.Stdout = cmd.ErrOrStderr()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil
		}
		return errors.Join(ErrVault, fmt.Errorf("run %s: %w", binary, err))
	}
	return nil
}

// pickRecorder returns the first available recorder binary along with its required args.
// Returned empty binary means none was found.
func pickRecorder(out string) (string, []string) {
	if p, err := exec.LookPath("sox"); err == nil {
		return p, []string{"-d", out}
	}
	if p, err := exec.LookPath("rec"); err == nil {
		return p, []string{out}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		input := ffmpegInputArgs()
		return p, append(append([]string{}, input...), "-y", out)
	}
	return "", nil
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

// transcribeWhisper sends the WAV file to OpenAI Whisper and returns the transcribed text.
func transcribeWhisper(path string) (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return "", errors.New("OPENAI_API_KEY is not set")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open audio: %w", err)
	}
	defer f.Close()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write field: %w", err)
	}
	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("copy audio: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close form: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", &body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return parsed.Text, nil
}
