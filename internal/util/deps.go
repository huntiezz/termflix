package util

import (
	"fmt"
	"os/exec"
)

// CheckRequiredBinaries verifies presence of core external dependencies.
func CheckRequiredBinaries(cfg Config) error {
	if _, err := exec.LookPath(cfg.FFMPEGPath); err != nil {
		return fmt.Errorf("ffmpeg not found (looked for %q). Install ffmpeg and ensure it is on PATH", cfg.FFMPEGPath)
	}
	if _, err := exec.LookPath(cfg.FFProbePath); err != nil {
		return fmt.Errorf("ffprobe not found (looked for %q). Install ffprobe and ensure it is on PATH", cfg.FFProbePath)
	}
	return nil
}

