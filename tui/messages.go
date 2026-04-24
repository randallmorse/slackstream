package tui

import (
	"github.com/randallmorse/slackstream/slack"
)

type NewMessageMsg struct {
	Message slack.Message
}

type TickMsg struct{}

type StatusToggledMsg struct {
	On  bool
	Err error
}

type ConnectionStateMsg struct {
	Connected bool
	Error     error
}
