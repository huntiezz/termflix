package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/example/termflix/internal/source"
)

// Info holds parsed media metadata.
type Info struct {
	Duration  time.Duration
	Width     int
	Height    int
	Title     string
	StreamURL string
}

func (i Info) DisplayTitle() string {
	if i.Title != "" {
		return i.Title
	}
	return "Unnamed"
}

// ffprobeJSON models the subset of ffprobe fields we care about.
type ffprobeJSON struct {
	Format struct {
		Duration string `json:"duration"`
		Tags     struct {
			Title string `json:"title"`
		} `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

// Probe queries ffprobe for metadata about the given source.
func Probe(ctx context.Context, ffprobePath string, src source.Source) (Info, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		src.PlayURL,
	}

	cmd := exec.CommandContext(ctx, ffprobePath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return Info{}, fmt.Errorf("ffprobe failed: %s", msg)
	}

	var data ffprobeJSON
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return Info{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var (
		duration time.Duration
	)
	if data.Format.Duration != "" {
		seconds, err := time.ParseDuration(data.Format.Duration + "s")
		if err == nil {
			duration = seconds
		}
	}

	var width, height int
	for _, s := range data.Streams {
		if s.CodecType == "video" {
			width = s.Width
			height = s.Height
			break
		}
	}

	title := data.Format.Tags.Title
	if title == "" {
		title = src.Title
	}

	return Info{
		Duration:  duration,
		Width:     width,
		Height:    height,
		Title:     title,
		StreamURL: src.PlayURL,
	}, nil
}

