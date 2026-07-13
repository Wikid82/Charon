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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// googleDriveTestSecrets returns a RemoteTargetSecrets with a far-future
// expiry so newGoogleDriveUploader's oauth2.TokenSource never attempts a
// refresh — tests exercise Upload/Delete/List/Test in isolation, not token
// refresh (already covered by oauthtoken_test.go, Commit 2).
func googleDriveTestSecrets() RemoteTargetSecrets {
	return RemoteTargetSecrets{
		OAuthAccessToken:  "test-access-token",
		OAuthRefreshToken: "test-refresh-token",
		OAuthExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
}

// withGoogleDriveTestServer points googleDriveAPIHost at server for the
// duration of the test (mirrors ssrf.go's ssrfValidateHost
// test-substitution seam) so Upload/Delete/List/Test never make a live
// network call to the real Drive API (spec §6 Commit 3 test requirements).
func withGoogleDriveTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	orig := googleDriveAPIHost
	googleDriveAPIHost = server.URL
	t.Cleanup(func() {
		googleDriveAPIHost = orig
		server.Close()
	})
}

func TestNewGoogleDriveUploader_MissingClientID(t *testing.T) {
	_, err := newGoogleDriveUploader(GoogleDriveConfig{}, googleDriveTestSecrets(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestNewGoogleDriveUploader_NotConnected_ReturnsErrOAuthNotConnected(t *testing.T) {
	_, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "id"}, RemoteTargetSecrets{}, nil)
	require.ErrorIs(t, err, ErrOAuthNotConnected)
}

func TestGoogleDriveEscapeQueryValue(t *testing.T) {
	assert.Equal(t, `it\'s`, googleDriveEscapeQueryValue(`it's`))
	assert.Equal(t, `back\\slash`, googleDriveEscapeQueryValue(`back\slash`))
	assert.Equal(t, "plain", googleDriveEscapeQueryValue("plain"))
}

// driveFakeBackend is a minimal, stateful fake of the Drive v3 files
// resource covering exactly the calls googledrive.go makes (files.list,
// files.create, files.get "about", resumable upload, delete) — no live
// network calls (spec §6 Commit 3).
type driveFakeBackend struct {
	mu           sync.Mutex
	nextID       int
	folders      map[string]map[string]string // parentID -> name -> folderID
	uploaded     map[string][]byte            // fileID -> content
	uploadedMeta map[string]string            // fileID -> parent
	deleted      []string
}

func newDriveFakeBackend() *driveFakeBackend {
	return &driveFakeBackend{
		folders:      make(map[string]map[string]string),
		uploaded:     make(map[string][]byte),
		uploadedMeta: make(map[string]string),
	}
}

func (b *driveFakeBackend) newID(prefix string) string {
	b.nextID++
	return fmt.Sprintf("%s-%d", prefix, b.nextID)
}

func (b *driveFakeBackend) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/drive/v3/files", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			// Both findFolder and List issue GET /drive/v3/files?q=...
			q := r.URL.Query().Get("q")
			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(q, "mimeType='application/vnd.google-apps.folder'") {
				parent, name := parseFolderQuery(q)
				id := b.folders[parent][name]
				if id == "" {
					_, _ = w.Write([]byte(`{"files":[]}`))
					return
				}
				_, _ = fmt.Fprintf(w, `{"files":[{"id":%q,"name":%q}]}`, id, name)
				return
			}

			// List: 'leafID' in parents and trashed=false
			leafID := parseParentsQuery(q)
			var files []string
			for fileID, parent := range b.uploadedMeta {
				if parent != leafID {
					continue
				}
				content := b.uploaded[fileID]
				files = append(files, fmt.Sprintf(`{"id":%q,"name":%q,"size":%q,"modifiedTime":"2026-01-01T00:00:00Z"}`,
					fileID, fileID+".zip", strconv.Itoa(len(content))))
			}
			_, _ = fmt.Fprintf(w, `{"files":[%s]}`, strings.Join(files, ","))

		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Name     string   `json:"name"`
				MimeType string   `json:"mimeType"`
				Parents  []string `json:"parents"`
			}
			_ = json.Unmarshal(body, &req)

			id := b.newID("folder")
			parent := ""
			if len(req.Parents) > 0 {
				parent = req.Parents[0]
			}
			if b.folders[parent] == nil {
				b.folders[parent] = make(map[string]string)
			}
			b.folders[parent][req.Name] = id

			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":%q,"name":%q}`, id, req.Name)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/drive/v3/files/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/drive/v3/files/")
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		b.mu.Lock()
		b.deleted = append(b.deleted, id)
		delete(b.uploaded, id)
		delete(b.uploadedMeta, id)
		b.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/upload/drive/v3/files", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var meta struct {
			Name    string   `json:"name"`
			Parents []string `json:"parents"`
		}
		_ = json.Unmarshal(body, &meta)

		b.mu.Lock()
		id := b.newID("file")
		parent := ""
		if len(meta.Parents) > 0 {
			parent = meta.Parents[0]
		}
		b.uploadedMeta[id] = parent
		b.mu.Unlock()

		w.Header().Set("Location", "http://"+r.Host+"/upload/session/"+id)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/upload/session/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/upload/session/")
		body, _ := io.ReadAll(r.Body)
		b.mu.Lock()
		b.uploaded[id] = body
		b.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/drive/v3/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"displayName":"Charon Test"}}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// parseFolderQuery extracts parentID and name from a findFolder query
// string of the shape:
// 'PARENT' in parents and name='NAME' and mimeType='application/vnd.google-apps.folder' and trashed=false
func parseFolderQuery(q string) (parent, name string) {
	parent = between(q, "'", "'")
	nameIdx := strings.Index(q, "name='")
	if nameIdx == -1 {
		return parent, ""
	}
	rest := q[nameIdx+len("name='"):]
	endIdx := strings.Index(rest, "'")
	if endIdx == -1 {
		return parent, ""
	}
	name = rest[:endIdx]
	return parent, name
}

// parseParentsQuery extracts the parent ID from a List query string of the
// shape: 'PARENT' in parents and trashed=false
func parseParentsQuery(q string) string {
	return between(q, "'", "'")
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i == -1 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j == -1 {
		return ""
	}
	return rest[:j]
}

// TestGoogleDriveUploader_Upload_CreatesFolderChainIfMissing is the
// required (§6 Commit 3, R8) folder-chain create-if-missing test: an Upload
// against a multi-segment, entirely-nonexistent FolderPath must create every
// missing segment and land the file in the final (leaf) folder.
func TestGoogleDriveUploader_Upload_CreatesFolderChainIfMissing(t *testing.T) {
	backend := newDriveFakeBackend()
	withGoogleDriveTestServer(t, backend.server(t))

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client", FolderPath: "Charon/Backups"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("drive backup content"), 0o600))

	require.NoError(t, uploader.Upload(context.Background(), local, "backup_1.zip"))

	backend.mu.Lock()
	defer backend.mu.Unlock()
	require.Contains(t, backend.folders[googleDriveRootParent], "Charon", "root segment must be created")
	charonID := backend.folders[googleDriveRootParent]["Charon"]
	require.Contains(t, backend.folders[charonID], "Backups", "nested segment must be created under the first")
	backupsID := backend.folders[charonID]["Backups"]

	found := false
	for fileID, parent := range backend.uploadedMeta {
		if parent == backupsID {
			found = true
			assert.Equal(t, "drive backup content", string(backend.uploaded[fileID]))
		}
	}
	assert.True(t, found, "uploaded file must live in the resolved leaf folder")
}

// TestGoogleDriveUploader_Upload_ReusesExistingFolderChain proves a second
// upload against the same FolderPath does not create duplicate folders.
func TestGoogleDriveUploader_Upload_ReusesExistingFolderChain(t *testing.T) {
	backend := newDriveFakeBackend()
	withGoogleDriveTestServer(t, backend.server(t))

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client", FolderPath: "Charon"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	local := filepath.Join(t.TempDir(), "backup_1.zip")
	require.NoError(t, os.WriteFile(local, []byte("a"), 0o600))
	require.NoError(t, uploader.Upload(context.Background(), local, "backup_1.zip"))
	require.NoError(t, uploader.Upload(context.Background(), local, "backup_2.zip"))

	backend.mu.Lock()
	defer backend.mu.Unlock()
	assert.Len(t, backend.folders[googleDriveRootParent], 1, "the folder chain must be reused, not recreated, on a second upload")
}

func TestGoogleDriveUploader_Delete_RemovesByFileID(t *testing.T) {
	backend := newDriveFakeBackend()
	withGoogleDriveTestServer(t, backend.server(t))

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	require.NoError(t, uploader.Delete(context.Background(), "file-42"))
	assert.Contains(t, backend.deleted, "file-42")
}

// TestGoogleDriveUploader_List_FolderNotFound_ReturnsEmptyNotError is the
// R8 sibling case for Google Drive: a not-yet-existing folder yields an
// empty listing, not an error.
func TestGoogleDriveUploader_List_FolderNotFound_ReturnsEmptyNotError(t *testing.T) {
	backend := newDriveFakeBackend()
	withGoogleDriveTestServer(t, backend.server(t))

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client", FolderPath: "does-not-exist"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	assert.Nil(t, objects)
}

// TestGoogleDriveUploader_List_FollowsPagination is the required (§6
// Commit 3, R12) multi-page files.list/nextPageToken follow test: a fake
// backend returns files across 2 pages, and List must accumulate entries
// from both — this is also the Key != Name proof point (spec §3.2): Key is
// the opaque file ID used for Delete, Name is the human-readable filename
// retention pruning filters on.
func TestGoogleDriveUploader_List_FollowsPagination(t *testing.T) {
	pageRequests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/drive/v3/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query().Get("q")
		if strings.Contains(q, "mimeType='application/vnd.google-apps.folder'") {
			// No configured FolderPath in this test, so resolveFolderChain
			// short-circuits before ever issuing this query — but keep a
			// safe default in case that changes.
			_, _ = w.Write([]byte(`{"files":[]}`))
			return
		}

		pageRequests++
		pageToken := r.URL.Query().Get("pageToken")
		switch pageToken {
		case "":
			_, _ = w.Write([]byte(`{
				"files": [{"id":"file-1","name":"backup_1.zip","size":"100","modifiedTime":"2026-01-01T00:00:00Z"}],
				"nextPageToken": "page-2"
			}`))
		case "page-2":
			_, _ = w.Write([]byte(`{
				"files": [{"id":"file-2","name":"backup_2.zip","size":"200","modifiedTime":"2026-01-02T00:00:00Z"}]
			}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	server := httptest.NewServer(mux)
	withGoogleDriveTestServer(t, server)

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	objects, err := uploader.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, objects, 2, "must accumulate files across both pages, not just the first")
	assert.Equal(t, 2, pageRequests)

	byName := make(map[string]string)
	for _, obj := range objects {
		byName[obj.Name] = obj.Key
	}
	assert.Equal(t, "file-1", byName["backup_1.zip"], "Key must be the opaque file ID, distinct from Name")
	assert.Equal(t, "file-2", byName["backup_2.zip"])
}

func TestGoogleDriveUploader_Test_Success(t *testing.T) {
	backend := newDriveFakeBackend()
	withGoogleDriveTestServer(t, backend.server(t))

	uploader, err := newGoogleDriveUploader(GoogleDriveConfig{ClientID: "client"}, googleDriveTestSecrets(), nil)
	require.NoError(t, err)

	require.NoError(t, uploader.Test(context.Background()))
}

func TestGoogleDriveOAuthConfig_Endpoint(t *testing.T) {
	conf := GoogleDriveOAuthConfig("client-id", "client-secret", "https://charon.example.com/callback")
	assert.Equal(t, "client-id", conf.ClientID)
	assert.Equal(t, "client-secret", conf.ClientSecret)
	assert.Equal(t, "https://charon.example.com/callback", conf.RedirectURL)
	assert.Equal(t, googleDriveAuthURL, conf.Endpoint.AuthURL)
	assert.Equal(t, googleDriveTokenURL, conf.Endpoint.TokenURL)
	assert.Contains(t, conf.Scopes, "https://www.googleapis.com/auth/drive.file")
}

func TestGoogleDriveAuthCodeOptions_RequestsOfflineAccessAndConsent(t *testing.T) {
	opts := GoogleDriveAuthCodeOptions()
	assert.Len(t, opts, 2)
}
