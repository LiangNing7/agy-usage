//go:build !linux && !darwin

package render

import (
	"os"
	"strconv"
)

// DetectTerminalWidth returns the current terminal column width or fallback if unavailable.
func DetectTerminalWidth(fallback int) int {
	if envCols := os.Getenv("COLUMNS"); envCols != "" {
		if n, err := strconv.Atoi(envCols); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
