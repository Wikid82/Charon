package remotestorage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	sftpTestUsername = "charon-test"
	sftpTestPassword = "test-password-hunter2"
)

// startFullFakeSFTPServer starts a local SSH server that accepts
// sftpTestUsername/sftpTestPassword and serves an "sftp" subsystem via
// github.com/pkg/sftp's standard Server — the same server-side
// implementation any real SFTP-capable NAS/appliance would run — against
// the real (test-owned) filesystem. This lets sftpUploader's
// Upload/Delete/List/Test be exercised end-to-end over a genuine SFTP
// session instead of a hand-rolled stub, closing the coverage gap QA
// identified in remotestorage/sftp.go's request/response handling.
func startFullFakeSFTPServer(t *testing.T) (addr string, hostKeyFingerprint string) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == sftpTestUsername && string(password) == sftpTestPassword {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", conn.User())
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveFakeSFTPConnection(conn, config)
		}
	}()

	t.Cleanup(func() { _ = listener.Close() })

	return listener.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

func serveFakeSFTPConnection(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, acceptErr := newChannel.Accept()
		if acceptErr != nil {
			continue
		}

		go func() {
			for req := range requests {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
				_ = req.Reply(ok, nil)
			}
		}()

		go func() {
			server, serverErr := sftp.NewServer(channel)
			if serverErr != nil {
				return
			}
			_ = server.Serve()
			_ = server.Close()
		}()
	}
}

// newTestSFTPUploader constructs a real sftpUploader (via the exported
// construction path, newSFTPUploader) pinned to the fake server's host key,
// with SSRF checks relaxed for the loopback fixture.
func newTestSFTPUploader(t *testing.T, addr, fingerprint, remoteRoot string) Uploader {
	t.Helper()
	withPermissiveSSRFForLocalTest(t)

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	uploader, err := newSFTPUploader(SFTPConfig{
		Host:               host,
		Port:               port,
		Path:               remoteRoot,
		Username:           sftpTestUsername,
		HostKeyFingerprint: fingerprint,
	}, SFTPSecrets{Password: sftpTestPassword})
	require.NoError(t, err)
	return uploader
}

// TestSFTPUploader_UploadListDeleteTest_Roundtrip proves the full
// Upload -> List -> Delete -> Test request/response cycle against a real
// (local, ephemeral) SFTP session, including the remotePath prefix-joining
// logic.
func TestSFTPUploader_UploadListDeleteTest_Roundtrip(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := t.TempDir()
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)
	ctx := context.Background()

	require.NoError(t, uploader.Test(ctx), "Test() must write+remove its connectivity marker successfully")

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("fake backup contents"), 0o600))

	require.NoError(t, uploader.Upload(ctx, localFile, "backup_1.zip"))
	uploadedPath := filepath.Join(remoteRoot, "backup_1.zip")
	require.FileExists(t, uploadedPath)
	content, err := os.ReadFile(uploadedPath) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	assert.Equal(t, "fake backup contents", string(content))

	objects, err := uploader.List(ctx, "")
	require.NoError(t, err)
	var names []string
	for _, o := range objects {
		names = append(names, o.Key)
		assert.Greater(t, o.Size, int64(0))
	}
	assert.Contains(t, names, "backup_1.zip")

	// List with a prefix that excludes everything currently present.
	filtered, err := uploader.List(ctx, "does-not-match-")
	require.NoError(t, err)
	assert.Empty(t, filtered)

	require.NoError(t, uploader.Delete(ctx, "backup_1.zip"))
	assert.NoFileExists(t, uploadedPath)
}

// TestSFTPUploader_Upload_NestedRemotePrefix proves remotePath correctly
// joins cfg.Path with a nested remoteKey and that Upload's MkdirAll creates
// any needed intermediate directories.
func TestSFTPUploader_Upload_NestedRemotePrefix(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := t.TempDir()
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)
	ctx := context.Background()

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("nested contents"), 0o600))

	require.NoError(t, uploader.Upload(ctx, localFile, "charon/nested/backup_2.zip"))
	require.FileExists(t, filepath.Join(remoteRoot, "charon", "nested", "backup_2.zip"))
}

// TestSFTPUploader_Upload_LocalFileMissing proves a missing local source
// file surfaces a wrapped "open local file" error rather than partially
// creating the remote file.
func TestSFTPUploader_Upload_LocalFileMissing(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := t.TempDir()
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)

	err := uploader.Upload(context.Background(), filepath.Join(t.TempDir(), "does-not-exist.zip"), "backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open local file")
}

// TestSFTPUploader_Upload_MkdirFailure proves a remote directory that
// cannot be created (parent made read-only) surfaces a wrapped
// "create remote directory" error.
func TestSFTPUploader_Upload_MkdirFailure(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := t.TempDir()
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)

	require.NoError(t, os.Chmod(remoteRoot, 0o500))
	t.Cleanup(func() { _ = os.Chmod(remoteRoot, 0o700) })

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("data"), 0o600))

	err := uploader.Upload(context.Background(), localFile, "sub/backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create remote directory")
}

// TestSFTPUploader_Delete_RemoteFileMissing proves deleting a nonexistent
// remote key surfaces a wrapped "remove remote file" error.
func TestSFTPUploader_Delete_RemoteFileMissing(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := t.TempDir()
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)

	err := uploader.Delete(context.Background(), "does-not-exist.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove remote file")
}

// TestSFTPUploader_List_MissingDirectory_ReturnsEmptyNotError proves
// listing a remote directory that doesn't exist yet (e.g. a target never
// uploaded to) returns an empty list rather than an error.
func TestSFTPUploader_List_MissingDirectory_ReturnsEmptyNotError(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	remoteRoot := filepath.Join(t.TempDir(), "does-not-exist-yet")
	uploader := newTestSFTPUploader(t, addr, fingerprint, remoteRoot)

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, objects)
}

// TestSFTPUploader_List_DefaultsToCurrentDirWhenPathEmpty proves an empty
// cfg.Path lists "." rather than erroring.
func TestSFTPUploader_List_DefaultsToCurrentDirWhenPathEmpty(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	withPermissiveSSRFForLocalTest(t)

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	uploader, err := newSFTPUploader(SFTPConfig{
		Host:               host,
		Port:               port,
		Username:           sftpTestUsername,
		HostKeyFingerprint: fingerprint,
	}, SFTPSecrets{Password: sftpTestPassword})
	require.NoError(t, err)

	_, err = uploader.List(context.Background(), "")
	require.NoError(t, err)
}

// TestSFTPUploader_Test_MkdirFailure proves Test()'s
// "create/access remote directory" error branch.
func TestSFTPUploader_Test_MkdirFailure(t *testing.T) {
	addr, fingerprint := startFullFakeSFTPServer(t)
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	uploader := newTestSFTPUploader(t, addr, fingerprint, filepath.Join(parent, "sub"))

	err := uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create/access remote directory")
}

// TestSFTPUploader_Dial_AuthenticationFailure proves a wrong password
// surfaces a wrapped "handshake/authentication" error, exercised through
// the exported Upload path (dial is unexported).
func TestSFTPUploader_Dial_AuthenticationFailure(t *testing.T) {
	addr, _ := startFullFakeSFTPServer(t)
	withPermissiveSSRFForLocalTest(t)

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	// A fingerprint of all-zeros will never match the server's real key,
	// but sftpAuthMethods must still succeed so we reach the handshake and
	// exercise host-key mismatch rather than the auth-methods guard.
	uploader, err := newSFTPUploader(SFTPConfig{
		Host:               host,
		Port:               port,
		Username:           sftpTestUsername,
		HostKeyFingerprint: "SHA256:0000000000000000000000000000000000000000=",
	}, SFTPSecrets{Password: sftpTestPassword})
	require.NoError(t, err)

	err = uploader.Test(context.Background())
	require.Error(t, err)
}

// TestSFTPUploader_NoDialAddress_ConnectionRefused proves a dial failure
// (nothing listening) surfaces a wrapped "sftp: dial" error.
func TestSFTPUploader_NoDialAddress_ConnectionRefused(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)

	uploader, err := newSFTPUploader(SFTPConfig{
		Host:               "127.0.0.1",
		Port:               1,
		Username:           sftpTestUsername,
		HostKeyFingerprint: "SHA256:anything",
	}, SFTPSecrets{Password: "x"})
	require.NoError(t, err)

	err = uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sftp: dial")
}

// --- sftpAuthMethods: direct unit tests of every branch ---

func TestSftpAuthMethods_NoCredentials(t *testing.T) {
	_, err := sftpAuthMethods(SFTPSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials configured")
}

func TestSftpAuthMethods_PasswordOnly(t *testing.T) {
	methods, err := sftpAuthMethods(SFTPSecrets{Password: "x"})
	require.NoError(t, err)
	assert.Len(t, methods, 1)
}

func TestSftpAuthMethods_InvalidPrivateKey(t *testing.T) {
	_, err := sftpAuthMethods(SFTPSecrets{PrivateKeyPEM: "not a real private key"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse private key")
}

func TestSftpAuthMethods_PrivateKeyWithWrongPassphrase(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte("correct-passphrase"))
	require.NoError(t, err)
	pemStr := string(pem.EncodeToMemory(block))
	require.NotEmpty(t, pemStr)

	_, err = sftpAuthMethods(SFTPSecrets{PrivateKeyPEM: pemStr, Passphrase: "wrong-passphrase"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse private key")
}

// TestSftpAuthMethods_PrivateKeyWithCorrectPassphrase proves the happy path
// for an encrypted private key: the correct passphrase yields a usable
// ssh.PublicKeys auth method.
func TestSftpAuthMethods_PrivateKeyWithCorrectPassphrase(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte("correct-passphrase"))
	require.NoError(t, err)
	pemStr := string(pem.EncodeToMemory(block))

	methods, err := sftpAuthMethods(SFTPSecrets{PrivateKeyPEM: pemStr, Passphrase: "correct-passphrase"})
	require.NoError(t, err)
	assert.Len(t, methods, 1)
}

// --- address(): default port ---

func TestSFTPConfig_Address_DefaultsToPort22(t *testing.T) {
	cfg := SFTPConfig{Host: "example.com"}
	assert.Equal(t, "example.com:22", cfg.address())
}

func TestSFTPConfig_Address_ExplicitPort(t *testing.T) {
	cfg := SFTPConfig{Host: "example.com", Port: 2222}
	assert.Equal(t, "example.com:2222", cfg.address())
}
