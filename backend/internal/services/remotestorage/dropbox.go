package remotestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// DropboxConfig is the non-secret configuration for a dropbox-type
// RemoteStorageTarget (spec §3.2/§3.5, Commit 3). AppKey is Dropbox's
// non-secret "App key" (the OAuth2 client_id); the paired "App secret"
// lives in RemoteTargetSecrets.OAuthClientSecret.
type DropboxConfig struct {
	AppKey     string `json:"app_key"`
	FolderPath string `json:"folder_path,omitempty"`
}

const (
	dropboxAuthURL         = "https://www.dropbox.com/oauth2/authorize"
	dropboxTokenURL        = "https://api.dropboxapi.com/oauth2/token" // #nosec G101 -- public OAuth2 token endpoint URL, not a credential
	dropboxChunkSize       = 8 * 1024 * 1024                           // 8 MiB (spec §2.5/R11)
	dropboxMaxSingleUpload = 150 * 1024 * 1024                         // Dropbox's /2/files/upload hard cap (spec §2.5)
	dropboxHTTPTimeout     = 60 * time.Second
)

// dropboxContentHost / dropboxAPIHost are indirected through package-level
// vars (rather than consts) purely so white-box tests in this package can
// point Upload/Delete/List/Test at a local httptest.Server instead of the
// real Dropbox API — mirrors the ssrfValidateHost test-substitution seam in
// ssrf.go. Production code never reassigns these; tests restore the
// original via t.Cleanup.
var (
	dropboxContentHost = "https://content.dropboxapi.com"
	dropboxAPIHost     = "https://api.dropboxapi.com"
)

// DropboxOAuthConfig returns the oauth2.Config for the Dropbox authorization
// code flow (spec §3.5 Commit 3) — the single source of truth for Dropbox's
// OAuth endpoint shape, used both by newDropboxUploader's own token refresh
// (via NewClient) and by BackupRemoteService's oauth/start + oauth/callback
// handlers, which need the identical Endpoint to build the authorize URL
// and exchange the code.
func DropboxOAuthConfig(appKey, appSecret, redirectURL string) oauth2.Config {
	return oauth2.Config{
		ClientID:     appKey,
		ClientSecret: appSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  dropboxAuthURL,
			TokenURL: dropboxTokenURL,
		},
	}
}

// DropboxAuthCodeOptions returns the extra authorize-URL parameters Dropbox
// requires to actually hand back a refresh token: without
// token_access_type=offline, Dropbox issues a short-lived access token only
// (spec §3.5 "transparent refresh" depends on always having a refresh
// token).
func DropboxAuthCodeOptions() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("token_access_type", "offline")}
}

// dropboxUploader implements Uploader against the Dropbox v2 HTTP API
// (spec §3.5, §2.5 — hand-rolled net/http client, no SDK dependency).
type dropboxUploader struct {
	client     *http.Client
	folderPath string // normalized: "" (root) or "/segment/segment", no trailing slash
}

// newDropboxUploader constructs the Uploader for a dropbox-type target.
// Returns ErrOAuthNotConnected if the target has not yet completed its
// OAuth authorization flow (spec §3.5/§3.9).
func newDropboxUploader(cfg DropboxConfig, secrets RemoteTargetSecrets, tokenSaver TokenSaver) (Uploader, error) {
	if strings.TrimSpace(cfg.AppKey) == "" {
		return nil, fmt.Errorf("dropbox: app_key is required")
	}
	if secrets.OAuthAccessToken == "" {
		return nil, ErrOAuthNotConnected
	}

	tok := &oauth2.Token{
		AccessToken:  secrets.OAuthAccessToken,
		RefreshToken: secrets.OAuthRefreshToken,
		Expiry:       parseOAuthExpiry(secrets.OAuthExpiresAt),
	}
	conf := DropboxOAuthConfig(cfg.AppKey, secrets.OAuthClientSecret, "")
	client := NewClient(context.Background(), conf, tok, tokenSaver)
	client.Timeout = dropboxHTTPTimeout

	return &dropboxUploader{client: client, folderPath: normalizeDropboxFolder(cfg.FolderPath)}, nil
}

// normalizeDropboxFolder returns "" for an unconfigured/root folder path, or
// a leading-slash, no-trailing-slash path otherwise (Dropbox's own
// convention: the root path is the empty string, never "/").
func normalizeDropboxFolder(folderPath string) string {
	p := strings.TrimSpace(folderPath)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimSuffix(p, "/")
}

// remotePath joins the configured folder with a bare filename (spec §3.2's
// SFTP precedent — remoteKey/prefix arguments passed in from
// BackupRemoteService are just the bare filename; the folder scope lives
// entirely in this provider's own nested config).
func (u *dropboxUploader) remotePath(name string) string {
	return u.folderPath + "/" + strings.TrimPrefix(name, "/")
}

func (u *dropboxUploader) Upload(ctx context.Context, localPath, remoteKey string) error {
	f, err := os.Open(localPath) // #nosec G304 -- localPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("dropbox: open local file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("dropbox: stat local file: %w", err)
	}

	remotePath := u.remotePath(remoteKey)
	if info.Size() <= dropboxMaxSingleUpload {
		return u.uploadSingle(ctx, f, remotePath)
	}
	return u.uploadChunked(ctx, f, info.Size(), remotePath)
}

func (u *dropboxUploader) uploadSingle(ctx context.Context, r io.Reader, remotePath string) error {
	argJSON, err := json.Marshal(map[string]string{"path": remotePath, "mode": "overwrite"})
	if err != nil {
		return fmt.Errorf("dropbox: encode upload arg: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dropboxContentHost+"/2/files/upload", r)
	if err != nil {
		return fmt.Errorf("dropbox: build upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(argJSON))

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("dropbox: upload request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return dropboxAPIError("upload", resp)
	}
	return nil
}

// uploadChunked implements Dropbox's upload-session flow (R11): start with
// the first dropboxChunkSize-sized chunk, append_v2 every subsequent
// full chunk, and finish (committing the path) on the last, partial-or-not,
// chunk. Called only when the local file exceeds dropboxMaxSingleUpload, so
// the first chunk read is always non-final.
func (u *dropboxUploader) uploadChunked(ctx context.Context, f *os.File, size int64, remotePath string) error {
	buf := make([]byte, dropboxChunkSize)

	n, err := io.ReadFull(f, buf)
	if err != nil {
		return fmt.Errorf("dropbox: read first chunk: %w", err)
	}
	sessionID, err := u.uploadSessionStart(ctx, buf[:n])
	if err != nil {
		return err
	}
	offset := int64(n)

	for offset < size {
		n, err = io.ReadFull(f, buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("dropbox: read chunk at offset %d: %w", offset, err)
		}
		chunkStart := offset
		offset += int64(n)
		if offset >= size {
			return u.uploadSessionFinish(ctx, sessionID, chunkStart, buf[:n], remotePath)
		}
		if err := u.uploadSessionAppend(ctx, sessionID, chunkStart, buf[:n]); err != nil {
			return err
		}
	}
	return nil
}

func (u *dropboxUploader) uploadSessionStart(ctx context.Context, chunk []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dropboxContentHost+"/2/files/upload_session/start", bytes.NewReader(chunk))
	if err != nil {
		return "", fmt.Errorf("dropbox: build upload_session/start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", "{}")

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dropbox: upload_session/start request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", dropboxAPIError("upload_session/start", resp)
	}

	var result struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("dropbox: decode upload_session/start response: %w", err)
	}
	return result.SessionID, nil
}

func (u *dropboxUploader) uploadSessionAppend(ctx context.Context, sessionID string, offset int64, chunk []byte) error {
	arg, err := json.Marshal(map[string]any{
		"cursor": map[string]any{"session_id": sessionID, "offset": offset},
	})
	if err != nil {
		return fmt.Errorf("dropbox: encode upload_session/append_v2 arg: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dropboxContentHost+"/2/files/upload_session/append_v2", bytes.NewReader(chunk))
	if err != nil {
		return fmt.Errorf("dropbox: build upload_session/append_v2 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(arg))

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("dropbox: upload_session/append_v2 request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return dropboxAPIError("upload_session/append_v2", resp)
	}
	return nil
}

func (u *dropboxUploader) uploadSessionFinish(ctx context.Context, sessionID string, offset int64, chunk []byte, remotePath string) error {
	arg, err := json.Marshal(map[string]any{
		"cursor": map[string]any{"session_id": sessionID, "offset": offset},
		"commit": map[string]any{"path": remotePath, "mode": "overwrite"},
	})
	if err != nil {
		return fmt.Errorf("dropbox: encode upload_session/finish arg: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dropboxContentHost+"/2/files/upload_session/finish", bytes.NewReader(chunk))
	if err != nil {
		return fmt.Errorf("dropbox: build upload_session/finish request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(arg))

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("dropbox: upload_session/finish request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return dropboxAPIError("upload_session/finish", resp)
	}
	return nil
}

func (u *dropboxUploader) Delete(ctx context.Context, remoteKey string) error {
	return u.apiPost(ctx, "/2/files/delete_v2", map[string]any{"path": remoteKey}, nil)
}

// dropboxEntry is a single "entries" element in a list_folder(/continue)
// response. Tag (".tag") distinguishes "file" from "folder"/"deleted".
type dropboxEntry struct {
	Tag            string    `json:".tag"`
	Name           string    `json:"name"`
	PathLower      string    `json:"path_lower"`
	Size           int64     `json:"size"`
	ServerModified time.Time `json:"server_modified"`
}

type dropboxListFolderResult struct {
	Entries []dropboxEntry `json:"entries"`
	Cursor  string         `json:"cursor"`
	HasMore bool           `json:"has_more"`
}

// List enumerates every object under the configured folder, following
// Dropbox's has_more/cursor pagination to completion (R12 — a single-page
// implementation would silently hide retention-pruning candidates once a
// folder exceeds one page). A not-yet-existing folder (path/not_found) is
// treated as an empty listing, mirroring SFTP's os.IsNotExist behavior.
func (u *dropboxUploader) List(ctx context.Context, _ string) ([]RemoteObject, error) {
	var result dropboxListFolderResult
	if err := u.apiPost(ctx, "/2/files/list_folder", map[string]any{
		"path":      u.folderPath,
		"recursive": false,
	}, &result); err != nil {
		if isDropboxPathNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	objects := dropboxEntriesToObjects(result.Entries)
	for result.HasMore {
		var page dropboxListFolderResult
		if err := u.apiPost(ctx, "/2/files/list_folder/continue", map[string]any{"cursor": result.Cursor}, &page); err != nil {
			return nil, err
		}
		objects = append(objects, dropboxEntriesToObjects(page.Entries)...)
		result = page
	}
	return objects, nil
}

func dropboxEntriesToObjects(entries []dropboxEntry) []RemoteObject {
	var objects []RemoteObject
	for _, e := range entries {
		if e.Tag != "file" {
			continue
		}
		objects = append(objects, RemoteObject{
			Key:          e.PathLower,
			Name:         e.Name,
			Size:         e.Size,
			LastModified: e.ServerModified,
		})
	}
	return objects
}

func isDropboxPathNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "path/not_found")
}

func (u *dropboxUploader) Test(ctx context.Context) error {
	return u.apiPost(ctx, "/2/users/get_current_account", nil, nil)
}

// apiPost issues a JSON-body POST against the Dropbox api (not content)
// host, decoding the JSON response into out (if non-nil).
func (u *dropboxUploader) apiPost(ctx context.Context, endpoint string, body any, out any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("dropbox: encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dropboxAPIHost+endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("dropbox: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("dropbox: request %s: %w", endpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return dropboxAPIError(endpoint, resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("dropbox: decode response for %s: %w", endpoint, err)
		}
	}
	return nil
}

// dropboxAPIError formats a non-200 Dropbox response into an error. The
// response body is Dropbox's own error_summary JSON — safe to include in an
// internally-logged/BackupRemoteCopy-recorded error (admin-facing only;
// never echoed into the OAuth callback redirect, which uses only short
// fixed message codes — spec §3.8).
func dropboxAPIError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("dropbox: %s failed: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))
}
