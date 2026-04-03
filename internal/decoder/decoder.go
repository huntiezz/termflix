package decoder

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
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
	FPS        int
	StartAt    float64 // seconds
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
	if d.cfg.StartAt > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", d.cfg.StartAt))
	}
	args = append(args,
		"-i", d.cfg.InputURL,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
	)
	if d.cfg.Width > 0 && d.cfg.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", d.cfg.Width, d.cfg.Height))
	}
	if d.cfg.FPS > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", d.cfg.FPS))
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

