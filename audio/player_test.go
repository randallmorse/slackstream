package audio

import (
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	d := NewDebouncer(50 * time.Millisecond)

	if !d.ShouldPlay("channels") {
		t.Error("first call should allow play")
	}

	if d.ShouldPlay("channels") {
		t.Error("immediate second call should be debounced")
	}

	if !d.ShouldPlay("dms") {
		t.Error("different category should allow play")
	}

	time.Sleep(60 * time.Millisecond)

	if !d.ShouldPlay("channels") {
		t.Error("after debounce period, should allow play")
	}
}

func TestPlayer_Disabled(t *testing.T) {
	p := NewPlayer(PlayerConfig{Enabled: false})
	err := p.PlayChannel()
	if err != nil {
		t.Errorf("disabled player should not error: %v", err)
	}
}
