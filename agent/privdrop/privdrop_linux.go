//go:build linux

package privdrop

import (
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

// realCredentials forwards to the kernel. Go's syscall.Setuid, Setgid and
// Setgroups apply to all OS threads of the process on Linux (since Go 1.16),
// so the drop cannot be left in effect on only one thread.
type realCredentials struct{}

func (realCredentials) Getuid() int                { return syscall.Getuid() }
func (realCredentials) Geteuid() int               { return syscall.Geteuid() }
func (realCredentials) Getgid() int                { return syscall.Getgid() }
func (realCredentials) Getegid() int               { return syscall.Getegid() }
func (realCredentials) Setgroups(gids []int) error { return syscall.Setgroups(gids) }
func (realCredentials) Setgid(gid int) error       { return syscall.Setgid(gid) }
func (realCredentials) Setuid(uid int) error       { return syscall.Setuid(uid) }

// DropPrivileges drops root to the PUID/PGID identity described in the package
// documentation. It must be called before any network or configuration work.
func DropPrivileges(log *logrus.Logger) error {
	return apply(os.Getenv, realCredentials{}, log)
}
