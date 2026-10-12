//go:build linux

package crowdsec

import (
	"os"
	"syscall"
)

// cacheOpenFlags opens cache files read-only without blocking, so a file of an unexpected type cannot
// stall the open; the type is checked on the descriptor afterwards.
const cacheOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK
