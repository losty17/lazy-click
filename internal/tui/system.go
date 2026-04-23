package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func copyToClipboard(ctx context.Context, value string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "windows":
		candidates = [][]string{{"clip"}}
	default:
		candidates = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	}

	var lastErr error
	for _, candidate := range candidates {
		// Check if the command exists in the PATH
		_, err := exec.LookPath(candidate[0])
		if err != nil {
			lastErr = fmt.Errorf("clipboard command %s not found: %w", candidate[0], err)
			continue // Try next candidate
		}

		cmd := exec.CommandContext(ctx, candidate[0], candidate[1:]...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("clipboard command %s failed: %w", candidate[0], err)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no clipboard command available")
	}
	return lastErr
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
