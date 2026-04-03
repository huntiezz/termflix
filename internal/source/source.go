package source

import (
	"bytes"
	"context"
	"encoding/json"
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
	PlayURL   string // direct media URL or file path
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

type ytDLPResult struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// resolveYouTube runs yt-dlp to obtain a playable URL and metadata.
func resolveYouTube(ctx context.Context, rawURL, ytdlpPath string) (Source, error) {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	cmd := exec.CommandContext(ctx, ytdlpPath, "-J", "-f", "bv*+ba/b", rawURL)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return Source{}, fmt.Errorf("yt-dlp failed: %s", strings.TrimSpace(msg))
	}

	var data ytDLPResult
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return Source{}, fmt.Errorf("parse yt-dlp output: %w", err)
	}
	if data.URL == "" {
		return Source{}, fmt.Errorf("yt-dlp did not return a playable URL")
	}

	title := data.Title
	if title == "" {
		title = rawURL
	}

	return Source{
		Type:    TypeYouTube,
		Input:   rawURL,
		PlayURL: data.URL,
		Title:   title,
	}, nil
}

