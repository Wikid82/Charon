//go:build !linux

package crowdsec

import "os"

// cacheOpenFlags opens cache files read-only; the file type is checked on the descriptor afterwards.
const cacheOpenFlags = os.O_RDONLY
