package player

import (
	"context"
	"fmt"
	"math"
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
	FPSCap     int
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
	Err      string
	Seq      uint64
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
	p.state.FPS = cfg.FPSCap
	p.state.Muted = audio != nil && audio.IsMuted()
	return p
}

// Snapshot returns current state for UI.
func (p *Player) Snapshot() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Start begins (or restarts) playback with the given terminal size.
func (p *Player) Start(ctx context.Context, termCols, termRows int) {
	// Cancel any prior decode loop (resize/restart).
	if p.cancelFn != nil {
		p.cancelFn()
	}
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	go p.run(ctx, termCols, termRows)
}

// Stop cancels decoding and stops audio playback.
func (p *Player) Stop() {
	if p.cancelFn != nil {
		p.cancelFn()
	}
	if p.audio != nil {
		_ = p.audio.Stop()
	}
}

func (p *Player) run(ctx context.Context, termCols, termRows int) {
	var (
		frameCh <-chan decoder.Frame
		errCh   <-chan error
	)

	startDecoder := func(offset time.Duration) {
		w, h := p.decodeSize(termCols, termRows)
		if w <= 0 || h <= 0 {
			p.setErr(fmt.Errorf("could not determine decode size (term %dx%d, media %dx%d)", termCols, termRows, p.meta.Width, p.meta.Height))
			return
		}
		dec := decoder.New(decoder.Config{
			FFMPEGPath: p.cfg.FFMPEGPath,
			InputURL:   p.meta.StreamURL,
			Width:      w,
			Height:     h,
			FPSCap:     p.cfg.FPSCap,
			StartAt:    offset.Seconds(),
			Realtime:   true,
		})
		var err error
		frameCh, errCh, err = dec.Start(ctx)
		if err != nil {
			p.setErr(err)
			return
		}
	}

	startDecoder(0)
	if frameCh == nil {
		// decoder didn't start; keep UI alive for error display
		<-ctx.Done()
		return
	}
	if p.audio != nil && !p.audio.IsMuted() {
		_ = p.audio.Play(0)
	}

	var lastFrameTime time.Time
	for {
		select {
		case <-ctx.Done():
			if p.audio != nil {
				_ = p.audio.Stop()
			}
			return
		case err := <-errCh:
			p.setErr(err)
			return
		case f, ok := <-frameCh:
			if !ok {
				return
			}
			p.handleFrame(f, termCols, termRows, &lastFrameTime)
		}
	}
}

func (p *Player) setErr(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Err = err.Error()
	// Pause so the last frame stays visible (or blank) and status reflects a stop.
	p.state.Paused = true
}

// decodeSize chooses an ffmpeg output size based on terminal and media dimensions.
// termRows counts terminal rows; for block rendering we treat one cell as ~2 vertical pixels.
func (p *Player) decodeSize(termCols, termRows int) (int, int) {
	if p.cfg.Width > 0 && p.cfg.Height > 0 {
		return even(p.cfg.Width), even(p.cfg.Height)
	}
	if termCols <= 0 || termRows <= 0 {
		return 0, 0
	}

	// Target pixel budget roughly matching "blocks" mode vertical resolution.
	targetW := termCols
	targetH := termRows * 2

	srcW := p.meta.Width
	srcH := p.meta.Height
	if srcW <= 0 || srcH <= 0 {
		// Safe fallback if metadata is missing.
		return even(targetW), even(targetH)
	}

	sx := float64(targetW) / float64(srcW)
	sy := float64(targetH) / float64(srcH)

	scale := sx
	if p.state.FitMode == util.FitModeFit {
		if sy < sx {
			scale = sy
		}
	} else { // fill
		if sy > sx {
			scale = sy
		}
	}

	w := int(math.Round(float64(srcW) * scale))
	h := int(math.Round(float64(srcH) * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	// Keep within budget.
	if p.state.FitMode == util.FitModeFit {
		if w > targetW {
			w = targetW
		}
		if h > targetH {
			h = targetH
		}
	}
	return even(w), even(h)
}

func even(v int) int {
	if v <= 1 {
		return 1
	}
	if v%2 == 1 {
		return v - 1
	}
	return v
}

func (p *Player) handleFrame(f decoder.Frame, termCols, termRows int, last *time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.Paused {
		return
	}
	p.state.Err = ""
	p.state.Seq++

	buf := render.FrameBuffer{
		Width:  f.Width,
		Height: f.Height,
		Data:   f.Data,
	}
	pxPerCellY := 2
	switch p.state.Mode {
	case util.RenderModeBlocks:
		pxPerCellY = 2
	case util.RenderModeBraille:
		pxPerCellY = 4
	case util.RenderModeASCII:
		pxPerCellY = 1
	}
	sf := render.ScaleFrame(buf, termCols, termRows, p.state.FitMode == util.FitModeFit, pxPerCellY)

	p.state.Frame = sf

	now := time.Now()
	if !last.IsZero() {
		dt := now.Sub(*last)
		sec := dt.Seconds()
		if sec > 0 {
			p.state.FPSActual = 1 / sec
		}

		// Drive position from real-time frame spacing. This stays correct even if
		// rendering is slow or fps is capped.
		p.state.Position += dt
		if p.state.Duration > 0 && p.state.Position > p.state.Duration {
			p.state.Position = p.state.Duration
		}
	}
	*last = now
}

func (p *Player) effectiveFPS() int {
	srcFPS := int(p.meta.FPS + 0.5)
	if srcFPS <= 0 {
		return p.cfg.FPSCap
	}
	// Default: use the source FPS exactly.
	if p.cfg.FPSCap <= 0 {
		return srcFPS
	}
	if srcFPS < p.cfg.FPSCap {
		return srcFPS
	}
	return p.cfg.FPSCap
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

