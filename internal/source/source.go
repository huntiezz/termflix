package source

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Type represents the kind of media source.
type Type int

const (
	TypeFile Type = iota + 1
	TypeYouTube
	TypeStdin
)

// Source describes a resolved media source, including a playable URL.
type Source struct {
	Type      Type
	Input     string // original input
	PlayURL   string // direct media URL or file path (video or combined)
	AudioURL  string // optional separate audio URL; falls back to PlayURL
	Title     string
	FromStdin bool
}

// Detect inspects the input and resolves it to a Source.
func Detect(ctx context.Context, input, ytdlpPath string) (Source, error) {
	if input == "-" {
		return Source{
			Type:      TypeStdin,
			Input:     input,
			PlayURL:   "pipe:0",
			AudioURL:  "pipe:0",
			FromStdin: true,
		}, nil
	}

	if isURL(input) {
		if isYouTubeURL(input) {
			return resolveYouTube(ctx, input, ytdlpPath)
		}
		// For non-YouTube URLs we assume it's already a direct media URL.
		return Source{
			Type:    TypeFile,
			Input:   input,
			PlayURL: input,
			AudioURL: input,
		}, nil
	}

	// Treat as local file path.
	if _, err := os.Stat(input); err != nil {
		return Source{}, fmt.Errorf("input %q not found or unreadable: %w", input, err)
	}
	return Source{
		Type:    TypeFile,
		Input:   input,
		PlayURL: input,
		AudioURL: input,
	}, nil
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func isYouTubeURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be")
}

// resolveYouTube runs yt-dlp to obtain a playable URL and metadata.
func resolveYouTube(ctx context.Context, rawURL, ytdlpPath string) (Source, error) {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	// Use "-g" to print direct media URL(s). It's more stable than parsing -J JSON
	// across extractor variations.
	//
	// Output is typically:
	//  - title line
	//  - 1+ URL lines
	//
	// We select the first URL line; ffmpeg will handle many direct media URLs.
	cmd := exec.CommandContext(ctx, ytdlpPath,
		// Prefer separate streams (video+audio) and fall back to a combined stream.
		"-f", "bestvideo+bestaudio/best",
		"--no-playlist",
		"--no-warnings",
		"--print", "title",
		"-g",
		rawURL,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return Source{}, fmt.Errorf("yt-dlp failed (is it installed and on PATH?): %s", strings.TrimSpace(msg))
	}

	lines := splitNonEmptyLines(out.String())
	if len(lines) < 2 {
		return Source{}, fmt.Errorf("yt-dlp did not return expected output (title + url)")
	}
	title := lines[0]
	urls := make([]string, 0, len(lines)-1)
	for _, ln := range lines[1:] {
		if strings.HasPrefix(ln, "http://") || strings.HasPrefix(ln, "https://") {
			urls = append(urls, ln)
		}
	}
	if len(urls) == 0 {
		return Source{}, fmt.Errorf("yt-dlp did not return a playable URL")
	}
	playURL := urls[0]
	audioURL := playURL
	if len(urls) >= 2 {
		audioURL = urls[1]
	}

	return Source{
		Type:    TypeYouTube,
		Input:   rawURL,
		PlayURL: playURL,
		AudioURL: audioURL,
		Title:   title,
	}, nil
}

func splitNonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

