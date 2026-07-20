package remotestorage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

// WebDAVConfig is the non-secret configuration for a webdav-type
// RemoteStorageTarget (spec §3.2/§3.5). Username is non-secret and lives
// here (mirroring how SFTPConfig.Username already lives in config, not
// secrets); Password/BearerToken are the secret half (WebDAVSecrets below).
type WebDAVConfig struct {
	URL                string `json:"url"`
	Username           string `json:"username,omitempty"`
	BasePath           string `json:"base_path,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// WebDAVSecrets is the secret credential material for a webdav-type target:
// either Password (Basic auth, paired with WebDAVConfig.Username) or
// BearerToken (spec §1.3 Non-Goals — Basic auth + bearer token only, no
// Digest).
type WebDAVSecrets struct {
	Password    string `json:"password,omitempty"`
	BearerToken string `json:"bearer_token,omitempty"`
}

// webdavConnectionTestFile is the marker file written/deleted by Test,
// mirroring sftp.go's Test semantics exactly (spec §3.5).
const webdavConnectionTestFile = "charon-connection-test"

// webdavUploader implements Uploader for a generic WebDAV server (e.g.
// Nextcloud, ownCloud, Apache/nginx mod_dav) via github.com/studio-b12/gowebdav.
type webdavUploader struct {
	client   *gowebdav.Client
	basePath string
}

// newWebDAVUploader constructs the Uploader for a webdav-type target. The
// host is SSRF-validated at construction time via the same indirected
// ssrfValidateHost var s3.go/sftp.go already use (spec §3.5 — "reuse it,
// don't reinvent"), and every subsequent request dials through the
// SSRF-safe dialContext (defeating DNS-rebinding TOCTOU, spec §3.8)
// exactly like s3.go's custom http.Transport.
func newWebDAVUploader(cfg WebDAVConfig, secrets WebDAVSecrets) (Uploader, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("webdav: url is required")
	}

	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("webdav: invalid url: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("webdav: url must include a host")
	}
	if err := ssrfValidateHost(host); err != nil {
		return nil, fmt.Errorf("webdav url failed SSRF validation: %w", err)
	}

	client := gowebdav.NewClient(cfg.URL, cfg.Username, secrets.Password)
	client.SetTransport(&http.Transport{
		DialContext: func(ctx context.Context, dialNetwork, addr string) (net.Conn, error) {
			return dialContext(ctx, dialNetwork, addr, 10*time.Second)
		},
		// InsecureSkipVerify is an explicit, admin-opted-in per-target
		// setting (WebDAVConfig.InsecureSkipVerify, default false) for
		// self-signed self-hosted WebDAV servers — not a blanket default.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify, MinVersion: tls.VersionTLS12}, // #nosec G402 -- admin-opt-in per target, defaults to false (verified)
	})
	if secrets.BearerToken != "" {
		client.SetHeader("Authorization", "Bearer "+secrets.BearerToken)
	}

	return &webdavUploader{client: client, basePath: cfg.BasePath}, nil
}

// remotePath joins the configured base path with name, mirroring
// sftpUploader.remotePath.
func (u *webdavUploader) remotePath(name string) string {
	if u.basePath == "" {
		return name
	}
	return path.Join(u.basePath, name)
}

func (u *webdavUploader) Upload(_ context.Context, localPath, remoteKey string) error {
	f, err := os.Open(localPath) // #nosec G304 -- localPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("webdav: open local file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	remoteFile := u.remotePath(remoteKey)
	if err := u.client.MkdirAll(path.Dir(remoteFile), 0o755); err != nil {
		return fmt.Errorf("webdav: create remote directory: %w", err)
	}
	if err := u.client.WriteStream(remoteFile, f, 0o644); err != nil {
		return fmt.Errorf("webdav: write remote file: %w", err)
	}
	return nil
}

func (u *webdavUploader) Delete(_ context.Context, remoteKey string) error {
	if err := u.client.Remove(remoteKey); err != nil {
		return fmt.Errorf("webdav: remove remote file: %w", err)
	}
	return nil
}

func (u *webdavUploader) List(_ context.Context, prefix string) ([]RemoteObject, error) {
	dir := u.basePath
	if dir == "" {
		dir = "/"
	}

	entries, err := u.client.ReadDir(dir)
	if err != nil {
		if gowebdav.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("webdav: list remote directory: %w", err)
	}

	var objects []RemoteObject
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		// Key is the full path (needed by Delete/Upload); Name is the
		// human-readable basename retention pruning filters on — the same
		// Key/Name split s3.go uses (spec §3.2 Locator abstraction).
		objects = append(objects, RemoteObject{
			Key:          path.Join(dir, name),
			Name:         name,
			Size:         entry.Size(),
			LastModified: entry.ModTime(),
		})
	}
	return objects, nil
}

func (u *webdavUploader) Test(_ context.Context) error {
	dir := u.basePath
	if dir == "" {
		dir = "/"
	}
	if err := u.client.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("webdav: create/access remote directory: %w", err)
	}

	testPath := path.Join(dir, webdavConnectionTestFile)
	if err := u.client.Write(testPath, []byte("charon connection test"), 0o644); err != nil {
		return fmt.Errorf("webdav: write connection test marker: %w", err)
	}
	if err := u.client.Remove(testPath); err != nil {
		return fmt.Errorf("webdav: remove connection test marker: %w", err)
	}
	return nil
}
