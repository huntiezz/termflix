package decoder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Frame represents a raw RGB frame.
type Frame struct {
	Width  int
	Height int
	Data   []byte // len = Width * Height * 3
}

// Config controls how ffmpeg outputs raw frames.
type Config struct {
	FFMPEGPath string
	InputURL   string
	Width      int
	Height     int
	FPSCap     int
	StartAt    float64 // seconds
	Realtime   bool    // throttle read to realtime
}

// Decoder spawns ffmpeg and exposes frames over a channel.
type Decoder struct {
	cfg Config
	cmd *exec.Cmd
	r   io.ReadCloser
}

// New creates a Decoder but does not start ffmpeg yet.
func New(cfg Config) *Decoder {
	return &Decoder{cfg: cfg}
}

// Start launches ffmpeg and returns a channel of frames and an error channel.
func (d *Decoder) Start(ctx context.Context) (<-chan Frame, <-chan error, error) {
	if d.cfg.FFMPEGPath == "" {
		d.cfg.FFMPEGPath = "ffmpeg"
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}
	if d.cfg.Realtime {
		args = append(args, "-re")
	}
	if d.cfg.StartAt > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", d.cfg.StartAt))
	}
	args = append(args,
		"-i", d.cfg.InputURL,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
	)

	var filters []string
	if d.cfg.Width > 0 && d.cfg.Height > 0 {
		filters = append(filters, fmt.Sprintf("scale=%d:%d", d.cfg.Width, d.cfg.Height))
	}
	if d.cfg.FPSCap > 0 {
		// Use filter-based FPS cap to avoid altering timestamps with output -r.
		filters = append(filters, fmt.Sprintf("fps=%d", d.cfg.FPSCap))
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}
	args = append(args, "pipe:1")

	cmd := exec.CommandContext(ctx, d.cfg.FFMPEGPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg stdout: %w", err)
	}
	d.cmd = cmd
	d.r = stdout

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	frameSize := d.cfg.Width * d.cfg.Height * 3
	if frameSize <= 0 {
		return nil, nil, fmt.Errorf("invalid frame size %dx%d", d.cfg.Width, d.cfg.Height)
	}

	frames := make(chan Frame, 2)
	errCh := make(chan error, 1)

	go func() {
		defer close(frames)
		defer close(errCh)

		reader := bufio.NewReader(stdout)
		buf := make([]byte, frameSize)

		for {
			_, err := io.ReadFull(reader, buf)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}
				errCh <- err
				return
			}

			cp := make([]byte, len(buf))
			copy(cp, buf)
			select {
			case frames <- Frame{Width: d.cfg.Width, Height: d.cfg.Height, Data: cp}:
			case <-ctx.Done():
				return
			default:
				// Drop frames when rendering can't keep up, to avoid backpressure
				// that would stall ffmpeg and cause very low FPS.
			}
		}
	}()

	go func() {
		_ = cmd.Wait()
	}()

	return frames, errCh, nil
}

// Stop terminates ffmpeg if running.
func (d *Decoder) Stop() {
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
	}
}

