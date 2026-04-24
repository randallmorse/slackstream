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
