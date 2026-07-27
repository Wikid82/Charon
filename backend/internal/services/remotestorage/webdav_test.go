package remotestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// newTestWebDAVServer starts a real WebDAV server (golang.org/x/net/webdav's
// server-side Handler — already an existing dependency, spec §2.5 research)
// backed by a real temp directory, optionally gated by authCheck. This lets
// webdavUploader be exercised end-to-end over genuine WebDAV
// PROPFIND/MKCOL/PUT/DELETE requests instead of a hand-rolled stub.
func newTestWebDAVServer(t *testing.T, root string, authCheck func(r *http.Request) bool) *httptest.Server {
	t.Helper()
	handler := &webdav.Handler{
		FileSystem: webdav.Dir(root),
		LockSystem: webdav.NewMemLS(),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authCheck != nil && !authCheck(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="charon-test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

// newTestWebDAVUploader constructs a real webdavUploader (via the exported
// construction path, newWebDAVUploader) pointed at server's URL, with SSRF
// checks relaxed for the loopback fixture.
func newTestWebDAVUploader(t *testing.T, server *httptest.Server, basePath string, secrets WebDAVSecrets) Uploader {
	t.Helper()
	withPermissiveSSRFForLocalTest(t)

	uploader, err := newWebDAVUploader(WebDAVConfig{
		URL:      server.URL + "/",
		Username: "charon",
		BasePath: basePath,
	}, secrets)
	require.NoError(t, err)
	return uploader
}

func TestWebDAVUploader_UploadListDeleteTest_Roundtrip(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, nil)
	uploader := newTestWebDAVUploader(t, server, "/charon-backups", WebDAVSecrets{})
	ctx := context.Background()

	require.NoError(t, uploader.Test(ctx), "Test() must MkdirAll + write/delete its connectivity marker successfully")

	localFile := filepath.Join(t.TempDir(), "backup.zip")
	require.NoError(t, os.WriteFile(localFile, []byte("fake webdav backup contents"), 0o600))

	require.NoError(t, uploader.Upload(ctx, localFile, "backup_1.zip"))
	require.FileExists(t, filepath.Join(root, "charon-backups", "backup_1.zip"))

	objects, err := uploader.List(ctx, "")
	require.NoError(t, err)
	var names []string
	var deleteKey string
	for _, o := range objects {
		names = append(names, o.Name)
		assert.Greater(t, o.Size, int64(0))
		if o.Name == "backup_1.zip" {
			deleteKey = o.Key
		}
	}
	assert.Contains(t, names, "backup_1.zip")
	require.NotEmpty(t, deleteKey, "List must return a usable Key for the uploaded object")

	require.NoError(t, uploader.Delete(ctx, deleteKey))
	assert.NoFileExists(t, filepath.Join(root, "charon-backups", "backup_1.zip"))
}

// TestWebDAVUploader_BasicAuth_Roundtrip proves Username/Password (Basic
// auth) credentials are actually presented and accepted by a server that
// requires them (spec §1.3 — Basic auth is the primary supported scheme).
func TestWebDAVUploader_BasicAuth_Roundtrip(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, func(r *http.Request) bool {
		user, pass, ok := r.BasicAuth()
		return ok && user == "charon" && pass == "s3cr3t"
	})
	uploader := newTestWebDAVUploader(t, server, "", WebDAVSecrets{Password: "s3cr3t"})
	ctx := context.Background()

	require.NoError(t, uploader.Test(ctx))
}

// TestWebDAVUploader_BearerToken_Roundtrip proves the bearer-token
// alternative (spec §1.3/§3.2) works end-to-end against a server that
// requires a bearer Authorization header instead of Basic auth.
func TestWebDAVUploader_BearerToken_Roundtrip(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer test-bearer-token"
	})
	uploader := newTestWebDAVUploader(t, server, "", WebDAVSecrets{BearerToken: "test-bearer-token"})
	ctx := context.Background()

	require.NoError(t, uploader.Test(ctx))
}

// TestWebDAVUploader_List_MissingDirectory_ReturnsEmptyNotError mirrors
// sftp.go's os.IsNotExist -> empty-list behavior: a target never uploaded
// to yet must not surface as an error.
func TestWebDAVUploader_List_MissingDirectory_ReturnsEmptyNotError(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, nil)
	uploader := newTestWebDAVUploader(t, server, "/does-not-exist-yet", WebDAVSecrets{})

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, objects)
}

func TestWebDAVUploader_Upload_LocalFileMissing(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, nil)
	uploader := newTestWebDAVUploader(t, server, "", WebDAVSecrets{})

	err := uploader.Upload(context.Background(), filepath.Join(t.TempDir(), "does-not-exist.zip"), "backup.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open local file")
}

func TestWebDAVUploader_Delete_RemoteFileMissing(t *testing.T) {
	root := t.TempDir()
	server := newTestWebDAVServer(t, root, nil)
	uploader := newTestWebDAVUploader(t, server, "", WebDAVSecrets{})

	// golang.org/x/net/webdav's DELETE handler returns 404 for a missing
	// resource, which gowebdav's Remove treats as success (idempotent
	// delete) — mirrored here to lock in that this does NOT surface as an
	// uploader-level error.
	err := uploader.Delete(context.Background(), "does-not-exist.zip")
	assert.NoError(t, err)
}

// TestWebDAVUploader_InsecureSkipVerify_AllowsSelfSignedCert proves
// cfg.InsecureSkipVerify actually reaches the underlying transport: a
// self-signed TLS server is only reachable when it's true.
func TestWebDAVUploader_InsecureSkipVerify_AllowsSelfSignedCert(t *testing.T) {
	root := t.TempDir()
	handler := &webdav.Handler{FileSystem: webdav.Dir(root), LockSystem: webdav.NewMemLS()}
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	withPermissiveSSRFForLocalTest(t)

	secureUploader, err := newWebDAVUploader(WebDAVConfig{URL: server.URL + "/"}, WebDAVSecrets{})
	require.NoError(t, err)
	require.Error(t, secureUploader.Test(context.Background()), "a self-signed cert must be rejected when InsecureSkipVerify is false")

	insecureUploader, err := newWebDAVUploader(WebDAVConfig{URL: server.URL + "/", InsecureSkipVerify: true}, WebDAVSecrets{})
	require.NoError(t, err)
	require.NoError(t, insecureUploader.Test(context.Background()), "InsecureSkipVerify=true must allow the self-signed cert through")
}

// --- newWebDAVUploader: construction-time validation ---

func TestNewWebDAVUploader_MissingURL(t *testing.T) {
	_, err := newWebDAVUploader(WebDAVConfig{}, WebDAVSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is required")
}

func TestNewWebDAVUploader_InvalidURL(t *testing.T) {
	_, err := newWebDAVUploader(WebDAVConfig{URL: "http://bad url with spaces/"}, WebDAVSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid url")
}

func TestNewWebDAVUploader_URLWithoutHost(t *testing.T) {
	_, err := newWebDAVUploader(WebDAVConfig{URL: "/just/a/path"}, WebDAVSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must include a host")
}

// TestNewWebDAVUploader_SSRFRejected mirrors the existing s3.go/sftp.go
// SSRF-rejection tests exactly (spec §6 Commit 2): a loopback/link-local/
// metadata URL host must be rejected at construction (config-save) time.
func TestNewWebDAVUploader_SSRFRejected(t *testing.T) {
	_, err := newWebDAVUploader(WebDAVConfig{URL: "http://169.254.169.254/dav/"}, WebDAVSecrets{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF validation")
}
