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
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/util"
)

// transcribeFunc turns a recorded audio file into transcript text.
type transcribeFunc func(path string) (string, error)

// defaultWhisperModel is the local model used when the config does not name one.
const defaultWhisperModel = "base.en"

// whisperModelURL is the download location pattern for ggml Whisper models.
const whisperModelURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin"

// resolveTranscriber picks the transcription backend and prepares it to run.
// cloud forces the OpenAI backend; otherwise the config whisper_backend applies,
// defaulting to local whisper.cpp. Preparing the local backend resolves the
// binary and the model, downloading the model on first use with progress
// written to progress.
func resolveTranscriber(cloud bool, progress io.Writer) (transcribeFunc, error) {
	if cloud {
		return transcribeOpenAI, nil
	}
	cfg, _ := userConfig()
	switch strings.ToLower(strings.TrimSpace(cfg.WhisperBackend)) {
	case "", "local":
		return localTranscriber(progress)
	case "openai", "cloud":
		return transcribeOpenAI, nil
	}
	return nil, fmt.Errorf("unknown whisper_backend %q: use local or openai", cfg.WhisperBackend)
}

// localTranscriber returns a transcriber backed by the whisper.cpp CLI.
func localTranscriber(progress io.Writer) (transcribeFunc, error) {
	binary, err := whisperBinary()
	if err != nil {
		return nil, err
	}
	model, err := resolveWhisperModel(progress)
	if err != nil {
		return nil, err
	}
	return func(path string) (string, error) {
		return runWhisperCLI(binary, model, path)
	}, nil
}

// whisperBinary returns the path to the whisper.cpp CLI.
func whisperBinary() (string, error) {
	for _, name := range []string{"whisper-cli", "whisper-cpp"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errors.New("whisper.cpp not found: brew install whisper-cpp")
}

// runWhisperCLI transcribes the audio file with the whisper.cpp CLI and returns the raw text.
// The transcript is read from a text output file because the CLI mixes backend
// log noise into its standard streams.
func runWhisperCLI(binary, model, path string) (string, error) {
	outBase := path + ".transcript"
	outFile := outBase + ".txt"
	defer func() { _ = os.Remove(outFile) }()
	args := []string{"-m", model, "-f", path, "--no-prints", "--no-timestamps", "--output-txt", "--output-file", outBase}
	c := exec.Command(binary, args...) //nolint:gosec // Binary resolved via exec.LookPath, paths built locally.
	var stderr bytes.Buffer
	c.Stdout = io.Discard
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("run %s: %w: %s", filepath.Base(binary), err, util.TruncateRunes(stderr.String(), 500))
	}
	data, err := os.ReadFile(outFile) //nolint:gosec // Output path built from the recording path.
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	return string(data), nil
}

// resolveWhisperModel returns the on-disk path of the configured local model,
// downloading it into the midden data directory when missing.
func resolveWhisperModel(progress io.Writer) (string, error) {
	cfg, _ := userConfig()
	name := strings.TrimSpace(cfg.WhisperModel)
	if isModelPath(name) {
		path, err := util.Absolute(name)
		if err != nil {
			return "", fmt.Errorf("resolve whisper_model: %w", err)
		}
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("whisper_model file not found: %s", path)
		}
		return path, nil
	}
	if name == "" {
		name = defaultWhisperModel
	}
	dir, err := whisperModelDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, "ggml-"+name+".bin")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	if err := downloadWhisperModel(progress, name, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// isModelPath reports whether the whisper_model config value names a file
// rather than a downloadable model name.
func isModelPath(name string) bool {
	return strings.ContainsRune(name, os.PathSeparator) || strings.HasPrefix(name, "~") || strings.HasSuffix(name, ".bin")
}

// whisperModelDir returns the directory that stores downloaded Whisper models.
func whisperModelDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "midden", "models"), nil
}

// downloadWhisperModel fetches the named ggml model into dest, writing
// download progress to progress.
func downloadWhisperModel(progress io.Writer, name, dest string) error {
	url := fmt.Sprintf(whisperModelURL, name)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download model %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download model %s: %s (unknown model name?)", name, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	fmt.Fprintf(progress, "downloading whisper model %s (%s) to %s\n", name, sizeLabel(resp.ContentLength), dest)
	tmp, err := os.CreateTemp(filepath.Dir(dest), "ggml-*.partial")
	if err != nil {
		return fmt.Errorf("create temp model file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	pw := &progressWriter{Out: progress, Total: resp.ContentLength}
	if _, err := io.Copy(io.MultiWriter(tmp, pw), resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("download model %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp model file: %w", err)
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("move model into place: %w", err)
	}
	fmt.Fprintf(progress, "downloaded whisper model %s\n", name)
	return nil
}

// progressWriter reports copy progress to Out at most once per second.
type progressWriter struct {
	// Out receives the progress lines.
	Out io.Writer
	// Total is the expected byte count, or -1 when unknown.
	Total int64
	// n counts bytes written so far.
	n int64
	// last is the time of the previous progress line.
	last time.Time
}

// Write counts bytes and periodically emits a progress line.
func (p *progressWriter) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	if time.Since(p.last) >= time.Second {
		p.last = time.Now()
		if p.Total > 0 {
			fmt.Fprintf(p.Out, "  %s / %s (%d%%)\n", sizeLabel(p.n), sizeLabel(p.Total), p.n*100/p.Total)
		} else {
			fmt.Fprintf(p.Out, "  %s\n", sizeLabel(p.n))
		}
	}
	return len(b), nil
}

// sizeLabel renders a byte count as a human-readable megabyte label.
func sizeLabel(n int64) string {
	if n < 0 {
		return "unknown size"
	}
	return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
}

// trimTranscript cleans Whisper output for use as an entry body.
// It drops noise-only lines such as [BLANK_AUDIO] or (silence) and trims
// surrounding whitespace while preserving interior line structure.
func trimTranscript(s string) string {
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isNoiseLine(strings.TrimSpace(line)) {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// isNoiseLine reports whether a trimmed transcript line is a Whisper noise
// marker rather than speech, such as "[BLANK_AUDIO]" or "(silence)".
func isNoiseLine(line string) bool {
	if len(line) < 2 {
		return false
	}
	first, last := line[0], line[len(line)-1]
	return (first == '[' && last == ']') || (first == '(' && last == ')')
}

// transcribeOpenAI sends the WAV file to the OpenAI Whisper API and returns the transcribed text.
func transcribeOpenAI(path string) (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return "", errors.New("OPENAI_API_KEY is not set")
	}
	f, err := os.Open(path) //nolint:gosec // Recording path built from the vault directory.
	if err != nil {
		return "", fmt.Errorf("open audio: %w", err)
	}
	defer func() { _ = f.Close() }()
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
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
