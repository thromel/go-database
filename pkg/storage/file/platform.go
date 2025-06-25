package file

import (
	"runtime"
)

// getDirectIOFlag returns the platform-specific flag for direct I/O.
func getDirectIOFlag() int {
	switch runtime.GOOS {
	case "linux":
		// O_DIRECT on Linux - bypass page cache
		return 0x4000 // O_DIRECT
	case "darwin":
		// macOS doesn't have O_DIRECT, use F_NOCACHE instead
		// This is handled in a separate function after opening
		return 0
	case "windows":
		// Windows FILE_FLAG_NO_BUFFERING equivalent
		// This would be handled differently on Windows
		return 0
	default:
		// Fallback for other platforms
		return 0
	}
}
