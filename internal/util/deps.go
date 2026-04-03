package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckRequiredBinaries verifies presence of core external dependencies.
func CheckRequiredBinaries(cfg Config) error {
	ffmpeg, err := ResolveBinary(cfg.FFMPEGPath, "ffmpeg")
	if err != nil {
		return dependencyError("ffmpeg", cfg.FFMPEGPath, err)
	}
	ffprobe, err := ResolveBinary(cfg.FFProbePath, "ffprobe")
	if err != nil {
		return dependencyError("ffprobe", cfg.FFProbePath, err)
	}

	// Update cfg paths to resolved absolute binaries when possible.
	_ = ffmpeg
	_ = ffprobe
	return nil
}

func dependencyError(tool, configured string, err error) error {
	base := fmt.Sprintf("%s not found (looked for %q): %v", tool, configured, err)
	if runtime.GOOS != "windows" {
		return errors.New(base)
	}
	return fmt.Errorf("%s\n\nWindows install:\n- winget: winget install Gyan.FFmpeg\n- scoop: scoop install ffmpeg\n- choco: choco install ffmpeg\n\nDiagnostics:\n- where.exe %s\n- echo $env:Path\n", base, tool)
}

// CheckYouTubeBinary verifies yt-dlp exists when needed.
func CheckYouTubeBinary(ytdlpPath string) error {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}
	if _, err := exec.LookPath(ytdlpPath); err != nil {
		msg := "yt-dlp not found (looked for %q). Install yt-dlp to play YouTube URLs.\n\nWindows:\n- winget: winget install yt-dlp.yt-dlp\n- scoop: scoop install yt-dlp\n\nmacOS: brew install yt-dlp\nLinux: use your package manager or pipx"
		if runtime.GOOS == "windows" {
			return fmt.Errorf(msg, ytdlpPath)
		}
		return fmt.Errorf(msg, ytdlpPath)
	}
	return nil
}

// ResolveBinary finds an executable, supporting common Windows install locations.
// If configured is an absolute/relative path to an existing file, it is used as-is.
// Otherwise it attempts PATH lookup and then OS-specific fallback locations.
func ResolveBinary(configured, tool string) (string, error) {
	if configured == "" {
		configured = tool
	}

	// If they passed an explicit path, honor it.
	if strings.ContainsAny(configured, `\/`) || filepath.IsAbs(configured) {
		if p, err := filepath.Abs(configured); err == nil {
			if fileExists(p) {
				return p, nil
			}
		}
		if fileExists(configured) {
			return configured, nil
		}
		// Continue to lookup in case they passed just "ffmpeg.exe".
	}

	if p, err := exec.LookPath(configured); err == nil {
		return p, nil
	}
	if configured != tool {
		if p, err := exec.LookPath(tool); err == nil {
			return p, nil
		}
	}

	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("not found on PATH")
	}

	for _, c := range windowsCandidates(tool) {
		if fileExists(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("not found on PATH and not found in common locations")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func windowsCandidates(tool string) []string {
	exe := tool + ".exe"
	var out []string

	// 1) Next to the running binary (portable distribution pattern).
	if self, err := os.Executable(); err == nil {
		dir := filepath.Dir(self)
		out = append(out,
			filepath.Join(dir, exe),
			filepath.Join(dir, "ffmpeg", "bin", exe),
		)
	}

	// 2) Scoop shims.
	if u, ok := os.LookupEnv("USERPROFILE"); ok {
		out = append(out,
			filepath.Join(u, "scoop", "shims", exe),
			filepath.Join(u, "scoop", "apps", "ffmpeg", "current", "bin", exe),
		)
	}

	// 3) Common manual installs / Program Files.
	pf := os.Getenv("ProgramFiles")
	pfx := os.Getenv("ProgramFiles(x86)")
	for _, root := range []string{pf, pfx} {
		if root == "" {
			continue
		}
		out = append(out,
			filepath.Join(root, "ffmpeg", "bin", exe),
			filepath.Join(root, "FFmpeg", "bin", exe),
			filepath.Join(root, "Gyan", "FFmpeg", "bin", exe),
			filepath.Join(root, "Gyan", "FFmpeg", "ffmpeg", "bin", exe),
		)
	}

	return out
}

