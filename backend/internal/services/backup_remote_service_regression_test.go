package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// This file is the hard requirement called out in spec §3.2/§5/§6 Commit 2:
// pruneRemoteRetention's filter switched from path.Base(obj.Key) to
// obj.Name, which is a silent, permanent regression for every already-
// configured S3/SFTP target unless s3.go/sftp.go's List() implementations
// were updated in lockstep to populate Name. These tests exercise the REAL
// s3Uploader/sftpUploader (constructed through the exported
// remotestorage.New entry point — the same path production uploaderFor
// uses) against a fake S3-compatible/SFTP backend end-to-end through
// pruneRemoteRetention, proving retention pruning still deletes candidates
// beyond the retention count after the Name field was introduced.

// --- fake S3-compatible backend ---

type regressionFakeS3Object struct {
	body     []byte
	modified time.Time
}

// regressionFakeS3Server is a minimal S3-compatible HTTP server. Unlike
// s3_upload_test.go's fakeS3Server (which stamps every listed object with
// time.Now() at LIST time, fine for that file's non-ordering-sensitive
// assertions), this one records each object's LastModified at PUT time so
// retention pruning's "sort by LastModified desc, delete beyond
// retentionCount" behavior is deterministic and exercisable here.
type regressionFakeS3Server struct {
	mu      sync.Mutex
	bucket  string
	objects map[string]*regressionFakeS3Object
}

func newRegressionFakeS3Server(bucket string) *regressionFakeS3Server {
	return &regressionFakeS3Server{bucket: bucket, objects: make(map[string]*regressionFakeS3Object)}
}

func (s *regressionFakeS3Server) start(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(server.Close)
	return server
}

func (s *regressionFakeS3Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	bucket := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	switch r.Method {
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.objects[key] = &regressionFakeS3Object{body: body, modified: time.Now()}
		w.Header().Set("ETag", `"fakeetag"`)
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		s.handleList(w, bucket)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *regressionFakeS3Server) handleList(w http.ResponseWriter, bucket string) {
	type content struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}
	type result struct {
		XMLName     xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
		Name        string   `xml:"Name"`
		IsTruncated bool     `xml:"IsTruncated"`
		Contents    []content
	}

	res := result{Name: bucket}
	for key, obj := range s.objects {
		res.Contents = append(res.Contents, content{
			Key:          key,
			LastModified: obj.modified.UTC().Format(time.RFC3339Nano),
			ETag:         `"x"`,
			Size:         int64(len(obj.body)),
			StorageClass: "STANDARD",
		})
	}

	payload, err := xml.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(payload) // nosemgrep: go.net.xss.no-direct-write-to-responsewriter-taint.no-direct-write-to-responsewriter-taint -- test-only fake S3 server responding with XML to the real s3Uploader (minio-go client) under test, not rendered in a browser.
}

// TestPruneRemoteRetention_RealS3List_StillDeletesCandidatesAfterNameFieldAdded
// is the S3 half of the hard-requirement regression guard (spec §3.2/§6
// Commit 2).
func TestPruneRemoteRetention_RealS3List_StillDeletesCandidatesAfterNameFieldAdded(t *testing.T) {
	restore := remotestorage.WithPermissiveSSRFForTesting()
	t.Cleanup(restore)

	fake := newRegressionFakeS3Server("charon-backups")
	server := fake.start(t)
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)

	cfgJSON, err := json.Marshal(remotestorage.S3Config{
		Endpoint:       endpoint.Host,
		Region:         "us-east-1",
		Bucket:         "charon-backups",
		ForcePathStyle: true,
	})
	require.NoError(t, err)

	target := &models.RemoteStorageTarget{Type: "s3", ConfigJSON: string(cfgJSON)}
	uploader, err := remotestorage.New(target, map[string]string{"access_key_id": "x", "secret_access_key": "y"}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	for _, name := range []string{"backup_1.zip", "backup_2.zip", "backup_3.zip", "backup_4.zip"} {
		local := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(local, []byte("data-"+name), 0o600))
		require.NoError(t, uploader.Upload(ctx, local, name))
		// Real wall-clock time between uploads keeps LastModified strictly
		// increasing so retention's "sort desc, keep newest N" is
		// deterministic without needing to fabricate timestamps.
		time.Sleep(5 * time.Millisecond)
	}

	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.pruneRemoteRetention(ctx, uploader, "", 2)

	remaining, err := uploader.List(ctx, "")
	require.NoError(t, err)
	var names []string
	for _, o := range remaining {
		names = append(names, o.Key)
	}
	assert.ElementsMatch(t, []string{"backup_3.zip", "backup_4.zip"}, names,
		"retention pruning must still delete the oldest real-S3-listed backups by Name after RemoteObject.Name was introduced")
}

// --- fake SFTP backend (mirrors remotestorage's sftp_upload_test.go
// startFullFakeSFTPServer, duplicated here because that helper is
// unexported and package-private to remotestorage — this regression test
// must live in package services to reach the unexported
// BackupRemoteService.pruneRemoteRetention it's guarding). ---

const (
	regressionSFTPUsername = "charon-regression-test"
	regressionSFTPPassword = "regression-password-hunter2"
)

func startRegressionFakeSFTPServer(t *testing.T) (addr, hostKeyFingerprint string) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == regressionSFTPUsername && string(password) == regressionSFTPPassword {
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
			go serveRegressionFakeSFTPConnection(conn, config)
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })

	return listener.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

func serveRegressionFakeSFTPConnection(conn net.Conn, config *ssh.ServerConfig) {
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
			srv, srvErr := sftp.NewServer(channel)
			if srvErr != nil {
				return
			}
			_ = srv.Serve()
			_ = srv.Close()
		}()
	}
}

// TestPruneRemoteRetention_RealSFTPList_StillDeletesCandidatesAfterNameFieldAdded
// is the SFTP half of the hard-requirement regression guard (spec §3.2/§6
// Commit 2).
func TestPruneRemoteRetention_RealSFTPList_StillDeletesCandidatesAfterNameFieldAdded(t *testing.T) {
	restore := remotestorage.WithPermissiveSSRFForTesting()
	t.Cleanup(restore)

	addr, fingerprint := startRegressionFakeSFTPServer(t)
	remoteRoot := t.TempDir()

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	cfgJSON, err := json.Marshal(remotestorage.SFTPConfig{
		Host:               host,
		Port:               port,
		Path:               remoteRoot,
		Username:           regressionSFTPUsername,
		HostKeyFingerprint: fingerprint,
	})
	require.NoError(t, err)

	target := &models.RemoteStorageTarget{Type: "sftp", ConfigJSON: string(cfgJSON)}
	uploader, err := remotestorage.New(target, map[string]string{"password": regressionSFTPPassword}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	for _, name := range []string{"backup_1.zip", "backup_2.zip", "backup_3.zip", "backup_4.zip"} {
		local := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(local, []byte("data-"+name), 0o600))
		require.NoError(t, uploader.Upload(ctx, local, name))
		time.Sleep(1100 * time.Millisecond) // SFTP ModTime has ~1s resolution on most filesystems
	}

	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, nil, t.TempDir())
	svc.pruneRemoteRetention(ctx, uploader, "", 2)

	remaining, err := uploader.List(ctx, "")
	require.NoError(t, err)
	var names []string
	for _, o := range remaining {
		names = append(names, o.Key)
	}
	assert.ElementsMatch(t, []string{"backup_3.zip", "backup_4.zip"}, names,
		"retention pruning must still delete the oldest real-SFTP-listed backups by Name after RemoteObject.Name was introduced")
}
