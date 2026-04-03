package audio

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/example/termflix/internal/util"
)

// Options configure the audio controller.
type Options struct {
	Engine   util.AudioEngine
	Muted    bool
	FFPlay   string
	MPV      string
	InputURL string
	SeekFunc func(offset time.Duration) error
}

// Controller is a subprocess-based audio manager.
type Controller struct {
	opts Options

	mu     sync.Mutex
	cmd    *exec.Cmd
	muted  bool
	playing bool
}

func NewController(opts Options) *Controller {
	return &Controller{
		opts:  opts,
		muted: opts.Muted,
	}
}

func (c *Controller) Play(offset time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.opts.Engine == util.AudioEngineNone || c.muted {
		return nil
	}

	if c.cmd != nil && c.playing {
		return nil
	}

	switch c.opts.Engine {
	case util.AudioEngineFFPlay:
		return c.startFFPlay(offset)
	case util.AudioEngineMPV:
		return c.startMPV(offset)
	default:
		return nil
	}
}

func (c *Controller) startFFPlay(offset time.Duration) error {
	args := []string{
		"-nodisp",
		"-autoexit",
	}
	if offset > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", offset.Seconds()))
	}
	args = append(args, c.opts.InputURL)
	if c.muted {
		args = append(args, "-an")
	}
	cmd := exec.Command(c.opts.FFPlay, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.playing = true
	go func() {
		_ = cmd.Wait()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.playing = false
	}()
	return nil
}

func (c *Controller) startMPV(offset time.Duration) error {
	args := []string{"--no-video", "--force-window=no", "--keep-open=no"}
	if offset > 0 {
		args = append(args, fmt.Sprintf("--start=%0.3f", offset.Seconds()))
	}
	if c.muted {
		args = append(args, "--mute=yes")
	}
	args = append(args, c.opts.InputURL)
	cmd := exec.Command(c.opts.MPV, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.playing = true
	go func() {
		_ = cmd.Wait()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.playing = false
	}()
	return nil
}

func (c *Controller) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

func (c *Controller) Mute(muted bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.muted = muted
	// MVP: we don't reconfigure running subprocess; callers may restart playback if needed.
	return nil
}

func (c *Controller) IsMuted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.muted
}

