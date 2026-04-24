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
