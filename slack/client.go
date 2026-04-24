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
