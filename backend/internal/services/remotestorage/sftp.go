package remotestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPConfig is the non-secret configuration for an sftp-type
// RemoteStorageTarget. HostKeyFingerprint (SHA256:...) is the discovered
// and user-confirmed public identity of the remote host — not itself
// sensitive (it's analogous to an SSH known_hosts entry) — so it lives
// alongside the rest of the non-secret config rather than in the encrypted
// secrets blob.
type SFTPConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Path               string `json:"path"`
	Username           string `json:"username"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
}

// SFTPSecrets is the secret credential material for an sftp-type target:
// either Password or PrivateKeyPEM (+ optional Passphrase for an encrypted
// private key).
type SFTPSecrets struct {
	Password      string `json:"password"`
	PrivateKeyPEM string `json:"private_key_pem"`
	Passphrase    string `json:"passphrase"`
}

// sftpConnectionTestFile is the marker file written/deleted by Test.
const sftpConnectionTestFile = "charon-connection-test"

func (cfg SFTPConfig) address() string {
	port := cfg.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(cfg.Host, strconv.Itoa(port))
}

// fingerprintPinnedHostKeyCallback is the SOLE host-key verification
// strategy used anywhere in this package for a "verified" connection (spec
// §3.7): it rejects any offered host key whose SHA-256 fingerprint doesn't
// exactly match pinnedFingerprint. There is no code path in this package
// that unconditionally trusts whatever key a server happens to present.
func fingerprintPinnedHostKeyCallback(pinnedFingerprint string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		actual := ssh.FingerprintSHA256(key)
		if actual != pinnedFingerprint {
			return fmt.Errorf("sftp: host key fingerprint mismatch: expected %s, got %s", pinnedFingerprint, actual)
		}
		return nil
	}
}

func sftpAuthMethods(secrets SFTPSecrets) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if strings.TrimSpace(secrets.PrivateKeyPEM) != "" {
		var signer ssh.Signer
		var err error
		if secrets.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(secrets.PrivateKeyPEM), []byte(secrets.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(secrets.PrivateKeyPEM))
		}
		if err != nil {
			return nil, fmt.Errorf("sftp: parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if secrets.Password != "" {
		methods = append(methods, ssh.Password(secrets.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("sftp: no credentials configured (need password or private_key_pem)")
	}

	return methods, nil
}

// sftpUploader implements Uploader for a verified sftp-type target — one
// whose HostKeyFingerprint has already been discovered (DiscoverSFTPHostKey
// below) and confirmed by the user (spec §3.7). Every operation dials fresh
// rather than keeping a long-lived connection: backup uploads are
// infrequent enough that connection setup cost is immaterial, and a
// stateless uploader is simpler to reason about and test.
type sftpUploader struct {
	cfg     SFTPConfig
	secrets SFTPSecrets
}

// newSFTPUploader constructs the Uploader for an sftp-type target.
func newSFTPUploader(cfg SFTPConfig, secrets SFTPSecrets) (Uploader, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("sftp: host is required")
	}
	if strings.TrimSpace(cfg.HostKeyFingerprint) == "" {
		return nil, fmt.Errorf("sftp: host key fingerprint must be discovered and confirmed before use")
	}
	if err := ssrfValidateHost(cfg.Host); err != nil {
		return nil, fmt.Errorf("sftp host failed SSRF validation: %w", err)
	}
	return &sftpUploader{cfg: cfg, secrets: secrets}, nil
}

// dial opens a verified (fingerprint-pinned) SFTP session.
func (u *sftpUploader) dial(ctx context.Context) (*sftp.Client, func(), error) {
	authMethods, err := sftpAuthMethods(u.secrets)
	if err != nil {
		return nil, nil, err
	}

	address := u.cfg.address()

	config := &ssh.ClientConfig{
		User:            u.cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: fingerprintPinnedHostKeyCallback(u.cfg.HostKeyFingerprint),
		Timeout:         15 * time.Second,
	}

	conn, err := dialContext(ctx, "tcp", address, 15*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("sftp: dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("sftp: handshake/authentication: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("sftp: open sftp session: %w", err)
	}

	closeFn := func() {
		_ = sftpClient.Close()
		_ = client.Close()
	}

	return sftpClient, closeFn, nil
}

func (u *sftpUploader) remotePath(name string) string {
	if u.cfg.Path == "" {
		return name
	}
	return path.Join(u.cfg.Path, name)
}

func (u *sftpUploader) Upload(ctx context.Context, localPath, remoteKey string) error {
	client, closeFn, err := u.dial(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	remoteFile := u.remotePath(remoteKey)
	if mkdirErr := client.MkdirAll(path.Dir(remoteFile)); mkdirErr != nil {
		return fmt.Errorf("sftp: create remote directory: %w", mkdirErr)
	}

	src, err := os.Open(localPath) // #nosec G304 -- localPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("sftp: open local file: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := client.Create(remoteFile)
	if err != nil {
		return fmt.Errorf("sftp: create remote file: %w", err)
	}
	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("sftp: write remote file: %w", err)
	}
	return nil
}

func (u *sftpUploader) Delete(ctx context.Context, remoteKey string) error {
	client, closeFn, err := u.dial(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	if err := client.Remove(u.remotePath(remoteKey)); err != nil {
		return fmt.Errorf("sftp: remove remote file: %w", err)
	}
	return nil
}

func (u *sftpUploader) List(ctx context.Context, prefix string) ([]RemoteObject, error) {
	client, closeFn, err := u.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	dir := u.cfg.Path
	if dir == "" {
		dir = "."
	}

	entries, err := client.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sftp: list remote directory: %w", err)
	}

	var objects []RemoteObject
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if prefix != "" && !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		objects = append(objects, RemoteObject{
			Key:          entry.Name(),
			Size:         entry.Size(),
			LastModified: entry.ModTime(),
		})
	}
	return objects, nil
}

func (u *sftpUploader) Test(ctx context.Context) error {
	client, closeFn, err := u.dial(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	dir := u.cfg.Path
	if dir == "" {
		dir = "."
	}
	if mkdirErr := client.MkdirAll(dir); mkdirErr != nil {
		return fmt.Errorf("sftp: create/access remote directory: %w", mkdirErr)
	}

	testPath := path.Join(dir, sftpConnectionTestFile)
	f, err := client.Create(testPath)
	if err != nil {
		return fmt.Errorf("sftp: write connection test marker: %w", err)
	}
	if _, err := f.Write([]byte("charon connection test")); err != nil {
		_ = f.Close()
		return fmt.Errorf("sftp: write connection test marker: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("sftp: close connection test marker: %w", err)
	}
	if err := client.Remove(testPath); err != nil {
		return fmt.Errorf("sftp: remove connection test marker: %w", err)
	}
	return nil
}

// DiscoverSFTPHostKey dials cfg.Host and returns the SHA-256 fingerprint of
// whatever host key the server offers, WITHOUT EVER attempting
// authentication (spec §3.7 discovery path — the never-merged counterpart
// of the verified path above).
//
// In golang.org/x/crypto/ssh, the host-key callback runs during transport
// key exchange, which completes before any authentication method is
// attempted (ssh.NewClientConn sequences KEX/host-key verification ahead of
// client authentication). The callback below unconditionally returns a
// non-nil error, which aborts the handshake at that point — so no password
// or private key is ever sent to an unpinned/unverified host. Auth is left
// empty for the same reason, defense in depth on top of the abort.
func DiscoverSFTPHostKey(cfg SFTPConfig) (string, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return "", fmt.Errorf("sftp: host is required")
	}
	if err := ssrfValidateHost(cfg.Host); err != nil {
		return "", fmt.Errorf("sftp host failed SSRF validation: %w", err)
	}

	var discovered string
	abort := errors.New("charon: aborting handshake before authentication (host key discovery)")

	config := &ssh.ClientConfig{
		User: "charon-discovery",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			discovered = ssh.FingerprintSHA256(key)
			return abort
		},
		Timeout: 15 * time.Second,
	}

	address := cfg.address()
	conn, err := dialContext(context.Background(), "tcp", address, 15*time.Second)
	if err != nil {
		return "", fmt.Errorf("sftp: dial for host key discovery: %w", err)
	}

	sshConn, _, _, handshakeErr := ssh.NewClientConn(conn, address, config)
	if sshConn != nil {
		_ = sshConn.Close()
	} else {
		_ = conn.Close()
	}

	if discovered == "" {
		return "", fmt.Errorf("sftp: host key was never offered during discovery: %w", handshakeErr)
	}

	return discovered, nil
}
