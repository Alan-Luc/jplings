// Package audio records push-to-talk microphone audio and plays synthesized speech.
//
// Both are thin wrappers over the host tools the doctor checks for, so the rest of
// the app never shells out directly.
package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Recorder captures microphone audio with pw-record (PipeWire).
type Recorder struct {
	// Binary defaults to pw-record.
	Binary string
	// SampleRate defaults to 16000, which is what Whisper wants.
	SampleRate int
}

// Record captures audio until ctx is cancelled, which is how push-to-talk ends a turn.
func (r Recorder) Record(ctx context.Context) ([]byte, error) {
	binary := r.Binary
	if binary == "" {
		binary = "pw-record"
	}
	rate := r.SampleRate
	if rate == 0 {
		rate = 16000
	}

	path := filepath.Join(os.TempDir(), "jplings-recording.wav")
	_ = os.Remove(path)
	defer os.Remove(path)

	cmd := exec.CommandContext(ctx, binary,
		"--rate", fmt.Sprint(rate),
		"--channels", "1",
		"--format", "s16",
		path,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}

	// Cancelling ctx (key release) stops the capture; wait returns after it exits.
	<-ctx.Done()
	_ = cmd.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recording: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("recording is empty")
	}
	return data, nil
}

// Player plays WAV audio with mpv.
type Player struct {
	// Binary defaults to mpv.
	Binary string
}

// Play writes the audio to a temporary file and plays it. mpv is used rather than a
// library so playback never blocks the TUI's event loop.
func (p Player) Play(wav []byte) error {
	binary := p.Binary
	if binary == "" {
		binary = "mpv"
	}

	file, err := os.CreateTemp("", "jplings-*.wav")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.Write(wav); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	cmd := exec.Command(binary, "--no-video", "--really-quiet", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mpv: %w: %s", err, output)
	}
	return nil
}
