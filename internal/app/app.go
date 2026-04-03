package app

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	"github.com/huntiezz/termflix/internal/audio"
	"github.com/huntiezz/termflix/internal/media"
	"github.com/huntiezz/termflix/internal/player"
	"github.com/huntiezz/termflix/internal/source"
	"github.com/huntiezz/termflix/internal/term"
	"github.com/huntiezz/termflix/internal/tui"
	"github.com/huntiezz/termflix/internal/util"
)

// App wires the high-level application pieces together.
type App struct {
	args   []string
	logger *log.Logger
}

// New constructs an App from CLI args.
func New(_ context.Context, args []string, logger *log.Logger) (*App, error) {
	return &App{
		args:   args,
		logger: logger,
	}, nil
}

// Run parses CLI flags, validates dependencies, and launches the TUI player.
func (a *App) Run(ctx context.Context) error {
	cfg, err := util.ParseFlags(a.args)
	if err != nil {
		return err
	}

	if err := term.EnsureTerminal(); err != nil {
		return err
	}

	if cfg.Input == "" {
		res, err := tui.RunLauncher(ctx, cfg)
		if err != nil {
			return err
		}
		if !res.Started {
			return nil
		}
		cfg = res.Config
	}

	if err := util.CheckRequiredBinaries(cfg); err != nil {
		return err
	}

	src, err := source.Detect(ctx, cfg.Input, cfg.YTDLPPath)
	if err != nil {
		return err
	}

	meta, err := media.Probe(ctx, cfg.FFProbePath, src)
	if err != nil {
		return fmt.Errorf("probe media: %w", err)
	}

	playCfg := player.Config{
		FFMPEGPath: cfg.FFMPEGPath,
		FPS:        cfg.FPS,
		Mode:       cfg.Mode,
		FitMode:    cfg.FitMode,
		Width:      cfg.Width,
		Height:     cfg.Height,
	}

	audioCtrl := audio.NewController(audio.Options{
		Engine:   cfg.AudioEngine,
		Muted:    cfg.Mute,
		FFPlay:   cfg.FFPlayPath,
		MPV:      cfg.MPVPath,
		InputURL: src.PlayURL,
		SeekFunc: nil, // wired by player as needed
	})

	p := player.New(src, meta, playCfg, audioCtrl)

	model := tui.NewModel(p, tui.Config{
		ShowUI:       !cfg.NoUI,
		InitialMode:  cfg.Mode,
		FitMode:      cfg.FitMode,
		Title:        meta.DisplayTitle(),
		Duration:     meta.Duration,
		InitialMuted: cfg.Mute,
		FPS:          cfg.FPS,
	})

	if err := term.EnableAltScreen(); err != nil {
		return err
	}
	defer func() {
		_ = term.Restore()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res, err := tui.RunProgram(ctx, model)
	if err != nil {
		return err
	}

	// Grace period for cleanup.
	select {
	case <-ctx.Done():
	case <-time.After(50 * time.Millisecond):
	}

	if res.Err != nil {
		return res.Err
	}

	return nil
}

