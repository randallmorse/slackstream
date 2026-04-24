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
		OnStatusToggle: func(on bool) error {
			if on {
				return client.SetStatus(ctx, cfg.Status.Text, cfg.Status.Emoji)
			}
			return client.ClearStatus(ctx)
		},
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
