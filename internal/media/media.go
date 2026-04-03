package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/huntiezz/termflix/internal/source"
)

// Info holds parsed media metadata.
type Info struct {
	Duration  time.Duration
	Width     int
	Height    int
	FPS       float64
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
		AvgFrameRate string `json:"avg_frame_rate"`
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
	var fps float64
	for _, s := range data.Streams {
		if s.CodecType == "video" {
			width = s.Width
			height = s.Height
			fps = parseFFProbeRate(s.AvgFrameRate)
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
		FPS:       fps,
		Title:     title,
		StreamURL: src.PlayURL,
	}, nil
}

func parseFFProbeRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) != 2 {
			return 0
		}
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

