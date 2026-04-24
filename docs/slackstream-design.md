# slackstream Design Spec

A terminal UI for ambient Slack awareness — monitor multiple channels and DMs in a single scrolling feed without the distraction of the full Slack app.

## Overview

**Problem:** Slack is distracting, but you need to stay aware of a few key channels and DMs.

**Solution:** A lightweight TUI that polls Slack channels and displays messages in a unified, scrolling view. Runs in a terminal pane for passive awareness.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    slackstream                      │
├─────────────┬─────────────────┬────────────────────┤
│   Config    │    Poller       │      TUI           │
│   Loader    │   (per channel) │   (bubbletea)      │
├─────────────┼─────────────────┼────────────────────┤
│ YAML parser │ conversations.  │ Scrolling viewport │
│ Validation  │ history API     │ Pause/resume       │
│             │ Deduplication   │ Color coding       │
│             │                 │ Sound alerts       │
└─────────────┴─────────────────┴────────────────────┘
                      │
              Slack Web API
```

### Components

1. **Config Loader** — reads `~/.slackstream.yaml`, validates channels, handles token from file or env var
2. **Poller** — one goroutine per channel/DM source, polls every N seconds, deduplicates by message timestamp
3. **Message Bus** — all pollers send to a single channel, messages sorted by timestamp for unified view
4. **TUI** — bubbletea-based with two views (Channels, DMs), minimal controls, sound notifications

### Data Flow

1. Config loaded at startup, validated
2. Poller goroutines started for each channel + DMs
3. Each poller calls `conversations.history` on interval
4. New messages (not seen before) sent to message bus
5. TUI receives messages, appends to view buffer, triggers sound if enabled
6. View auto-scrolls unless paused

## API Approach

**Method:** Polling via Slack Web API (`conversations.history`)

**Why polling over Socket Mode:**
- Simpler auth — works with user token, no bot setup
- No Slack app installation required
- Acceptable latency (2-5 seconds) for ambient awareness use case

**Rate limits:** Tier 3 (~50 req/min). With 5 channels + DMs polling every 3s = ~12 req/min. Well within limits.

## Message Format

```
[#engineering][alice][10:42am]: deployed v2.3.1 to staging
[#engineering][bob][10:43am]: 👍 looks good
  ↳ [thread][alice][10:44am]: thanks, will push to prod in 30
[#incidents][oncall-bot][10:45am]: Alert resolved: db-latency
[DM @carol][carol][10:46am]: hey, got a minute?
```

**Formatting rules:**
- Channel name in color (consistent color per channel)
- Username in bold
- Thread replies indented with `↳ [thread]` prefix
- DMs shown as `[DM @username]`
- Timestamps in local time, condensed format (10:42am)
- Long messages truncated at configurable max width with `...`
- All message types shown (including bots, system messages)

## Views & Navigation

### Layout

```
┌─ slackstream ───────────────────── [DMs: 2 new] [Status: ON] ─┐
│ [#engineering][alice][10:42am]: deployed v2.3.1...            │
│ [#engineering][bob][10:43am]: 👍 looks good                   │
│   ↳ [thread][alice][10:44am]: thanks, will push...            │
│ [#incidents][oncall-bot][10:45am]: Alert resolved             │
│ [#general][carol][10:46am]: anyone up for lunch?              │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ ● Connected | 3 channels | Last update: 2s ago                │
├───────────────────────────────────────────────────────────────┤
│ [Tab] switch view │ [Space] pause │ [s] status │ [q] quit     │
└───────────────────────────────────────────────────────────────┘
```

### Controls

| Key | Action |
|-----|--------|
| `Tab` | Toggle between Channels and DMs view |
| `Space` | Pause/resume auto-scroll |
| `↑/↓` or `j/k` | Scroll through history (when paused) |
| `s` | Toggle Slack status on/off |
| `q` | Quit |

### Notification Badges

- **Symmetrical:** Each view shows badge for the other
- In Channels view: header shows `[DMs: N new]`
- In DMs view: header shows `[Channels: N new]`
- Badge count resets when switching to that view

## Slack Status Feature

**Purpose:** Signal to teammates that you're in a work session and DMs are the way to reach you.

**Behavior:**
- Press `s` to toggle status on/off (manual control only)
- Header shows `[Status: ON]` or `[Status: OFF]` indicator
- Status NOT automatically set on start or cleared on quit
- Fully optional — works without this feature configured

**Requires:** Additional OAuth scope `users.profile:write`

## Sound Notifications

**Purpose:** Ambient audio cues without visual attention.

**Sounds:**
- Channels: "tick" sound
- DMs: "tah" sound
- Built-in sounds embedded in binary: `tick`, `tah`, `pop`, `ding`
- Custom sounds supported via file path

**Behavior:**
- Debounced per type — max one sound per 3 seconds per category
- If 5 messages arrive in quick succession, only one sound plays
- Channels and DMs have separate debounce timers
- Configurable: can disable either or both

**Implementation:** `github.com/faiface/beep` for cross-platform audio. Sounds embedded via `go:embed`.

## Configuration

**Location:** `~/.slackstream.yaml`

```yaml
# Slack authentication
token: "xoxp-your-user-token"  # or use SLACK_TOKEN env var

# Channels to monitor (name or ID)
channels:
  - "#engineering"
  - "#incidents"
  - "#general"

# DM monitoring
dms: true

# Polling and display settings
settings:
  poll_interval: 3s
  max_message_length: 120
  show_threads: true
  show_bots: true

# Sound notifications
sounds:
  enabled: true
  channels:
    enabled: true
    sound: "tick"           # built-in or path to .wav/.mp3
  dms:
    enabled: true
    sound: "tah"
  debounce: 3s              # max one sound per type per interval

# Slack status (optional)
status:
  enabled: true
  text: "Work Session In Progress - DM to Reach Me"
  emoji: ":headphones:"     # optional status emoji
```

**Token handling:**
- `SLACK_TOKEN` env var takes precedence over config file
- Token is a user token (`xoxp-...`), not a bot token

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid/missing token | Exit with clear error message and setup instructions |
| Channel not found | Log warning, continue with other channels |
| Rate limited (429) | Exponential backoff, show indicator in status bar |
| Network loss | Auto-reconnect with backoff, show "Reconnecting..." status |
| Channel access revoked | Remove from rotation, log warning |
| Empty/missing config | Exit with example config snippet |

**Status bar states:**
- `● Connected | 3 channels | Last update: 2s ago` — normal
- `◌ Reconnecting... | Last update: 45s ago` — network issues
- `⚠ Rate limited | Backing off...` — API throttling

## Project Structure

```
slackstream/
├── main.go                 # entry point, CLI flags
├── config/
│   └── config.go           # YAML parsing, validation
├── slack/
│   ├── client.go           # Slack API wrapper
│   ├── poller.go           # per-channel polling logic
│   ├── status.go           # status toggle logic
│   └── types.go            # message types, deduplication
├── tui/
│   ├── app.go              # bubbletea model, Update/View
│   ├── channels_view.go    # channels tab rendering
│   ├── dms_view.go         # DMs tab rendering
│   └── styles.go           # lipgloss styles, colors
├── audio/
│   ├── player.go           # sound playback, debouncing
│   └── sounds/             # embedded .wav files
│       ├── tick.wav
│       ├── tah.wav
│       ├── pop.wav
│       └── ding.wav
├── docs/
│   └── slackstream-design.md
├── README.md               # setup guide, usage, screenshots
└── go.mod
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Styling and colors |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/faiface/beep` | Cross-platform audio playback |
| `gopkg.in/yaml.v3` | Config file parsing |

## Slack Token Setup

Users need a Slack user token with these scopes:
- `channels:history` — read channel messages
- `channels:read` — list channels, get channel info
- `im:history` — read DM messages
- `im:read` — list DM conversations
- `users:read` — resolve user IDs to names
- `users.profile:write` — set/clear status (optional, only if status feature used)

**Setup steps** (to be documented in README):
1. Go to api.slack.com/apps, create new app
2. Add required OAuth scopes
3. Install to workspace
4. Copy user token (`xoxp-...`)
5. Set as `SLACK_TOKEN` env var or in config file

## Out of Scope

- Sending messages (read-only)
- Reactions, edits, deletions (show original only)
- File/image previews (show filename only)
- Multi-workspace support
- Message search/filtering
- Keyboard shortcuts for opening messages in Slack

## Success Criteria

1. Can monitor 2-5 channels + DMs with <5 second latency
2. Runs stably for hours without memory growth or crashes
3. Single binary, no runtime dependencies
4. Clear setup instructions get user from zero to working in <5 minutes
5. Sounds provide awareness without being annoying
6. Status toggle works reliably without affecting other Slack state
