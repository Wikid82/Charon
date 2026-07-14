package remotestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropboxTestSecrets returns a RemoteTargetSecrets with a far-future expiry
// so newDropboxUploader's oauth2.TokenSource never attempts a refresh —
// tests exercise Upload/Delete/List/Test in isolation, not token refresh
// (already covered by oauthtoken_test.go, Commit 2).
func dropboxTestSecrets() RemoteTargetSecrets {
	return RemoteTargetSecrets{
		OAuthAccessToken:  "test-access-token",
		OAuthRefreshToken: "test-refresh-token",
		OAuthExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
}

// withDropboxTestServer points dropboxContentHost/dropboxAPIHost at server
// for the duration of the test (mirrors ssrf.go's ssrfValidateHost
// test-substitution seam) so Upload/Delete/List/Test never make a live
// network call to the real Dropbox API (spec §6 Commit 3 test
// requirements).
func withDropboxTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	origContent, origAPI := dropboxContentHost, dropboxAPIHost
	dropboxContentHost = server.URL
	dropboxAPIHost = server.URL
	t.Cleanup(func() {
		dropboxContentHost = origContent
		dropboxAPIHost = origAPI
		server.Close()
	})
}

func TestNewDropboxUploader_MissingAppKey(t *testing.T) {
	_, err := newDropboxUploader(DropboxConfig{}, dropboxTestSecrets(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_key")
}

func TestNewDropboxUploader_NotConnected_ReturnsErrOAuthNotConnected(t *testing.T) {
	_, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, RemoteTargetSecrets{}, nil)
	require.ErrorIs(t, err, ErrOAuthNotConnected)
}

func TestNormalizeDropboxFolder(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"/":                "",
		"charon-backups":   "/charon-backups",
		"/charon-backups/": "/charon-backups",
		"/a/b":             "/a/b",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeDropboxFolder(in), "input %q", in)
	}
}

func TestDropboxUploader_UploadSingle_SendsCorrectPathAndBody(t *testing.T) {
	var gotPath string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/2/files/upload") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		arg := r.Header.Get("Dropbox-API-Arg")
		var decoded struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
		}
		_ = json.Unmarshal([]byte(arg), &decoded)
		gotPath = decoded.Path
		gotBody, _ = io.ReadAll(r.Body)
		assert.Equal(t, "overwrite", decoded.Mode)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key", FolderPath: "/charon-backups"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("hello world"), 0o600))

	require.NoError(t, uploader.Upload(context.Background(), local, "backup_1.zip"))
	assert.Equal(t, "/charon-backups/backup_1.zip", gotPath)
	assert.Equal(t, "hello world", string(gotBody))
}

// TestDropboxUploader_UploadChunked_LargeFile is the required (§6 Commit 3)
// chunked-upload-path test: a synthetic file over Dropbox's 150 MiB
// single-request cap must go through upload_session/start → append_v2 →
// finish (R11), and every byte of the original file must reach the fake
// server across those three calls.
func TestDropboxUploader_UploadChunked_LargeFile(t *testing.T) {
	var mu sync.Mutex
	var totalBytes int64
	var finishedPath string
	sessionStarts := 0
	appends := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/start"):
			mu.Lock()
			sessionStarts++
			totalBytes += int64(len(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"session_id":"session-1"}`)
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/append_v2"):
			mu.Lock()
			appends++
			totalBytes += int64(len(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/finish"):
			arg := r.Header.Get("Dropbox-API-Arg")
			var decoded struct {
				Commit struct {
					Path string `json:"path"`
				} `json:"commit"`
			}
			_ = json.Unmarshal([]byte(arg), &decoded)
			mu.Lock()
			finishedPath = decoded.Commit.Path
			totalBytes += int64(len(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	// 150 MiB cap + 3 MiB: forces the chunked path, and isn't an exact
	// multiple of the 8 MiB chunk size, exercising the partial-final-chunk
	// branch of uploadChunked.
	size := int64(dropboxMaxSingleUpload) + 3*1024*1024
	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, size), 0o600))

	require.NoError(t, uploader.Upload(context.Background(), local, "backup_big.zip"))

	assert.Equal(t, "/backup_big.zip", finishedPath)
	assert.Equal(t, 1, sessionStarts)
	assert.Positive(t, appends)
	assert.Equal(t, size, totalBytes, "every byte of the local file must reach the fake server across start+append+finish")
}

func TestDropboxUploader_Delete_SendsCorrectPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(body, &decoded)
		gotPath = decoded.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	require.NoError(t, uploader.Delete(context.Background(), "/backup_1.zip"))
	assert.Equal(t, "/backup_1.zip", gotPath)
}

// TestDropboxUploader_List_FollowsPagination is the required (§6 Commit 3,
// R12) multi-page list_folder/list_folder_continue cursor-follow test: a
// fake backend returns has_more:true across 3 pages, and List must
// accumulate entries from every page, not just the first.
func TestDropboxUploader_List_FollowsPagination(t *testing.T) {
	continueCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/list_folder"):
			_, _ = w.Write([]byte(`{
				"entries": [
					{".tag":"file","name":"backup_1.zip","path_lower":"/charon/backup_1.zip","size":100,"server_modified":"2026-01-01T00:00:00Z"},
					{".tag":"folder","name":"subdir","path_lower":"/charon/subdir"}
				],
				"cursor": "cursor-page-1",
				"has_more": true
			}`))
		case strings.HasSuffix(r.URL.Path, "/2/files/list_folder/continue"):
			continueCalls++
			body, _ := io.ReadAll(r.Body)
			var decoded struct {
				Cursor string `json:"cursor"`
			}
			_ = json.Unmarshal(body, &decoded)
			if decoded.Cursor == "cursor-page-1" {
				_, _ = w.Write([]byte(`{
					"entries": [
						{".tag":"file","name":"backup_2.zip","path_lower":"/charon/backup_2.zip","size":200,"server_modified":"2026-01-02T00:00:00Z"}
					],
					"cursor": "cursor-page-2",
					"has_more": true
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"entries": [
					{".tag":"file","name":"backup_3.zip","path_lower":"/charon/backup_3.zip","size":300,"server_modified":"2026-01-03T00:00:00Z"}
				],
				"cursor": "cursor-page-3",
				"has_more": false
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key", FolderPath: "/charon"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, objects, 3, "must accumulate entries across all 3 pages, not just the first")

	var names []string
	for _, obj := range objects {
		names = append(names, obj.Name)
	}
	assert.ElementsMatch(t, []string{"backup_1.zip", "backup_2.zip", "backup_3.zip"}, names)
	assert.Equal(t, 2, continueCalls, "must follow the cursor chain through both continuation pages")
}

func TestDropboxUploader_List_FolderNotFound_ReturnsEmptyNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error_summary": "path/not_found/...", "error": {".tag": "path", "path": {".tag": "not_found"}}}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	assert.Nil(t, objects)
}

func TestDropboxUploader_Test_Success(t *testing.T) {
	var hitPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"account_id":"abc"}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	require.NoError(t, uploader.Test(context.Background()))
	assert.Equal(t, "/2/users/get_current_account", hitPath)
}

func TestDropboxUploader_Test_APIError_Propagated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error_summary": "invalid_access_token/..."}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	err = uploader.Test(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_access_token")
}

func TestDropboxOAuthConfig_Endpoint(t *testing.T) {
	conf := DropboxOAuthConfig("app-key", "app-secret", "https://charon.example.com/callback")
	assert.Equal(t, "app-key", conf.ClientID)
	assert.Equal(t, "app-secret", conf.ClientSecret)
	assert.Equal(t, "https://charon.example.com/callback", conf.RedirectURL)
	assert.Equal(t, dropboxAuthURL, conf.Endpoint.AuthURL)
	assert.Equal(t, dropboxTokenURL, conf.Endpoint.TokenURL)
}

func TestDropboxAuthCodeOptions_RequestsOfflineAccess(t *testing.T) {
	opts := DropboxAuthCodeOptions()
	require.Len(t, opts, 1)
}

func TestSetDropboxTokenURLForTesting_RestoresOriginal(t *testing.T) {
	orig := dropboxTokenURL
	restore := SetDropboxTokenURLForTesting("http://example.invalid/token")
	assert.Equal(t, "http://example.invalid/token", dropboxTokenURL)
	restore()
	assert.Equal(t, orig, dropboxTokenURL)
}

func TestDropboxUploader_Upload_OpenFileError(t *testing.T) {
	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	err = uploader.Upload(context.Background(), filepath.Join(t.TempDir(), "does-not-exist.zip"), "backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open local file")
}

func TestDropboxUploader_UploadSingle_NonOKStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("hi"), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_UploadSingle_RequestBuildError(t *testing.T) {
	origContent, origAPI := dropboxContentHost, dropboxAPIHost
	dropboxContentHost = "http://\x7f"
	dropboxAPIHost = "http://\x7f"
	t.Cleanup(func() {
		dropboxContentHost = origContent
		dropboxAPIHost = origAPI
	})

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("hi"), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build upload request")
}

func TestDropboxUploader_UploadSingle_ClientDoError(t *testing.T) {
	origContent, origAPI := dropboxContentHost, dropboxAPIHost
	dropboxContentHost = "http://127.0.0.1:1" // reserved, closed port: connection refused
	dropboxAPIHost = "http://127.0.0.1:1"
	t.Cleanup(func() {
		dropboxContentHost = origContent
		dropboxAPIHost = origAPI
	})

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("hi"), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload request")
}

// TestDropboxUploader_UploadChunked_FirstChunkReadError covers the "read
// first chunk" error branch: closing the local file out from under
// uploadChunked before it reads guarantees a deterministic read error,
// without relying on OS-specific I/O failure injection.
func TestDropboxUploader_UploadChunked_FirstChunkReadError(t *testing.T) {
	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)
	du := uploader.(*dropboxUploader)

	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, dropboxChunkSize), 0o600))
	f, err := os.Open(local) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	require.NoError(t, f.Close())

	err = du.uploadChunked(context.Background(), f, dropboxMaxSingleUpload+1, "/backup_big.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read first chunk")
}

// TestDropboxUploader_UploadChunked_ExactChunkSizeFile_ReturnsNil directly
// exercises uploadChunked (not reachable via the public Upload, which only
// calls it when size > dropboxMaxSingleUpload) with a file whose length
// exactly equals size and exactly fills the first chunk read: the loop
// condition `offset < size` is false on the very first check, falling
// through to the trailing `return nil` rather than any in-loop return.
func TestDropboxUploader_UploadChunked_ExactChunkSizeFile_ReturnsNil(t *testing.T) {
	var sessionStarted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/2/files/upload_session/start") {
			sessionStarted = true
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"session_id":"session-1"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)
	du := uploader.(*dropboxUploader)

	local := filepath.Join(t.TempDir(), "backup_exact.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, dropboxChunkSize), 0o600))
	f, err := os.Open(local) // #nosec G304 -- test fixture path
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	err = du.uploadChunked(context.Background(), f, dropboxChunkSize, "/backup_exact.zip")
	require.NoError(t, err)
	assert.True(t, sessionStarted, "must still start an upload session for the single, exactly-sized chunk")
}

func TestDropboxUploader_UploadChunked_SessionStartError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, dropboxMaxSingleUpload+1024), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_big.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_UploadChunked_SessionStartDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, dropboxMaxSingleUpload+1024), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_big.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode upload_session/start response")
}

func TestDropboxUploader_UploadChunked_AppendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/start"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"session_id":"session-1"}`)
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/append_v2"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	// Large enough to require at least one append_v2 call beyond the initial
	// session-start chunk (dropboxMaxSingleUpload + 3 chunk-widths).
	size := int64(dropboxMaxSingleUpload) + 3*dropboxChunkSize
	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, size), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_big.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_UploadChunked_FinishError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/start"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"session_id":"session-1"}`)
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/append_v2"):
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/2/files/upload_session/finish"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	size := int64(dropboxMaxSingleUpload) + 3*1024*1024
	local := filepath.Join(t.TempDir(), "backup_big.zip")
	require.NoError(t, os.WriteFile(local, make([]byte, size), 0o600))

	err = uploader.Upload(context.Background(), local, "backup_big.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_Delete_RequestBuildError(t *testing.T) {
	origContent, origAPI := dropboxContentHost, dropboxAPIHost
	dropboxContentHost = "http://\x7f"
	dropboxAPIHost = "http://\x7f"
	t.Cleanup(func() {
		dropboxContentHost = origContent
		dropboxAPIHost = origAPI
	})

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	err = uploader.Delete(context.Background(), "/backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build request")
}

func TestDropboxUploader_Delete_ClientDoError(t *testing.T) {
	origContent, origAPI := dropboxContentHost, dropboxAPIHost
	dropboxContentHost = "http://127.0.0.1:1"
	dropboxAPIHost = "http://127.0.0.1:1"
	t.Cleanup(func() {
		dropboxContentHost = origContent
		dropboxAPIHost = origAPI
	})

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	err = uploader.Delete(context.Background(), "/backup_1.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request /2/files/delete_v2")
}

func TestDropboxUploader_List_InitialCallError_NotPathNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, objects)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_List_ContinuePageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/list_folder"):
			_, _ = w.Write([]byte(`{"entries":[],"cursor":"cursor-1","has_more":true}`))
		case strings.HasSuffix(r.URL.Path, "/2/files/list_folder/continue"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error_summary": "internal_error/..."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, objects)
	assert.Contains(t, err.Error(), "internal_error")
}

func TestDropboxUploader_List_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	withDropboxTestServer(t, server)

	uploader, err := newDropboxUploader(DropboxConfig{AppKey: "key"}, dropboxTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, objects)
	assert.Contains(t, err.Error(), "decode response")
}
