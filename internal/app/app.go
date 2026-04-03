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

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

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

	startedFromLauncher := false
	if cfg.Input == "" {
		res, err := tui.RunLauncher(ctx, cfg)
		if err != nil {
			return err
		}
		if !res.Started {
			return nil
		}
		cfg = res.Config
		startedFromLauncher = true
	}

	if err := util.CheckRequiredBinaries(cfg); err != nil {
		if startedFromLauncher {
			return tui.RunError(ctx, err)
		}
		return err
	}
	// Resolve to absolute paths when possible (Windows-friendly).
	if p, err := util.ResolveBinary(cfg.FFMPEGPath, "ffmpeg"); err == nil && p != "" {
		cfg.FFMPEGPath = p
	}
	if p, err := util.ResolveBinary(cfg.FFProbePath, "ffprobe"); err == nil && p != "" {
		cfg.FFProbePath = p
	}
	if p, err := util.ResolveBinary(cfg.MPVPath, "mpv"); err == nil && p != "" {
		cfg.MPVPath = p
	}
	if p, err := util.ResolveBinary(cfg.FFPlayPath, "ffplay"); err == nil && p != "" {
		cfg.FFPlayPath = p
	}

	// Best-effort resolve yt-dlp upfront so YouTube URLs work even when
	// installed by winget/scoop but not yet on PATH.
	if p, err := util.ResolveBinary(cfg.YTDLPPath, "yt-dlp"); err == nil && p != "" {
		cfg.YTDLPPath = p
	}

	src, err := source.Detect(ctx, cfg.Input, cfg.YTDLPPath)
	if err != nil {
		if startedFromLauncher {
			return tui.RunError(ctx, err)
		}
		return err
	}

	if src.Type == source.TypeYouTube {
		if err := util.CheckYouTubeBinary(cfg.YTDLPPath); err != nil {
			if startedFromLauncher {
				return tui.RunError(ctx, err)
			}
			return err
		}
	}

	meta, err := media.Probe(ctx, cfg.FFProbePath, src)
	if err != nil {
		wrapped := fmt.Errorf("probe media: %w", err)
		if startedFromLauncher {
			return tui.RunError(ctx, wrapped)
		}
		return wrapped
	}

	playCfg := player.Config{
		FFMPEGPath: cfg.FFMPEGPath,
		FPSCap:     cfg.FPS,
		Mode:       cfg.Mode,
		FitMode:    cfg.FitMode,
		Width:      cfg.Width,
		Height:     cfg.Height,
	}

	// If the user didn't request an audio engine explicitly, pick one if available.
	if cfg.AudioEngine == util.AudioEngineNone && !cfg.Mute {
		if cfg.MPVPath != "" {
			cfg.AudioEngine = util.AudioEngineMPV
		} else if cfg.FFPlayPath != "" {
			cfg.AudioEngine = util.AudioEngineFFPlay
		}
	}

	audioCtrl := audio.NewController(audio.Options{
		Engine:   cfg.AudioEngine,
		Muted:    cfg.Mute,
		FFPlay:   cfg.FFPlayPath,
		MPV:      cfg.MPVPath,
		InputURL: firstNonEmpty(src.AudioURL, src.PlayURL),
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res, err := tui.RunProgram(ctx, model)
	if err != nil {
		if startedFromLauncher {
			return tui.RunError(ctx, err)
		}
		return err
	}

	// Grace period for cleanup.
	select {
	case <-ctx.Done():
	case <-time.After(50 * time.Millisecond):
	}

	if res.Err != nil {
		if startedFromLauncher {
			return tui.RunError(ctx, res.Err)
		}
		return res.Err
	}

	return nil
}

