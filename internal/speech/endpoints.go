// Package speech holds the local speech-service contract.
package speech

// Loopback endpoints fixed by infra/docker-compose.yml (S0.1). Both services bind
// to 127.0.0.1 only, so these are the only addresses that can ever answer.
const (
	STTAddr = "127.0.0.1:8770"
	TTSAddr = "127.0.0.1:8771"
)
