//go:build unix

package services

import (
	"io/fs"
	"syscall"
)

// openNoFollowFlags make the open fail on a symbolic link instead of following it, and keep a FIFO from blocking.
// The close-on-exec flag is already set by os.OpenFile.
const openNoFollowFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

// ownedBy reports whether the file or folder described by info belongs to uid.
func ownedBy(info fs.FileInfo, uid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == uid
}
