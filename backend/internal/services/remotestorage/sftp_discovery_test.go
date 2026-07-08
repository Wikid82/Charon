package remotestorage

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// startFakeSSHServer starts a minimal local SSH server presenting a
// freshly-generated (i.e. unpinned/unknown-to-the-caller) host key, whose
// PasswordCallback and PublicKeyCallback both record whether they were ever
// invoked. It returns the listener address and a thread-safe accessor for
// whether any authentication method was attempted.
func startFakeSSHServer(t *testing.T) (addr string, authAttempted func() bool) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	var mu sync.Mutex
	attempted := false
	markAttempted := func() {
		mu.Lock()
		defer mu.Unlock()
		attempted = true
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			markAttempted()
			return nil, errors.New("auth rejected by test fixture")
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			markAttempted()
			return nil, errors.New("auth rejected by test fixture")
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
			go func() {
				sshConn, chans, reqs, handshakeErr := ssh.NewServerConn(conn, config)
				if handshakeErr != nil {
					_ = conn.Close()
					return
				}
				defer func() { _ = sshConn.Close() }()
				go ssh.DiscardRequests(reqs)
				for newChan := range chans {
					_ = newChan.Reject(ssh.Prohibited, "test fixture accepts no channels")
				}
			}()
		}
	}()

	t.Cleanup(func() { _ = listener.Close() })

	return listener.Addr().String(), func() bool {
		mu.Lock()
		defer mu.Unlock()
		return attempted
	}
}

// withPermissiveSSRFForLocalTest temporarily relaxes the SSRF host/dial
// checks so a test can dial its own 127.0.0.1 fixture server, restoring the
// production defaults on cleanup. Production code never touches these vars.
func withPermissiveSSRFForLocalTest(t *testing.T) {
	t.Helper()
	origHost, origDial := ssrfValidateHost, ssrfValidateDialAddress
	ssrfValidateHost = func(string) error { return nil }
	ssrfValidateDialAddress = func(net.IP) error { return nil }
	t.Cleanup(func() {
		ssrfValidateHost = origHost
		ssrfValidateDialAddress = origDial
	})
}

// TestDiscoverSFTPHostKey_NeverAuthenticates is required test #8 from the
// Issue #32 gap-closing plan: exercising the discovery path against a
// server presenting an unpinned/unknown key must NEVER invoke
// PasswordCallback or PublicKeyCallback on that server — proving no
// credentials reach the wire before the user has confirmed the fingerprint
// out of band (spec §3.7).
func TestDiscoverSFTPHostKey_NeverAuthenticates(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)

	addr, authAttempted := startFakeSSHServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	fingerprint, err := DiscoverSFTPHostKey(SFTPConfig{Host: host, Port: port})
	require.NoError(t, err, "discovery reports the fingerprint as success, not as a handshake error")
	require.NotEmpty(t, fingerprint)
	require.Contains(t, fingerprint, "SHA256:")

	require.False(t, authAttempted(), "no password or public-key auth method may ever be attempted during host key discovery")
}

// TestDiscoverSFTPHostKey_NoHostAvailable proves discovery surfaces a dial
// error (rather than panicking or hanging) when nothing is listening.
func TestDiscoverSFTPHostKey_NoHostAvailable(t *testing.T) {
	withPermissiveSSRFForLocalTest(t)

	_, err := DiscoverSFTPHostKey(SFTPConfig{Host: "127.0.0.1", Port: 1})
	require.Error(t, err)
}

// TestDiscoverSFTPHostKey_RequiresHost proves the empty-host guard runs
// before any SSRF/dial logic.
func TestDiscoverSFTPHostKey_RequiresHost(t *testing.T) {
	_, err := DiscoverSFTPHostKey(SFTPConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "host is required")
}

// TestFingerprintPinnedHostKeyCallback_RejectsMismatch proves the verified
// path's sole HostKeyCallback fails closed on any fingerprint mismatch, and
// accepts only an exact match — this is the pinning half of the
// discovery/verified split (spec §3.7).
func TestFingerprintPinnedHostKeyCallback_RejectsMismatch(t *testing.T) {
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	actual := ssh.FingerprintSHA256(signer.PublicKey())

	callback := fingerprintPinnedHostKeyCallback("SHA256:does-not-match")
	require.Error(t, callback("host:22", nil, signer.PublicKey()))

	callback = fingerprintPinnedHostKeyCallback(actual)
	require.NoError(t, callback("host:22", nil, signer.PublicKey()))
}
