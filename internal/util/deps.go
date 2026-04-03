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
	_, err := ResolveBinary(cfg.FFMPEGPath, "ffmpeg")
	if err != nil {
		return dependencyError("ffmpeg", cfg.FFMPEGPath, err)
	}
	_, err = ResolveBinary(cfg.FFProbePath, "ffprobe")
	if err != nil {
		return dependencyError("ffprobe", cfg.FFProbePath, err)
	}
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
	if _, err := ResolveBinary(ytdlpPath, "yt-dlp"); err != nil {
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

	// "where.exe" often succeeds even when PATH updates are partial.
	if p, err := windowsWhere(tool); err == nil && p != "" {
		return p, nil
	}

	for _, c := range windowsCandidates(tool) {
		if fileExists(c) {
			return c, nil
		}
	}

	// Last resort: scan common winget package directory for the binary.
	if p, err := windowsWingetScan(tool); err == nil && p != "" {
		return p, nil
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

func windowsWhere(tool string) (string, error) {
	cmd := exec.Command("where.exe", tool)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Prefer real executables, not cmd scripts.
		if strings.HasSuffix(strings.ToLower(line), ".exe") && fileExists(line) {
			return line, nil
		}
	}
	return "", fmt.Errorf("where.exe did not return an .exe path")
}

func windowsWingetScan(tool string) (string, error) {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return "", fmt.Errorf("LOCALAPPDATA not set")
	}

	// Common winget location: %LOCALAPPDATA%\Microsoft\WinGet\Packages\...
	root := filepath.Join(la, "Microsoft", "WinGet", "Packages")
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("winget packages dir not found")
	}

	target := tool + ".exe"

	// Keep it bounded: only scan up to N entries to avoid hanging on huge directories.
	const maxDirs = 2500
	seen := 0

	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			seen++
			if seen > maxDirs {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), target) {
			found = path
			return errors.New("found") // sentinel to stop walk early
		}
		return nil
	})

	if found != "" && fileExists(found) {
		return found, nil
	}
	return "", fmt.Errorf("not found under %s", root)
}

