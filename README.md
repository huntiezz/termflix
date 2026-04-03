### termflix

**termflix** is a terminal video player written in Go. It lets you watch local files or YouTube URLs directly in your terminal using text/Unicode rendering modes – truecolor half‑blocks, braille, and grayscale ASCII.

termflix is designed as a clean, production‑quality open‑source project with a focus on clarity, correctness, and maintainability.

### Features

- **Multiple sources**: local files, YouTube URLs (via `yt-dlp`), or stdin (`-`)
- **FFmpeg pipeline**: uses `ffmpeg` for decoding and `ffprobe` for metadata
- **Render modes**:
  - **blocks**: truecolor half‑block renderer (default)
  - **braille**: dense braille renderer for higher spatial detail
  - **ascii**: grayscale ASCII ramp
- **Scaling**:
  - auto‑fit to terminal size
  - fit/fill toggle
  - optional `--width` / `--height`
- **Playback controls**:
  - **space**: pause/resume
  - **q / ctrl+c**: quit
  - **← / →**: seek backward/forward 5 seconds
  - **j / l**: seek backward/forward 10 seconds
  - **r**: cycle render mode
  - **f**: toggle fit/fill
  - **m**: mute
  - **?**: toggle help overlay
- **Status bar**:
  - title / filename
  - elapsed / total duration
  - render mode
  - FPS
  - paused / muted state
- **Optional audio**:
  - `--audio=mpv`
  - `--audio=ffplay`
  - `--mute`

### Requirements

- **Go**: 1.22+
- **External binaries**:
  - required:
    - `ffmpeg`
    - `ffprobe`
  - required for YouTube URLs:
    - `yt-dlp`
  - optional audio:
    - `mpv`
    - `ffplay`

termflix checks for required binaries at startup and prints clear error messages if they are missing.

### Installation

Clone the repository and build:

```bash
git clone https://github.com/yourname/termflix.git
cd termflix
make build
```

This produces `bin/termflix`.

You can also install directly with Go:

```bash
go install ./cmd/termflix
```

### Usage

Basic examples:

```bash
termflix video.mp4
termflix https://www.youtube.com/watch?v=abc123
termflix movie.mkv --mode braille
termflix movie.mp4 --fps 15
termflix movie.mp4 --audio mpv
termflix movie.mp4 --mute
cat video.mp4 | termflix -
```

Useful flags:

- `--mode` (`blocks|braille|ascii`)
- `--fps` (default: 15)
- `--mute`
- `--audio` (`none|mpv|ffplay`)
- `--width` / `--height`
- `--no-ui` (render raw frames only; minimal TUI)
- `--fit` / `--fill` (mutually exclusive)

### Renderer modes

- **blocks**: Uses upper/lower half block characters with 24‑bit ANSI colors for two vertical pixels per cell.
- **braille**: Maps small pixel blocks into braille patterns for higher detail. Uses foreground color only.
- **ascii**: Converts luminance to a compact ASCII ramp for simple, readable output in limited environments.

### Keyboard controls

- **space** – pause/resume
- **q**, **ctrl+c** – quit
- **← / →** – seek −5s / +5s
- **j / l** – seek −10s / +10s
- **r** – cycle render mode (blocks → braille → ascii)
- **f** – toggle fit/fill scaling
- **m** – toggle mute
- **?** – toggle help overlay

### Architecture overview

The codebase is structured into focused internal packages:

- `cmd/termflix` – CLI entrypoint
- `internal/app` – high‑level orchestration and wiring
- `internal/source` – input detection and YouTube resolution via `yt-dlp`
- `internal/media` – metadata probing via `ffprobe`
- `internal/decoder` – ffmpeg spawning and raw RGB frame decoding
- `internal/render` – frame scaling and renderers (blocks, braille, ASCII)
- `internal/player` – playback state, timing, and controls
- `internal/audio` – subprocess‑based audio playback via `mpv` / `ffplay`
- `internal/tui` – Bubble Tea TUI, key bindings, and status bar
- `internal/term` – terminal setup / teardown
- `internal/util` – CLI parsing and dependency checks

Each package has a clear responsibility and minimal surface area, making it straightforward to extend the player with new features (e.g., additional renderers or input sources).

### Development

- **Build**: `make build`
- **Run**: `make run ARGS='video.mp4'`
- **Test**: `make test`
- **Format**: `make fmt`
- **Lint (go vet)**: `make lint`

Tests focus on pure logic (source detection, renderer logic, scaling) and avoid brittle full integration tests around `ffmpeg` and `yt-dlp`.

### Screenshots

You can add screenshots or terminal recordings here after running termflix:

- blocks mode: `docs/screenshots/blocks.png`
- braille mode: `docs/screenshots/braille.png`
- ascii mode: `docs/screenshots/ascii.png`

### License

termflix is released under the **MIT License**. See `LICENSE` for details.

