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
