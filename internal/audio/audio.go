package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Recorder struct {
	Binary     string
	SampleRate int
}

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

	cmd := exec.Command(binary,
		"--rate", fmt.Sprint(rate),
		"--channels", "1",
		"--format", "s16",
		path,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}

	// Interrupt, never kill: SIGINT lets pw-record finalise the WAV header, a kill truncates it.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Signal(os.Interrupt)
			}
		case <-done:
		}
	}()
	waitErr := cmd.Wait()
	close(done)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recording: %w", err)
	}
	if len(data) == 0 {
		if waitErr != nil {
			return nil, fmt.Errorf("recording failed: %w", waitErr)
		}
		return nil, fmt.Errorf("recording is empty")
	}
	return data, nil
}

type Player struct {
	Binary string
}

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
