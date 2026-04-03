package player

import (
	"context"
	"sync"
	"time"

	"github.com/huntiezz/termflix/internal/decoder"
	"github.com/huntiezz/termflix/internal/media"
	"github.com/huntiezz/termflix/internal/render"
	"github.com/huntiezz/termflix/internal/source"
	"github.com/huntiezz/termflix/internal/util"
)

// Config controls player behaviour.
type Config struct {
	FFMPEGPath string
	FPS        int
	Mode       util.RenderMode
	FitMode    util.FitMode
	Width      int
	Height     int
}

// AudioController abstracts subprocess-based audio playback.
type AudioController interface {
	Play(offset time.Duration) error
	Stop() error
	Mute(muted bool) error
	IsMuted() bool
}

// State represents immutable snapshot of playback state.
type State struct {
	Position time.Duration
	Duration time.Duration
	Paused   bool
	Muted    bool
	Mode     util.RenderMode
	FitMode  util.FitMode
	FPS      int
	FPSActual float64
	Frame    render.ScaledFrame
}

// Player coordinates decoding, timing, and state.
type Player struct {
	src   source.Source
	meta  media.Info
	cfg   Config
	audio AudioController

	mu       sync.RWMutex
	state    State
	cancelFn context.CancelFunc
}

func New(src source.Source, meta media.Info, cfg Config, audio AudioController) *Player {
	p := &Player{
		src:   src,
		meta:  meta,
		cfg:   cfg,
		audio: audio,
	}
	p.state.Duration = meta.Duration
	p.state.Mode = cfg.Mode
	p.state.FitMode = cfg.FitMode
	p.state.FPS = cfg.FPS
	p.state.Muted = audio != nil && audio.IsMuted()
	return p
}

// Snapshot returns current state for UI.
func (p *Player) Snapshot() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Start begins playback loop and decoding. It should be called once.
func (p *Player) Start(ctx context.Context, termCols, termRows int) {
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	go p.run(ctx, termCols, termRows)
}

func (p *Player) run(ctx context.Context, termCols, termRows int) {
	var (
		frameCh <-chan decoder.Frame
		errCh   <-chan error
	)

	startDecoder := func(offset time.Duration) {
		dec := decoder.New(decoder.Config{
			FFMPEGPath: p.cfg.FFMPEGPath,
			InputURL:   p.meta.StreamURL,
			Width:      p.cfg.Width,
			Height:     p.cfg.Height,
			FPS:        p.cfg.FPS,
			StartAt:    offset.Seconds(),
		})
		var err error
		frameCh, errCh, err = dec.Start(ctx)
		if err != nil {
			return
		}
	}

	startDecoder(0)
	if p.audio != nil && !p.audio.IsMuted() {
		_ = p.audio.Play(0)
	}

	tick := time.NewTicker(time.Second / time.Duration(p.cfg.FPS))
	defer tick.Stop()

	var lastFrameTime time.Time
	for {
		select {
		case <-ctx.Done():
			if p.audio != nil {
				_ = p.audio.Stop()
			}
			return
		case err := <-errCh:
			_ = err
			return
		case f, ok := <-frameCh:
			if !ok {
				return
			}
			p.handleFrame(f, termCols, termRows, &lastFrameTime)
		case <-tick.C:
			// timing tick; position update is derived from frames.
		}
	}
}

func (p *Player) handleFrame(f decoder.Frame, termCols, termRows int, last *time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.Paused {
		return
	}

	buf := render.FrameBuffer{
		Width:  f.Width,
		Height: f.Height,
		Data:   f.Data,
	}
	sf := render.ScaleFrame(buf, termCols, termRows, p.state.FitMode == util.FitModeFit)

	p.state.Frame = sf

	now := time.Now()
	if !last.IsZero() {
		d := now.Sub(*last).Seconds()
		if d > 0 {
			p.state.FPSActual = 1 / d
		}
	}
	*last = now

	if p.state.Duration > 0 {
		step := time.Second / time.Duration(p.cfg.FPS)
		p.state.Position += step
		if p.state.Position > p.state.Duration {
			p.state.Position = p.state.Duration
		}
	}
}

func (p *Player) TogglePause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Paused = !p.state.Paused
}

func (p *Player) ToggleMute() {
	p.mu.Lock()
	defer p.mu.Unlock()
	newMuted := !p.state.Muted
	p.state.Muted = newMuted
	if p.audio != nil {
		_ = p.audio.Mute(newMuted)
	}
}

func (p *Player) CycleMode() {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.state.Mode {
	case util.RenderModeBlocks:
		p.state.Mode = util.RenderModeBraille
	case util.RenderModeBraille:
		p.state.Mode = util.RenderModeASCII
	default:
		p.state.Mode = util.RenderModeBlocks
	}
}

func (p *Player) ToggleFit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state.FitMode == util.FitModeFit {
		p.state.FitMode = util.FitModeFill
	} else {
		p.state.FitMode = util.FitModeFit
	}
}

// Seek is currently implemented as a logical seek only. For MVP,
// restarting ffmpeg is left as an extension point.
func (p *Player) Seek(delta time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pos := p.state.Position + delta
	if pos < 0 {
		pos = 0
	}
	if p.state.Duration > 0 && pos > p.state.Duration {
		pos = p.state.Duration
	}
	p.state.Position = pos
}

