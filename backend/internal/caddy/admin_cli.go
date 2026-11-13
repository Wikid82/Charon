package caddy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	charonconfig "github.com/Wikid82/charon/backend/internal/config"
)

// adminCommandTimeout bounds one `caddy-admin get` request.
const adminCommandTimeout = 10 * time.Second

// adminCommandUsage is printed for an unknown or malformed `caddy-admin` invocation.
const adminCommandUsage = "usage: charon caddy-admin listen | prepare | get <absolute-path>"

// RunAdminCommand implements `charon caddy-admin <command>`. It reads the admin API
// setting from the environment and returns the process exit status. The command
// runs before the server's logging is set up, so its stdout carries only its output.
//
//	listen          print the admin.listen value for the configured admin API
//	prepare         create the admin socket directory and remove a stale socket
//	get <path>      print the body of a GET request to the admin API
func RunAdminCommand(args []string, stdout, stderr io.Writer) int {
	cfg, err := charonconfig.Load()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	return runAdminCommand(cfg.CaddyAdminAPI, args, stdout, stderr)
}

// runAdminCommand dispatches one caddy-admin command against adminAPI.
func runAdminCommand(adminAPI string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, adminCommandUsage)
		return 2
	}
	switch args[0] {
	case "listen":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, adminCommandUsage)
			return 2
		}
		_, _ = fmt.Fprintln(stdout, charonconfig.CaddyAdminListen(adminAPI))
		return 0
	case "prepare":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, adminCommandUsage)
			return 2
		}
		socketPath, ok := charonconfig.CaddyAdminSocketPath(adminAPI)
		if !ok {
			// A TCP admin listener has no socket to prepare.
			return 0
		}
		if err := PrepareAdminSocket(socketPath); err != nil {
			_, _ = fmt.Fprintf(stderr, "prepare admin socket: %v\n", err)
			return 1
		}
		return 0
	case "get":
		if len(args) != 2 || args[1] == "" || args[1][0] != '/' {
			_, _ = fmt.Fprintln(stderr, adminCommandUsage)
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), adminCommandTimeout)
		defer cancel()
		body, err := NewClient(adminAPI).Fetch(ctx, args[1])
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "admin request failed: %v\n", err)
			return 1
		}
		if _, err := stdout.Write(body); err != nil {
			_, _ = fmt.Fprintf(stderr, "write response: %v\n", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintln(stderr, adminCommandUsage)
		return 2
	}
}

// PrepareAdminSocket makes socketPath ready for Caddy to bind. It creates the
// directory with mode 0700 when it is missing. It removes a socket file that no
// process accepts on, such as one left by a process that exited without cleaning
// up. It refuses to touch a live socket or any file that is not a socket.
func PrepareAdminSocket(socketPath string) error {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	info, err := os.Lstat(socketPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect socket path: %w", err)
	}
	if info.Mode()&fs.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace %s: it is not a socket", socketPath)
	}

	conn, dialErr := net.DialTimeout("unix", socketPath, time.Second)
	if dialErr == nil {
		_ = conn.Close()
		return fmt.Errorf("socket %s is in use by another process", socketPath)
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("check socket %s: %w", socketPath, dialErr)
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}
