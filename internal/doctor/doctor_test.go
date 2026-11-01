package doctor

import (
	"errors"
	"testing"
	"time"
)

func host(overrides ...func(*Deps)) Deps {
	d := Deps{
		LookPath: func(string) (string, error) { return "/bin/tool", nil },
		Glob:     func(string) ([]string, error) { return []string{"/dev/dri/renderD128"}, nil },
		Dial:     func(string, string, time.Duration) error { return nil },
		Getenv:   func(string) string { return "key" },
	}
	for _, apply := range overrides {
		apply(&d)
	}
	return d
}

func TestRunPassesOnHealthyHost(t *testing.T) {
	if failed := Failures(Run(host())); len(failed) != 0 {
		t.Fatalf("expected no failures, got %v", failed)
	}
}

func TestRunBlocksOnMissingRequiredBinary(t *testing.T) {
	d := host(func(d *Deps) {
		d.LookPath = func(name string) (string, error) {
			if name == "mpv" {
				return "", errors.New("not found")
			}
			return "/bin/" + name, nil
		}
	})
	failed := Failures(Run(d))
	if len(failed) != 1 || failed[0].Name != "mpv" {
		t.Fatalf("expected mpv to be the single failure, got %v", failed)
	}
}

func TestRunBlocksOnMissingGPUDevice(t *testing.T) {
	d := host(func(d *Deps) {
		d.Glob = func(string) ([]string, error) { return nil, nil }
	})
	failed := Failures(Run(d))
	if len(failed) != 1 || failed[0].Name != "GPU render node" {
		t.Fatalf("expected the GPU render node to be the single failure, got %v", failed)
	}
}

func TestRunWarnsWithoutBlockingWhenSpeechIsDown(t *testing.T) {
	d := host(func(d *Deps) {
		d.Dial = func(string, string, time.Duration) error { return errors.New("connection refused") }
	})
	checks := Run(d)

	if failed := Failures(checks); len(failed) != 0 {
		t.Fatalf("speech services must not block startup, got failures %v", failed)
	}
	if status(checks, "STT service") != Warn {
		t.Fatalf("expected STT service to warn, got %s", status(checks, "STT service"))
	}
}

func TestRunWarnsWithoutBlockingWhenKeyIsUnset(t *testing.T) {
	d := host(func(d *Deps) {
		d.Getenv = func(string) string { return "" }
	})
	checks := Run(d)

	if failed := Failures(checks); len(failed) != 0 {
		t.Fatalf("a missing grading key must not block startup, got %v", failed)
	}
	if status(checks, "DEEPSEEK_API_KEY") != Warn {
		t.Fatalf("expected the key to warn, got %s", status(checks, "DEEPSEEK_API_KEY"))
	}
}

func status(checks []Check, name string) Status {
	for _, c := range checks {
		if c.Name == name {
			return c.Status
		}
	}
	return Missing
}
