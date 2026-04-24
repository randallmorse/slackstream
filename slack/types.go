package slack

import "time"

type Message struct {
	ID        string
	ChannelID string
	Channel   string    // display name like "#engineering" or "DM @alice"
	UserID    string
	Username  string
	Text      string
	Timestamp time.Time
	ThreadTS  string    // parent thread timestamp, empty if top-level
	IsThread  bool      // true if this is a thread reply
	IsDM      bool
	IsBot     bool
}

type Channel struct {
	ID   string
	Name string
}

type User struct {
	ID       string
	Name     string
	RealName string
}
