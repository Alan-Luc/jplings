package audio

import (
	"context"
	"strings"
	"testing"
)

func TestPlayerReportsMissingBinary(t *testing.T) {
	player := Player{Binary: "definitely-not-a-real-player"}
	err := player.Play([]byte("RIFF....WAVE"))
	if err == nil {
		t.Fatal("expected an error when the player is missing")
	}
	if !strings.Contains(err.Error(), "mpv") {
		t.Fatalf("error should name the player, got %v", err)
	}
}

func TestRecordFailsCleanlyWhenBinaryMissing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // an already-cancelled turn must not hang

	recorder := Recorder{Binary: "definitely-not-a-real-recorder"}
	if _, err := recorder.Record(ctx); err == nil {
		t.Fatal("expected an error when the recorder is missing")
	}
}
