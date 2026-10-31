// Package doctor checks the host prerequisites the app needs before a session can
// start, so a missing runtime is a clean startup error rather than a mid-lesson
// failure (S1.1).
package doctor

import (
	"os"
	"time"

	"jplings/internal/speech"
)

// Status is the outcome of one prerequisite check.
type Status string

const (
	OK      Status = "ok"
	Missing Status = "missing"
	Warn    Status = "warn"
)

// Check is one prerequisite result. Required failures block startup; Warn records
// a degraded capability the app can run without.
type Check struct {
	Name     string
	Detail   string
	Status   Status
	Required bool
}

// Deps are the host probes, injected so the checks are testable without the host.
type Deps struct {
	LookPath func(string) (string, error)
	Glob     func(string) ([]string, error)
	Dial     func(network, addr string, timeout time.Duration) error
	Getenv   func(string) string
}

// Default wires Deps to the real host.
func Default() Deps {
	return Deps{
		LookPath: execLookPath,
		Glob:     filepathGlob,
		Dial:     dial,
		Getenv:   os.Getenv,
	}
}

// Run probes every prerequisite.
func Run(d Deps) []Check {
	return []Check{
		requireBinary(d, "mpv", "audio playback"),
		requireBinary(d, "pw-record", "microphone capture (PipeWire)"),
		requireGPU(d),
		optionalService(d, "STT service", speech.STTAddr),
		optionalService(d, "TTS service", speech.TTSAddr),
		optionalEnv(d, "DEEPSEEK_API_KEY", "free-form grading is unavailable; responses save ungraded"),
	}
}

// Failures returns the required checks that did not pass.
func Failures(checks []Check) []Check {
	var failed []Check
	for _, c := range checks {
		if c.Required && c.Status != OK {
			failed = append(failed, c)
		}
	}
	return failed
}

func requireBinary(d Deps, name, purpose string) Check {
	if path, err := d.LookPath(name); err == nil {
		return Check{Name: name, Detail: path, Status: OK, Required: true}
	}
	return Check{
		Name:     name,
		Detail:   "not found on PATH; needed for " + purpose,
		Status:   Missing,
		Required: true,
	}
}

func requireGPU(d Deps) Check {
	const name = "GPU render node"
	nodes, err := d.Glob("/dev/dri/renderD*")
	if err != nil {
		return Check{Name: name, Detail: err.Error(), Status: Missing, Required: true}
	}
	if len(nodes) == 0 {
		return Check{Name: name, Detail: "no /dev/dri/renderD* device; Whisper needs a Vulkan device", Status: Missing, Required: true}
	}
	return Check{Name: name, Detail: nodes[0], Status: OK, Required: true}
}

func optionalService(d Deps, name, addr string) Check {
	const timeout = 300 * time.Millisecond
	if err := d.Dial("tcp", addr, timeout); err != nil {
		return Check{
			Name:   name,
			Detail: addr + " unreachable; start it with infra/docker-compose.yml",
			Status: Warn,
		}
	}
	return Check{Name: name, Detail: addr, Status: OK}
}

func optionalEnv(d Deps, name, purpose string) Check {
	if d.Getenv(name) == "" {
		return Check{Name: name, Detail: "unset; " + purpose, Status: Warn}
	}
	return Check{Name: name, Detail: "set", Status: OK}
}
