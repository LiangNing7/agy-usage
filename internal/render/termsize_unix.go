//go:build linux || darwin

package render

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// DetectTerminalWidth returns the current terminal column width or fallback if unavailable.
func DetectTerminalWidth(fallback int) int {
	if envCols := os.Getenv("COLUMNS"); envCols != "" {
		if n, err := strconv.Atoi(envCols); err == nil && n > 0 {
			return n
		}
	}
	var req uintptr = 0x5413 // TIOCGWINSZ on Linux
	if runtime.GOOS == "darwin" {
		req = 0x40087468 // TIOCGWINSZ on macOS
	}
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stderr.Fd(), os.Stdin.Fd()} {
		var ws winsize
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(&ws)))
		if errno == 0 && ws.Col > 0 {
			return int(ws.Col)
		}
	}
	return fallback
}
