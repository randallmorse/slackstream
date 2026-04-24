package slack

import (
	"testing"
	"time"
)

func TestDeduplicator(t *testing.T) {
	d := NewDeduplicator()

	msg1 := Message{ID: "1234.5678", ChannelID: "C123"}
	msg2 := Message{ID: "1234.5679", ChannelID: "C123"}

	if d.Seen(msg1) {
		t.Error("msg1 should not be seen initially")
	}

	d.Mark(msg1)

	if !d.Seen(msg1) {
		t.Error("msg1 should be seen after marking")
	}

	if d.Seen(msg2) {
		t.Error("msg2 should not be seen")
	}
}

func TestDeduplicator_Expiry(t *testing.T) {
	d := NewDeduplicatorWithTTL(10 * time.Millisecond)

	msg := Message{ID: "1234.5678", ChannelID: "C123"}
	d.Mark(msg)

	if !d.Seen(msg) {
		t.Error("msg should be seen immediately after marking")
	}

	time.Sleep(20 * time.Millisecond)
	d.Cleanup()

	if d.Seen(msg) {
		t.Error("msg should be expired after TTL")
	}
}
