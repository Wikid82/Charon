// Package privdrop lets the Orthrus agent start as root and then permanently
// drop to an unprivileged uid/gid before doing any network or config work.
//
// The agent ships in a FROM scratch image with no shell or entrypoint script,
// so the linuxserver.io-style PUID/PGID environment variables are interpreted
// here, in the binary itself.
//
// Environment variables:
//
//	PUID                 target uid. Unset or empty means 65532.
//	PGID                 target gid. Unset or empty means 65532.
//	ORTHRUS_ALLOW_ROOT   set to exactly "true" to permit running as root when
//	                     PUID or PGID is explicitly 0.
//
// When the process is not root, PUID/PGID are ignored and nothing is changed.
package privdrop

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	envPUID      = "PUID"
	envPGID      = "PGID"
	envAllowRoot = "ORTHRUS_ALLOW_ROOT"

	// defaultID is the "nobody" style uid/gid used when PUID/PGID are unset.
	defaultID = 65532

	// maxID is the largest valid uid/gid. 0xFFFFFFFF is reserved by the kernel
	// to mean "leave unchanged" for the set*id family of syscalls.
	maxID = 4294967294
)

// credentials abstracts the process identity syscalls so the drop logic can be
// exercised in unit tests without root privileges.
type credentials interface {
	Getuid() int
	Geteuid() int
	Getgid() int
	Getegid() int
	Setgroups(gids []int) error
	Setgid(gid int) error
	Setuid(uid int) error
}

// apply performs the privilege drop using getenv for configuration and creds
// for the identity syscalls. It is a no-op for non-root processes.
func apply(getenv func(string) string, creds credentials, log *logrus.Logger) error {
	if creds.Geteuid() != 0 {
		warnIfIgnoredIDs(getenv, creds, log)
		return nil
	}

	uid, err := parseID(envPUID, getenv(envPUID))
	if err != nil {
		return err
	}
	gid, err := parseID(envPGID, getenv(envPGID))
	if err != nil {
		return err
	}

	if uid == 0 || gid == 0 {
		if getenv(envAllowRoot) != "true" {
			return fmt.Errorf("refusing to run as root (PUID=%d PGID=%d); set a non-zero PUID/PGID or %s=true to override", uid, gid, envAllowRoot)
		}
		log.WithFields(logrus.Fields{"puid": uid, "pgid": gid}).
			Warn("orthrus: running as root because " + envAllowRoot + "=true; this is insecure")
		return nil
	}

	if err := dropTo(creds, uid, gid); err != nil {
		return err
	}

	log.WithFields(logrus.Fields{"uid": uid, "gid": gid}).Info("orthrus: dropped root privileges")
	return nil
}

// dropTo clears supplementary groups, sets gid then uid, and verifies that the
// drop is irreversible and took effect.
func dropTo(creds credentials, uid, gid int) error {
	if err := creds.Setgroups([]int{}); err != nil {
		return fmt.Errorf("privdrop: clear supplementary groups: %w", err)
	}
	if err := creds.Setgid(gid); err != nil {
		return fmt.Errorf("privdrop: setgid(%d): %w", gid, err)
	}
	if err := creds.Setuid(uid); err != nil {
		return fmt.Errorf("privdrop: setuid(%d): %w", uid, err)
	}

	if creds.Getuid() != uid || creds.Geteuid() != uid || creds.Getgid() != gid || creds.Getegid() != gid {
		return fmt.Errorf("privdrop: credential verification failed: uid=%d euid=%d gid=%d egid=%d (want %d/%d)",
			creds.Getuid(), creds.Geteuid(), creds.Getgid(), creds.Getegid(), uid, gid)
	}

	if err := creds.Setuid(0); err == nil {
		return errors.New("privdrop: able to regain root after drop; refusing to continue")
	}
	return nil
}

// warnIfIgnoredIDs logs when PUID/PGID are set to values that differ from the
// current non-root identity, since they have no effect in that case.
func warnIfIgnoredIDs(getenv func(string) string, creds credentials, log *logrus.Logger) {
	puid := strings.TrimSpace(getenv(envPUID))
	pgid := strings.TrimSpace(getenv(envPGID))
	uidMismatch := puid != "" && puid != strconv.Itoa(creds.Geteuid())
	gidMismatch := pgid != "" && pgid != strconv.Itoa(creds.Getegid())
	if !uidMismatch && !gidMismatch {
		return
	}
	log.WithFields(logrus.Fields{
		"puid": puid,
		"pgid": pgid,
		"uid":  creds.Geteuid(),
		"gid":  creds.Getegid(),
	}).Warn("orthrus: process is not root; PUID/PGID are ignored")
}

// parseID parses a PUID/PGID value. An empty (or whitespace-only) value yields
// defaultID. Non-numeric, negative, or out-of-range values are errors.
func parseID(name, raw string) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultID, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("privdrop: invalid %s %q: must be a numeric id", name, raw)
	}
	if n < 0 || n > maxID {
		return 0, fmt.Errorf("privdrop: invalid %s %q: must be between 0 and %d", name, raw, maxID)
	}
	return int(n), nil
}
