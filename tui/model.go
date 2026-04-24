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
