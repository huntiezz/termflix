package util

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type FitMode string

const (
	FitModeFit  FitMode = "fit"
	FitModeFill FitMode = "fill"
)

type RenderMode string

const (
	RenderModeBlocks  RenderMode = "blocks"
	RenderModeBraille RenderMode = "braille"
	RenderModeASCII   RenderMode = "ascii"
)

type AudioEngine string

const (
	AudioEngineNone  AudioEngine = "none"
	AudioEngineFFPlay AudioEngine = "ffplay"
	AudioEngineMPV    AudioEngine = "mpv"
)

// Config holds parsed CLI configuration.
type Config struct {
	Input       string
	Mode        RenderMode
	FPS         int
	Mute        bool
	AudioEngine AudioEngine
	Width       int
	Height      int
	NoUI        bool
	FitMode     FitMode

	// External binaries, overrideable for testing.
	FFMPEGPath  string
	FFProbePath string
	YTDLPPath   string
	FFPlayPath  string
	MPVPath     string
}

// ParseFlags parses CLI arguments into Config.
func ParseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("termflix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		modeStr   = fs.String("mode", string(RenderModeBlocks), "render mode: blocks|braille|ascii")
		fps       = fs.Int("fps", 15, "max frames per second")
		mute      = fs.Bool("mute", false, "start muted")
		audioStr  = fs.String("audio", string(AudioEngineNone), "audio backend: none|mpv|ffplay")
		width     = fs.Int("width", 0, "target width (0=auto)")
		height    = fs.Int("height", 0, "target height (0=auto)")
		noUI      = fs.Bool("no-ui", false, "disable TUI and render raw frames only")
		fit       = fs.Bool("fit", true, "scale to fit terminal (default)")
		fill      = fs.Bool("fill", false, "scale to fill terminal")
		ffmpeg    = fs.String("ffmpeg", "ffmpeg", "path to ffmpeg binary")
		ffprobe   = fs.String("ffprobe", "ffprobe", "path to ffprobe binary")
		ytdlp     = fs.String("yt-dlp", "yt-dlp", "path to yt-dlp binary")
		ffplay    = fs.String("ffplay", "ffplay", "path to ffplay binary")
		mpv       = fs.String("mpv", "mpv", "path to mpv binary")
	)

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	rest := fs.Args()
	input := ""
	if len(rest) > 0 {
		input = rest[0]
	}

	mode := RenderMode(strings.ToLower(*modeStr))
	switch mode {
	case RenderModeBlocks, RenderModeBraille, RenderModeASCII:
	default:
		return Config{}, fmt.Errorf("invalid mode %q (expected blocks|braille|ascii)", *modeStr)
	}

	audioMode := AudioEngine(strings.ToLower(*audioStr))
	switch audioMode {
	case AudioEngineNone, AudioEngineFFPlay, AudioEngineMPV:
	default:
		return Config{}, fmt.Errorf("invalid audio engine %q (expected none|mpv|ffplay)", *audioStr)
	}

	if *fps <= 0 {
		return Config{}, fmt.Errorf("fps must be positive, got %d", *fps)
	}

	fitMode := FitModeFit
	if *fill {
		fitMode = FitModeFill
	}
	if *fit && *fill {
		return Config{}, errors.New("flags --fit and --fill are mutually exclusive")
	}

	return Config{
		Input:       input,
		Mode:        mode,
		FPS:         *fps,
		Mute:        *mute,
		AudioEngine: audioMode,
		Width:       *width,
		Height:      *height,
		NoUI:        *noUI,
		FitMode:     fitMode,

		FFMPEGPath:  *ffmpeg,
		FFProbePath: *ffprobe,
		YTDLPPath:   *ytdlp,
		FFPlayPath:  *ffplay,
		MPVPath:     *mpv,
	}, nil
}

