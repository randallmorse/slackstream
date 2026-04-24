package tui

import (
	"testing"
	"time"

	"github.com/randallmorse/slackstream/slack"
)

func TestModel_AddMessage(t *testing.T) {
	m := NewModel(ModelConfig{
		MaxMessages: 100,
	})

	msg := slack.Message{
		ID:        "123.456",
		Channel:   "#test",
		Username:  "alice",
		Text:      "hello",
		Timestamp: time.Now(),
		IsDM:      false,
	}

	m.AddMessage(msg)

	if len(m.channelMessages) != 1 {
		t.Errorf("channelMessages count = %d, want 1", len(m.channelMessages))
	}
}

func TestModel_AddDM(t *testing.T) {
	m := NewModel(ModelConfig{
		MaxMessages: 100,
	})

	msg := slack.Message{
		ID:        "123.456",
		Channel:   "DM @bob",
		Username:  "bob",
		Text:      "hey",
		Timestamp: time.Now(),
		IsDM:      true,
	}

	m.AddMessage(msg)

	if len(m.dmMessages) != 1 {
		t.Errorf("dmMessages count = %d, want 1", len(m.dmMessages))
	}
}

func TestModel_SwitchView(t *testing.T) {
	m := NewModel(ModelConfig{})

	if m.view != ViewChannels {
		t.Error("initial view should be channels")
	}

	m.SwitchView()

	if m.view != ViewDMs {
		t.Error("after switch, view should be DMs")
	}

	m.SwitchView()

	if m.view != ViewChannels {
		t.Error("after second switch, view should be channels")
	}
}

func TestModel_Badges(t *testing.T) {
	m := NewModel(ModelConfig{})

	m.AddMessage(slack.Message{IsDM: true, ID: "1"})
	m.AddMessage(slack.Message{IsDM: true, ID: "2"})

	if m.dmBadge != 2 {
		t.Errorf("dmBadge = %d, want 2", m.dmBadge)
	}

	m.SwitchView() // switch to DMs

	if m.dmBadge != 0 {
		t.Errorf("dmBadge after switch = %d, want 0", m.dmBadge)
	}
}
