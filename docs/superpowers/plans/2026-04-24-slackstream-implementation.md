# slackstream Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a terminal UI that displays real-time Slack channel and DM messages in a unified scrolling feed with sound notifications and status toggle.

**Architecture:** Polling-based Slack client with bubbletea TUI. Config loader reads YAML, pollers fetch messages per-channel via goroutines, message bus unifies the stream, TUI renders with two switchable views (Channels/DMs).

**Tech Stack:** Go 1.21+, bubbletea (TUI), lipgloss (styling), slack-go (API), beep (audio), yaml.v3 (config)

---

## File Structure

```
slackstream/
├── main.go                     # CLI entry, config load, app bootstrap
├── go.mod                      # module definition
├── config/
│   └── config.go               # Config struct, Load(), validation
├── config/config_test.go       # config loading tests
├── slack/
│   ├── types.go                # Message, Channel, User types
│   ├── client.go               # SlackClient wrapper, API calls
│   ├── client_test.go          # client tests with mock
│   ├── poller.go               # Poller goroutine, deduplication
│   ├── poller_test.go          # poller tests
│   └── status.go               # SetStatus, ClearStatus
├── tui/
│   ├── model.go                # bubbletea Model, Init, Update, View
│   ├── model_test.go           # TUI state tests
│   ├── messages.go             # bubbletea Msg types
│   ├── keys.go                 # key bindings
│   └── styles.go               # lipgloss styles
├── audio/
│   ├── player.go               # Player, debounced playback
│   ├── player_test.go          # audio tests
│   └── sounds/                 # embedded wav files
│       ├── tick.wav
│       ├── tah.wav
│       ├── pop.wav
│       └── ding.wav
└── testdata/
    └── config.yaml             # test config file
```

---

## Task 1: Project Initialization

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/rmorse/joy/slackstream
go mod init github.com/randallmorse/slackstream
```

Expected: `go.mod` created with module path

- [ ] **Step 2: Create minimal main.go**

Create `main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("slackstream")
}
```

- [ ] **Step 3: Verify it builds and runs**

```bash
go build -o slackstream . && ./slackstream
```

Expected output: `slackstream`

- [ ] **Step 4: Commit**

```bash
git add go.mod main.go
git commit -m "feat: initialize go module and main entry point"
```

---

## Task 2: Config Types and Loading

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`
- Create: `testdata/config.yaml`

- [ ] **Step 1: Create test config file**

Create `testdata/config.yaml`:
```yaml
token: "xoxp-test-token"

channels:
  - "#engineering"
  - "#incidents"

dms: true

settings:
  poll_interval: 3s
  max_message_length: 120
  show_threads: true
  show_bots: true

sounds:
  enabled: true
  channels:
    enabled: true
    sound: "tick"
  dms:
    enabled: true
    sound: "tah"
  debounce: 3s

status:
  enabled: true
  text: "Work Session In Progress - DM to Reach Me"
  emoji: ":headphones:"
```

- [ ] **Step 2: Write failing test for config loading**

Create `config/config_test.go`:
```go
package config

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := Load("../testdata/config.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Token != "xoxp-test-token" {
		t.Errorf("Token = %q, want %q", cfg.Token, "xoxp-test-token")
	}

	if len(cfg.Channels) != 2 {
		t.Errorf("Channels count = %d, want 2", len(cfg.Channels))
	}

	if cfg.Settings.PollInterval != 3*time.Second {
		t.Errorf("PollInterval = %v, want 3s", cfg.Settings.PollInterval)
	}

	if !cfg.DMs {
		t.Error("DMs should be true")
	}

	if !cfg.Sounds.Enabled {
		t.Error("Sounds.Enabled should be true")
	}

	if cfg.Status.Text != "Work Session In Progress - DM to Reach Me" {
		t.Errorf("Status.Text = %q, want work session text", cfg.Status.Text)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-env-token")

	cfg, err := Load("../testdata/config.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Token != "xoxp-env-token" {
		t.Errorf("Token = %q, want env override %q", cfg.Token, "xoxp-env-token")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/rmorse/joy/slackstream
go test ./config/...
```

Expected: FAIL (package/function not found)

- [ ] **Step 4: Implement config types and Load function**

Create `config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token    string   `yaml:"token"`
	Channels []string `yaml:"channels"`
	DMs      bool     `yaml:"dms"`
	Settings Settings `yaml:"settings"`
	Sounds   Sounds   `yaml:"sounds"`
	Status   Status   `yaml:"status"`
}

type Settings struct {
	PollInterval     time.Duration `yaml:"poll_interval"`
	MaxMessageLength int           `yaml:"max_message_length"`
	ShowThreads      bool          `yaml:"show_threads"`
	ShowBots         bool          `yaml:"show_bots"`
}

type Sounds struct {
	Enabled  bool        `yaml:"enabled"`
	Channels SoundConfig `yaml:"channels"`
	DMs      SoundConfig `yaml:"dms"`
	Debounce time.Duration `yaml:"debounce"`
}

type SoundConfig struct {
	Enabled bool   `yaml:"enabled"`
	Sound   string `yaml:"sound"`
}

type Status struct {
	Enabled bool   `yaml:"enabled"`
	Text    string `yaml:"text"`
	Emoji   string `yaml:"emoji"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if envToken := os.Getenv("SLACK_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required (set in config or SLACK_TOKEN env var)")
	}
	if len(c.Channels) == 0 && !c.DMs {
		return fmt.Errorf("at least one channel or dms: true required")
	}
	if c.Settings.PollInterval == 0 {
		c.Settings.PollInterval = 3 * time.Second
	}
	if c.Settings.MaxMessageLength == 0 {
		c.Settings.MaxMessageLength = 120
	}
	if c.Sounds.Debounce == 0 {
		c.Sounds.Debounce = 3 * time.Second
	}
	return nil
}
```

- [ ] **Step 5: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./config/... -v
```

Expected: PASS (all 3 tests)

- [ ] **Step 7: Commit**

```bash
git add config/ testdata/ go.mod go.sum
git commit -m "feat: add config loading with YAML parsing and env override"
```

---

## Task 3: Slack Types

**Files:**
- Create: `slack/types.go`

- [ ] **Step 1: Create message and channel types**

Create `slack/types.go`:
```go
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
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./slack/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add slack/types.go
git commit -m "feat: add slack message and channel types"
```

---

## Task 4: Slack Client

**Files:**
- Create: `slack/client.go`
- Create: `slack/client_test.go`

- [ ] **Step 1: Write failing test for client**

Create `slack/client_test.go`:
```go
package slack

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("xoxp-test-token")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.token != "xoxp-test-token" {
		t.Errorf("token = %q, want %q", client.token, "xoxp-test-token")
	}
}

func TestClient_ResolveChannelID(t *testing.T) {
	client := NewClient("xoxp-test")

	tests := []struct {
		input string
		want  string
	}{
		{"C01234567", "C01234567"},
		{"#engineering", "#engineering"}, // name stays as-is, resolved later via API
	}

	for _, tt := range tests {
		got := client.NormalizeChannelRef(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeChannelRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./slack/... -v
```

Expected: FAIL (NewClient not found)

- [ ] **Step 3: Implement client**

Create `slack/client.go`:
```go
package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

type Client struct {
	api   *slack.Client
	token string

	userCache   map[string]*User
	userCacheMu sync.RWMutex

	channelCache   map[string]*Channel
	channelCacheMu sync.RWMutex
}

func NewClient(token string) *Client {
	return &Client{
		api:          slack.New(token),
		token:        token,
		userCache:    make(map[string]*User),
		channelCache: make(map[string]*Channel),
	}
}

func (c *Client) NormalizeChannelRef(ref string) string {
	return ref
}

func (c *Client) ResolveChannelID(ctx context.Context, ref string) (string, error) {
	if strings.HasPrefix(ref, "C") || strings.HasPrefix(ref, "G") {
		return ref, nil
	}

	name := strings.TrimPrefix(ref, "#")

	c.channelCacheMu.RLock()
	for id, ch := range c.channelCache {
		if ch.Name == name {
			c.channelCacheMu.RUnlock()
			return id, nil
		}
	}
	c.channelCacheMu.RUnlock()

	channels, _, err := c.api.GetConversationsContext(ctx, &slack.GetConversationsParameters{
		Types: []string{"public_channel", "private_channel"},
		Limit: 1000,
	})
	if err != nil {
		return "", fmt.Errorf("list channels: %w", err)
	}

	c.channelCacheMu.Lock()
	for _, ch := range channels {
		c.channelCache[ch.ID] = &Channel{ID: ch.ID, Name: ch.Name}
		if ch.Name == name {
			c.channelCacheMu.Unlock()
			return ch.ID, nil
		}
	}
	c.channelCacheMu.Unlock()

	return "", fmt.Errorf("channel %q not found", ref)
}

func (c *Client) GetChannelHistory(ctx context.Context, channelID string, oldest time.Time) ([]Message, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    fmt.Sprintf("%d.%06d", oldest.Unix(), oldest.Nanosecond()/1000),
		Limit:     100,
	}

	resp, err := c.api.GetConversationHistoryContext(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	var messages []Message
	for _, msg := range resp.Messages {
		m, err := c.convertMessage(ctx, channelID, msg, false)
		if err != nil {
			continue
		}
		messages = append(messages, m)
	}

	return messages, nil
}

func (c *Client) GetThreadReplies(ctx context.Context, channelID, threadTS string, oldest time.Time) ([]Message, error) {
	msgs, _, _, err := c.api.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
		Oldest:    fmt.Sprintf("%d.%06d", oldest.Unix(), oldest.Nanosecond()/1000),
	})
	if err != nil {
		return nil, fmt.Errorf("get replies: %w", err)
	}

	var messages []Message
	for _, msg := range msgs {
		if msg.Timestamp == threadTS {
			continue
		}
		m, err := c.convertMessage(ctx, channelID, msg, true)
		if err != nil {
			continue
		}
		messages = append(messages, m)
	}

	return messages, nil
}

func (c *Client) GetDMConversations(ctx context.Context) ([]Channel, error) {
	convs, _, err := c.api.GetConversationsContext(ctx, &slack.GetConversationsParameters{
		Types: []string{"im"},
		Limit: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("list DMs: %w", err)
	}

	var dms []Channel
	for _, conv := range convs {
		user, err := c.GetUser(ctx, conv.User)
		if err != nil {
			continue
		}
		dms = append(dms, Channel{
			ID:   conv.ID,
			Name: fmt.Sprintf("DM @%s", user.Name),
		})
	}

	return dms, nil
}

func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	c.userCacheMu.RLock()
	if u, ok := c.userCache[userID]; ok {
		c.userCacheMu.RUnlock()
		return u, nil
	}
	c.userCacheMu.RUnlock()

	user, err := c.api.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	u := &User{
		ID:       user.ID,
		Name:     user.Name,
		RealName: user.RealName,
	}

	c.userCacheMu.Lock()
	c.userCache[userID] = u
	c.userCacheMu.Unlock()

	return u, nil
}

func (c *Client) GetChannelName(ctx context.Context, channelID string) (string, error) {
	c.channelCacheMu.RLock()
	if ch, ok := c.channelCache[channelID]; ok {
		c.channelCacheMu.RUnlock()
		return ch.Name, nil
	}
	c.channelCacheMu.RUnlock()

	info, err := c.api.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: channelID,
	})
	if err != nil {
		return "", fmt.Errorf("get channel info: %w", err)
	}

	c.channelCacheMu.Lock()
	c.channelCache[channelID] = &Channel{ID: info.ID, Name: info.Name}
	c.channelCacheMu.Unlock()

	return info.Name, nil
}

func (c *Client) convertMessage(ctx context.Context, channelID string, msg slack.Message, isThread bool) (Message, error) {
	ts, err := parseSlackTimestamp(msg.Timestamp)
	if err != nil {
		return Message{}, err
	}

	username := "unknown"
	isBot := msg.BotID != ""
	if msg.User != "" {
		if user, err := c.GetUser(ctx, msg.User); err == nil {
			username = user.Name
		}
	} else if msg.Username != "" {
		username = msg.Username
	}

	channelName, _ := c.GetChannelName(ctx, channelID)

	return Message{
		ID:        msg.Timestamp,
		ChannelID: channelID,
		Channel:   "#" + channelName,
		UserID:    msg.User,
		Username:  username,
		Text:      msg.Text,
		Timestamp: ts,
		ThreadTS:  msg.ThreadTimestamp,
		IsThread:  isThread,
		IsBot:     isBot,
	}, nil
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	var sec, usec int64
	_, err := fmt.Sscanf(ts, "%d.%d", &sec, &usec)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, usec*1000), nil
}
```

- [ ] **Step 4: Add slack-go dependency**

```bash
go get github.com/slack-go/slack
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./slack/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add slack/client.go slack/client_test.go go.mod go.sum
git commit -m "feat: add slack client with channel history and user caching"
```

---

## Task 5: Slack Status Toggle

**Files:**
- Create: `slack/status.go`

- [ ] **Step 1: Implement status toggle**

Create `slack/status.go`:
```go
package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

func (c *Client) SetStatus(ctx context.Context, text, emoji string) error {
	err := c.api.SetUserCustomStatusContextWithUser(ctx, "", text, emoji, 0)
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

func (c *Client) ClearStatus(ctx context.Context) error {
	err := c.api.SetUserCustomStatusContextWithUser(ctx, "", "", "", 0)
	if err != nil {
		return fmt.Errorf("clear status: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./slack/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add slack/status.go
git commit -m "feat: add slack status set/clear functionality"
```

---

## Task 6: Poller

**Files:**
- Create: `slack/poller.go`
- Create: `slack/poller_test.go`

- [ ] **Step 1: Write failing test for poller**

Create `slack/poller_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./slack/... -v -run TestDedup
```

Expected: FAIL (NewDeduplicator not found)

- [ ] **Step 3: Implement poller with deduplication**

Create `slack/poller.go`:
```go
package slack

import (
	"context"
	"sync"
	"time"
)

type Deduplicator struct {
	seen map[string]time.Time
	mu   sync.RWMutex
	ttl  time.Duration
}

func NewDeduplicator() *Deduplicator {
	return NewDeduplicatorWithTTL(10 * time.Minute)
}

func NewDeduplicatorWithTTL(ttl time.Duration) *Deduplicator {
	return &Deduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
}

func (d *Deduplicator) key(m Message) string {
	return m.ChannelID + ":" + m.ID
}

func (d *Deduplicator) Seen(m Message) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.seen[d.key(m)]
	return ok
}

func (d *Deduplicator) Mark(m Message) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[d.key(m)] = time.Now()
}

func (d *Deduplicator) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	cutoff := time.Now().Add(-d.ttl)
	for k, t := range d.seen {
		if t.Before(cutoff) {
			delete(d.seen, k)
		}
	}
}

type Poller struct {
	client      *Client
	channelID   string
	channelName string
	isDM        bool
	interval    time.Duration
	showThreads bool
	dedup       *Deduplicator
	messages    chan<- Message
	lastPoll    time.Time
}

type PollerConfig struct {
	Client      *Client
	ChannelID   string
	ChannelName string
	IsDM        bool
	Interval    time.Duration
	ShowThreads bool
	Messages    chan<- Message
}

func NewPoller(cfg PollerConfig) *Poller {
	return &Poller{
		client:      cfg.Client,
		channelID:   cfg.ChannelID,
		channelName: cfg.ChannelName,
		isDM:        cfg.IsDM,
		interval:    cfg.Interval,
		showThreads: cfg.ShowThreads,
		dedup:       NewDeduplicator(),
		messages:    cfg.Messages,
		lastPoll:    time.Now(),
	}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
			p.dedup.Cleanup()
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	oldest := p.lastPoll
	p.lastPoll = time.Now()

	msgs, err := p.client.GetChannelHistory(ctx, p.channelID, oldest)
	if err != nil {
		return
	}

	threadTSs := make(map[string]bool)
	for _, msg := range msgs {
		if msg.ThreadTS != "" && msg.ThreadTS != msg.ID {
			threadTSs[msg.ThreadTS] = true
		}
	}

	if p.showThreads {
		for threadTS := range threadTSs {
			replies, err := p.client.GetThreadReplies(ctx, p.channelID, threadTS, oldest)
			if err != nil {
				continue
			}
			msgs = append(msgs, replies...)
		}
	}

	for _, msg := range msgs {
		if p.dedup.Seen(msg) {
			continue
		}
		p.dedup.Mark(msg)

		msg.IsDM = p.isDM
		if p.isDM {
			msg.Channel = p.channelName
		}

		select {
		case p.messages <- msg:
		case <-ctx.Done():
			return
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./slack/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add slack/poller.go slack/poller_test.go
git commit -m "feat: add poller with deduplication and thread support"
```

---

## Task 7: Audio Player with Debouncing

**Files:**
- Create: `audio/player.go`
- Create: `audio/player_test.go`
- Create: `audio/sounds/` directory with placeholder

- [ ] **Step 1: Write failing test for debounced player**

Create `audio/player_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./audio/... -v
```

Expected: FAIL (package not found)

- [ ] **Step 3: Implement audio player**

Create `audio/player.go`:
```go
package audio

import (
	"embed"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

//go:embed sounds/*.wav
var soundsFS embed.FS

type Debouncer struct {
	interval time.Duration
	lastPlay map[string]time.Time
	mu       sync.Mutex
}

func NewDebouncer(interval time.Duration) *Debouncer {
	return &Debouncer{
		interval: interval,
		lastPlay: make(map[string]time.Time),
	}
}

func (d *Debouncer) ShouldPlay(category string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	last, ok := d.lastPlay[category]
	if ok && time.Since(last) < d.interval {
		return false
	}
	d.lastPlay[category] = time.Now()
	return true
}

type PlayerConfig struct {
	Enabled         bool
	ChannelsEnabled bool
	ChannelsSound   string
	DMsEnabled      bool
	DMsSound        string
	Debounce        time.Duration
}

type Player struct {
	config    PlayerConfig
	debouncer *Debouncer
	initOnce  sync.Once
	initErr   error
}

func NewPlayer(cfg PlayerConfig) *Player {
	return &Player{
		config:    cfg,
		debouncer: NewDebouncer(cfg.Debounce),
	}
}

func (p *Player) init() error {
	p.initOnce.Do(func() {
		sr := beep.SampleRate(44100)
		p.initErr = speaker.Init(sr, sr.N(time.Second/10))
	})
	return p.initErr
}

func (p *Player) PlayChannel() error {
	if !p.config.Enabled || !p.config.ChannelsEnabled {
		return nil
	}
	if !p.debouncer.ShouldPlay("channels") {
		return nil
	}
	return p.playSound(p.config.ChannelsSound)
}

func (p *Player) PlayDM() error {
	if !p.config.Enabled || !p.config.DMsEnabled {
		return nil
	}
	if !p.debouncer.ShouldPlay("dms") {
		return nil
	}
	return p.playSound(p.config.DMsSound)
}

func (p *Player) playSound(sound string) error {
	if err := p.init(); err != nil {
		return err
	}

	var reader io.ReadCloser
	var err error

	switch sound {
	case "tick", "tah", "pop", "ding":
		f, err := soundsFS.Open(fmt.Sprintf("sounds/%s.wav", sound))
		if err != nil {
			return fmt.Errorf("open embedded sound: %w", err)
		}
		reader = f
	default:
		f, err := os.Open(sound)
		if err != nil {
			return fmt.Errorf("open sound file: %w", err)
		}
		reader = f
	}
	defer reader.Close()

	streamer, format, err := wav.Decode(reader)
	if err != nil {
		return fmt.Errorf("decode wav: %w", err)
	}
	defer streamer.Close()

	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(44100), streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done
	return nil
}
```

- [ ] **Step 4: Create placeholder sound files**

```bash
mkdir -p /Users/rmorse/joy/slackstream/audio/sounds
```

Create minimal WAV files (we'll use actual sounds later, for now create placeholders that won't crash):

```bash
cd /Users/rmorse/joy/slackstream/audio/sounds
# Create minimal valid WAV files (44 bytes each - silent)
for name in tick tah pop ding; do
  printf 'RIFF$\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x44\xac\x00\x00\x88\x58\x01\x00\x02\x00\x10\x00data\x00\x00\x00\x00' > ${name}.wav
done
```

- [ ] **Step 5: Add beep dependency**

```bash
cd /Users/rmorse/joy/slackstream
go get github.com/faiface/beep
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./audio/... -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add audio/ go.mod go.sum
git commit -m "feat: add audio player with debouncing and embedded sounds"
```

---

## Task 8: TUI Styles

**Files:**
- Create: `tui/styles.go`

- [ ] **Step 1: Create styles**

Create `tui/styles.go`:
```go
package tui

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	channelColors = []lipgloss.Color{
		lipgloss.Color("205"), // pink
		lipgloss.Color("39"),  // cyan
		lipgloss.Color("214"), // orange
		lipgloss.Color("82"),  // green
		lipgloss.Color("141"), // purple
		lipgloss.Color("220"), // yellow
	}

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)

	StyleHelpBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	StyleUsername = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	StyleTimestamp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	StyleMessage = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	StyleThread = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	StyleBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("214")).
			Padding(0, 1)

	StyleStatusOn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("82")).
			Padding(0, 1)

	StyleStatusOff = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)

	StyleConnected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	StyleDisconnected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	StylePaused = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
)

func ChannelStyle(channelName string) lipgloss.Style {
	h := fnv.New32a()
	h.Write([]byte(channelName))
	idx := h.Sum32() % uint32(len(channelColors))
	return lipgloss.NewStyle().Foreground(channelColors[idx])
}
```

- [ ] **Step 2: Add lipgloss dependency**

```bash
go get github.com/charmbracelet/lipgloss
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./tui/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add tui/styles.go go.mod go.sum
git commit -m "feat: add TUI styles with channel coloring"
```

---

## Task 9: TUI Messages and Keys

**Files:**
- Create: `tui/messages.go`
- Create: `tui/keys.go`

- [ ] **Step 1: Create message types**

Create `tui/messages.go`:
```go
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
```

- [ ] **Step 2: Create key bindings**

Create `tui/keys.go`:
```go
package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit       key.Binding
	Tab        key.Binding
	Pause      key.Binding
	Up         key.Binding
	Down       key.Binding
	Status     key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
	Pause: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "pause"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "scroll up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "scroll down"),
	),
	Status: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
}
```

- [ ] **Step 3: Add bubbles dependency**

```bash
go get github.com/charmbracelet/bubbles
```

- [ ] **Step 4: Verify it compiles**

```bash
go build ./tui/...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add tui/messages.go tui/keys.go go.mod go.sum
git commit -m "feat: add TUI message types and key bindings"
```

---

## Task 10: TUI Model

**Files:**
- Create: `tui/model.go`
- Create: `tui/model_test.go`

- [ ] **Step 1: Write failing test for model**

Create `tui/model_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tui/... -v
```

Expected: FAIL (NewModel not found)

- [ ] **Step 3: Implement model**

Create `tui/model.go`:
```go
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/randallmorse/slackstream/slack"
)

type View int

const (
	ViewChannels View = iota
	ViewDMs
)

type ModelConfig struct {
	MaxMessages      int
	MaxMessageLength int
	ChannelCount     int
	StatusEnabled    bool
	StatusText       string
	StatusEmoji      string
}

type Model struct {
	config          ModelConfig
	view            View
	channelMessages []slack.Message
	dmMessages      []slack.Message
	channelBadge    int
	dmBadge         int
	paused          bool
	statusOn        bool
	connected       bool
	lastUpdate      time.Time
	width           int
	height          int
	viewport        viewport.Model
	keys            KeyMap
}

func NewModel(cfg ModelConfig) *Model {
	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 500
	}
	if cfg.MaxMessageLength == 0 {
		cfg.MaxMessageLength = 120
	}

	return &Model{
		config:     cfg,
		view:       ViewChannels,
		connected:  true,
		lastUpdate: time.Now(),
		keys:       DefaultKeyMap,
		viewport:   viewport.New(80, 20),
	}
}

func (m *Model) AddMessage(msg slack.Message) {
	if msg.IsDM {
		m.dmMessages = append(m.dmMessages, msg)
		if len(m.dmMessages) > m.config.MaxMessages {
			m.dmMessages = m.dmMessages[1:]
		}
		if m.view != ViewDMs {
			m.dmBadge++
		}
	} else {
		m.channelMessages = append(m.channelMessages, msg)
		if len(m.channelMessages) > m.config.MaxMessages {
			m.channelMessages = m.channelMessages[1:]
		}
		if m.view != ViewChannels {
			m.channelBadge++
		}
	}
	m.lastUpdate = time.Now()
	m.updateViewport()
}

func (m *Model) SwitchView() {
	if m.view == ViewChannels {
		m.view = ViewDMs
		m.dmBadge = 0
	} else {
		m.view = ViewChannels
		m.channelBadge = 0
	}
	m.updateViewport()
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Tab):
			m.SwitchView()
		case key.Matches(msg, m.keys.Pause):
			m.paused = !m.paused
		case key.Matches(msg, m.keys.Up):
			m.viewport.LineUp(1)
		case key.Matches(msg, m.keys.Down):
			m.viewport.LineDown(1)
		case key.Matches(msg, m.keys.Status):
			if m.config.StatusEnabled {
				m.statusOn = !m.statusOn
				return m, func() tea.Msg {
					return StatusToggledMsg{On: m.statusOn}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 // header + status + help
		m.updateViewport()

	case NewMessageMsg:
		m.AddMessage(msg.Message)

	case TickMsg:
		cmds = append(cmds, tickCmd())

	case ConnectionStateMsg:
		m.connected = msg.Connected

	case StatusToggledMsg:
		// handled externally
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	var msgs []slack.Message
	if m.view == ViewChannels {
		msgs = m.channelMessages
	} else {
		msgs = m.dmMessages
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Timestamp.Before(msgs[j].Timestamp)
	})

	var lines []string
	for _, msg := range msgs {
		lines = append(lines, m.formatMessage(msg))
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
	if !m.paused {
		m.viewport.GotoBottom()
	}
}

func (m *Model) formatMessage(msg slack.Message) string {
	channelStyle := ChannelStyle(msg.Channel)
	channel := channelStyle.Render(fmt.Sprintf("[%s]", msg.Channel))
	user := StyleUsername.Render(fmt.Sprintf("[%s]", msg.Username))
	ts := StyleTimestamp.Render(fmt.Sprintf("[%s]", msg.Timestamp.Format("3:04pm")))

	text := msg.Text
	if len(text) > m.config.MaxMessageLength {
		text = text[:m.config.MaxMessageLength-3] + "..."
	}
	text = StyleMessage.Render(text)

	line := fmt.Sprintf("%s%s%s: %s", channel, user, ts, text)

	if msg.IsThread {
		line = StyleThread.Render("  ↳ [thread]") + line[len(msg.Channel)+2:]
	}

	return line
}

func (m *Model) View() string {
	header := m.renderHeader()
	content := m.viewport.View()
	statusBar := m.renderStatusBar()
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		statusBar,
		helpBar,
	)
}

func (m *Model) renderHeader() string {
	title := "slackstream"
	if m.view == ViewDMs {
		title = "slackstream (DMs)"
	}

	var badges []string
	if m.view == ViewChannels && m.dmBadge > 0 {
		badges = append(badges, StyleBadge.Render(fmt.Sprintf("DMs: %d new", m.dmBadge)))
	}
	if m.view == ViewDMs && m.channelBadge > 0 {
		badges = append(badges, StyleBadge.Render(fmt.Sprintf("Channels: %d new", m.channelBadge)))
	}

	if m.config.StatusEnabled {
		if m.statusOn {
			badges = append(badges, StyleStatusOn.Render("Status: ON"))
		} else {
			badges = append(badges, StyleStatusOff.Render("Status: OFF"))
		}
	}

	right := strings.Join(badges, " ")
	left := StyleHeader.Render(title)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) renderStatusBar() string {
	var status string
	if m.connected {
		status = StyleConnected.Render("●") + " Connected"
	} else {
		status = StyleDisconnected.Render("◌") + " Reconnecting..."
	}

	status += fmt.Sprintf(" | %d channels", m.config.ChannelCount)
	status += fmt.Sprintf(" | Last update: %s ago", time.Since(m.lastUpdate).Round(time.Second))

	if m.paused {
		status += " | " + StylePaused.Render("PAUSED")
	}

	return StyleStatusBar.Render(status)
}

func (m *Model) renderHelpBar() string {
	help := "[Tab] switch view │ [Space] pause │ [↑/↓] scroll │ [q] quit"
	if m.config.StatusEnabled {
		help = "[Tab] switch view │ [Space] pause │ [s] status │ [↑/↓] scroll │ [q] quit"
	}
	return StyleHelpBar.Render(help)
}
```

- [ ] **Step 4: Add bubbletea dependency**

```bash
go get github.com/charmbracelet/bubbletea
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./tui/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tui/model.go tui/model_test.go go.mod go.sum
git commit -m "feat: add TUI model with dual views and badge notifications"
```

---

## Task 11: Main Application Wiring

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Implement main application**

Replace `main.go`:
```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/randallmorse/slackstream/audio"
	"github.com/randallmorse/slackstream/config"
	"github.com/randallmorse/slackstream/slack"
	"github.com/randallmorse/slackstream/tui"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: ~/.slackstream.yaml)")
	flag.Parse()

	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		*configPath = filepath.Join(home, ".slackstream.yaml")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nExample config (~/.slackstream.yaml):\n")
		fmt.Fprintf(os.Stderr, `
token: "xoxp-your-token"  # or set SLACK_TOKEN env var
channels:
  - "#engineering"
  - "#general"
dms: true
settings:
  poll_interval: 3s
`)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	client := slack.NewClient(cfg.Token)

	player := audio.NewPlayer(audio.PlayerConfig{
		Enabled:         cfg.Sounds.Enabled,
		ChannelsEnabled: cfg.Sounds.Channels.Enabled,
		ChannelsSound:   cfg.Sounds.Channels.Sound,
		DMsEnabled:      cfg.Sounds.DMs.Enabled,
		DMsSound:        cfg.Sounds.DMs.Sound,
		Debounce:        cfg.Sounds.Debounce,
	})

	messages := make(chan slack.Message, 100)

	for _, channelRef := range cfg.Channels {
		channelID, err := client.ResolveChannelID(ctx, channelRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			continue
		}

		channelName, _ := client.GetChannelName(ctx, channelID)

		poller := slack.NewPoller(slack.PollerConfig{
			Client:      client,
			ChannelID:   channelID,
			ChannelName: "#" + channelName,
			IsDM:        false,
			Interval:    cfg.Settings.PollInterval,
			ShowThreads: cfg.Settings.ShowThreads,
			Messages:    messages,
		})
		go poller.Run(ctx)
	}

	if cfg.DMs {
		dms, err := client.GetDMConversations(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list DMs: %v\n", err)
		} else {
			for _, dm := range dms {
				poller := slack.NewPoller(slack.PollerConfig{
					Client:      client,
					ChannelID:   dm.ID,
					ChannelName: dm.Name,
					IsDM:        true,
					Interval:    cfg.Settings.PollInterval,
					ShowThreads: cfg.Settings.ShowThreads,
					Messages:    messages,
				})
				go poller.Run(ctx)
			}
		}
	}

	model := tui.NewModel(tui.ModelConfig{
		MaxMessages:      500,
		MaxMessageLength: cfg.Settings.MaxMessageLength,
		ChannelCount:     len(cfg.Channels),
		StatusEnabled:    cfg.Status.Enabled,
		StatusText:       cfg.Status.Text,
		StatusEmoji:      cfg.Status.Emoji,
	})

	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-messages:
				p.Send(tui.NewMessageMsg{Message: msg})

				if msg.IsDM {
					player.PlayDM()
				} else {
					player.PlayChannel()
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				p.Send(tea.Quit())
				return
			}
		}
	}()

	_, err := p.Run()
	return err
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build -o slackstream .
```

Expected: binary created

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire up main application with pollers, TUI, and audio"
```

---

## Task 12: Status Toggle Integration

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add status toggle handling**

Find and update the message loop in `main.go` to handle status toggles. Replace the `go func()` that handles messages:

```go
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-messages:
				p.Send(tui.NewMessageMsg{Message: msg})

				if msg.IsDM {
					player.PlayDM()
				} else {
					player.PlayChannel()
				}
			}
		}
	}()
```

Add a custom message handler after creating the program. Update the full `run` function to include a goroutine that listens for status toggle events. This requires wrapping the model to intercept StatusToggledMsg.

Actually, a cleaner approach is to handle this in the TUI's Update. Let's modify `tui/model.go` to accept a status toggle callback:

In `tui/model.go`, update `ModelConfig`:
```go
type ModelConfig struct {
	MaxMessages      int
	MaxMessageLength int
	ChannelCount     int
	StatusEnabled    bool
	StatusText       string
	StatusEmoji      string
	OnStatusToggle   func(on bool) error
}
```

Update the `Update` method's status handling:
```go
		case key.Matches(msg, m.keys.Status):
			if m.config.StatusEnabled {
				m.statusOn = !m.statusOn
				if m.config.OnStatusToggle != nil {
					go m.config.OnStatusToggle(m.statusOn)
				}
			}
```

Update `main.go` model creation:
```go
	model := tui.NewModel(tui.ModelConfig{
		MaxMessages:      500,
		MaxMessageLength: cfg.Settings.MaxMessageLength,
		ChannelCount:     len(cfg.Channels),
		StatusEnabled:    cfg.Status.Enabled,
		StatusText:       cfg.Status.Text,
		StatusEmoji:      cfg.Status.Emoji,
		OnStatusToggle: func(on bool) error {
			if on {
				return client.SetStatus(ctx, cfg.Status.Text, cfg.Status.Emoji)
			}
			return client.ClearStatus(ctx)
		},
	})
```

- [ ] **Step 2: Verify it builds**

```bash
go build -o slackstream .
```

Expected: binary created

- [ ] **Step 3: Commit**

```bash
git add main.go tui/model.go
git commit -m "feat: integrate slack status toggle with TUI"
```

---

## Task 13: README Update

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README with setup instructions**

Add a "Quick Start" section to `README.md` after the existing content:

```markdown
---

## Quick Start

### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click "Create New App" → "From scratch"
3. Name it "SlackStream" and select your workspace

### 2. Add OAuth Scopes

Under "OAuth & Permissions", add these User Token Scopes:

- `channels:history` — read channel messages
- `channels:read` — list channels
- `im:history` — read DM messages
- `im:read` — list DM conversations
- `users:read` — resolve usernames
- `users.profile:write` — set status (optional)

### 3. Install to Workspace

Click "Install to Workspace" and authorize. Copy the **User OAuth Token** (`xoxp-...`).

### 4. Configure SlackStream

Create `~/.slackstream.yaml`:

```yaml
token: "xoxp-your-token-here"  # or set SLACK_TOKEN env var

channels:
  - "#engineering"
  - "#incidents"
  - "#general"

dms: true

settings:
  poll_interval: 3s
  max_message_length: 120
  show_threads: true
  show_bots: true

sounds:
  enabled: true
  channels:
    enabled: true
    sound: "tick"
  dms:
    enabled: true
    sound: "tah"
  debounce: 3s

status:
  enabled: true
  text: "Work Session In Progress - DM to Reach Me"
  emoji: ":headphones:"
```

### 5. Run

```bash
./slackstream
```

Or with a custom config:

```bash
./slackstream -config /path/to/config.yaml
```

### Controls

| Key | Action |
|-----|--------|
| `Tab` | Switch between Channels and DMs |
| `Space` | Pause/resume scrolling |
| `↑/↓` or `j/k` | Scroll through history |
| `s` | Toggle Slack status |
| `q` | Quit |

### Building from Source

```bash
go build -o slackstream .
```
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add quick start and setup instructions"
```

---

## Task 14: Final Integration Test

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: All tests pass

- [ ] **Step 2: Build final binary**

```bash
go build -o slackstream .
```

- [ ] **Step 3: Verify binary runs with help**

```bash
./slackstream -help
```

Expected: Shows usage with -config flag

- [ ] **Step 4: Create release commit**

```bash
git add .
git commit -m "feat: slackstream v0.1.0 - initial release"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Project init | go.mod, main.go |
| 2 | Config loading | config/config.go |
| 3 | Slack types | slack/types.go |
| 4 | Slack client | slack/client.go |
| 5 | Status toggle | slack/status.go |
| 6 | Poller | slack/poller.go |
| 7 | Audio player | audio/player.go |
| 8 | TUI styles | tui/styles.go |
| 9 | TUI messages/keys | tui/messages.go, keys.go |
| 10 | TUI model | tui/model.go |
| 11 | Main wiring | main.go |
| 12 | Status integration | main.go, tui/model.go |
| 13 | README | README.md |
| 14 | Final test | - |
