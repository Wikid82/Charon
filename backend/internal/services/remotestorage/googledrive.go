package remotestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// GoogleDriveConfig is the non-secret configuration for a google_drive-type
// RemoteStorageTarget (spec §3.2/§3.5, Commit 3). ClientID is Google's
// non-secret OAuth2 Client ID; the paired Client Secret lives in
// RemoteTargetSecrets.OAuthClientSecret.
type GoogleDriveConfig struct {
	ClientID   string `json:"client_id"`
	FolderPath string `json:"folder_path,omitempty"`
}

const (
	googleDriveAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	googleDriveTokenURL     = "https://oauth2.googleapis.com/token" // #nosec G101 -- public OAuth2 token endpoint URL, not a credential
	googleDriveRootParent   = "root"
	googleDriveHTTPTimeout  = 60 * time.Second
	googleDriveListPageSize = "1000"
)

// googleDriveAPIHost is indirected through a package-level var (rather than
// a const) purely so white-box tests in this package can point
// Upload/Delete/List/Test at a local httptest.Server instead of the real
// Drive API — mirrors the ssrfValidateHost test-substitution seam in
// ssrf.go. Production code never reassigns this; tests restore the
// original via t.Cleanup.
var googleDriveAPIHost = "https://www.googleapis.com"

// errGoogleDriveFolderNotFound is an internal sentinel used by
// resolveFolderChain(create=false) (List's read-only resolution path) to
// signal "a configured folder segment does not exist yet" — List translates
// this into an empty (nil, nil) result rather than an error (R8), mirroring
// SFTP's os.IsNotExist → empty-list behavior.
var errGoogleDriveFolderNotFound = errors.New("google_drive: folder segment not found")

// GoogleDriveOAuthConfig returns the oauth2.Config for the Google Drive
// authorization code flow (spec §3.5 Commit 3) — the single source of truth
// for Google's OAuth endpoint/scope shape, used both by
// newGoogleDriveUploader's own token refresh (via NewClient) and by
// BackupRemoteService's oauth/start + oauth/callback handlers.
func GoogleDriveOAuthConfig(clientID, clientSecret, redirectURL string) oauth2.Config {
	return oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  googleDriveAuthURL,
			TokenURL: googleDriveTokenURL,
		},
		// drive.file scopes access to only the files/folders this app
		// itself creates — least-privilege for a backup-upload use case,
		// no visibility into the user's wider Drive.
		Scopes: []string{"https://www.googleapis.com/auth/drive.file"},
	}
}

// GoogleDriveAuthCodeOptions returns the extra authorize-URL parameters
// needed to reliably get a refresh token back from Google: access_type=offline
// requests one at all, and prompt=consent forces the consent screen (and
// therefore a fresh refresh token) even if the user previously authorized
// this app, avoiding a silent "no refresh_token in response" on reconnect.
func GoogleDriveAuthCodeOptions() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	}
}

// googleDriveUploader implements Uploader against the Google Drive v3 HTTP
// API (spec §3.5, §2.5 — hand-rolled net/http client, no SDK dependency).
type googleDriveUploader struct {
	client     *http.Client
	folderPath string // trimmed of leading/trailing "/", "" means root
}

// newGoogleDriveUploader constructs the Uploader for a google_drive-type
// target. Returns ErrOAuthNotConnected if the target has not yet completed
// its OAuth authorization flow (spec §3.5/§3.9).
func newGoogleDriveUploader(cfg GoogleDriveConfig, secrets RemoteTargetSecrets, tokenSaver TokenSaver) (Uploader, error) {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("google_drive: client_id is required")
	}
	if secrets.OAuthAccessToken == "" {
		return nil, ErrOAuthNotConnected
	}

	tok := &oauth2.Token{
		AccessToken:  secrets.OAuthAccessToken,
		RefreshToken: secrets.OAuthRefreshToken,
		Expiry:       parseOAuthExpiry(secrets.OAuthExpiresAt),
	}
	conf := GoogleDriveOAuthConfig(cfg.ClientID, secrets.OAuthClientSecret, "")
	client := NewClient(context.Background(), conf, tok, tokenSaver)
	client.Timeout = googleDriveHTTPTimeout

	return &googleDriveUploader{client: client, folderPath: strings.Trim(cfg.FolderPath, "/")}, nil
}

// resolveFolderChain walks u.folderPath's "/"-separated segments starting
// from Drive's synthetic "root" parent, returning the leaf folder's file
// ID. When create is true, a missing segment is created (Upload's need,
// R8); when false, a missing segment returns errGoogleDriveFolderNotFound
// (List's need, R8's "not-yet-existing folder is an empty listing, not an
// error" case). Walked fresh on every call — no caching across calls, so a
// folder renamed/deleted out-of-band never causes a stale-ID failure (spec
// §3.5/§3.9).
func (u *googleDriveUploader) resolveFolderChain(ctx context.Context, create bool) (string, error) {
	parent := googleDriveRootParent
	if u.folderPath == "" {
		return parent, nil
	}

	for _, segment := range strings.Split(u.folderPath, "/") {
		if segment == "" {
			continue
		}
		id, err := u.findFolder(ctx, parent, segment)
		if err != nil {
			return "", err
		}
		if id == "" {
			if !create {
				return "", errGoogleDriveFolderNotFound
			}
			id, err = u.createFolder(ctx, parent, segment)
			if err != nil {
				return "", err
			}
		}
		parent = id
	}
	return parent, nil
}

func (u *googleDriveUploader) findFolder(ctx context.Context, parentID, name string) (string, error) {
	q := fmt.Sprintf("'%s' in parents and name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false",
		googleDriveEscapeQueryValue(parentID), googleDriveEscapeQueryValue(name))

	values := url.Values{}
	values.Set("q", q)
	values.Set("fields", "files(id,name)")
	values.Set("spaces", "drive")

	var result googleDriveFileListResponse
	if err := u.apiGet(ctx, "/drive/v3/files?"+values.Encode(), &result); err != nil {
		return "", err
	}
	if len(result.Files) == 0 {
		return "", nil
	}
	return result.Files[0].ID, nil
}

func (u *googleDriveUploader) createFolder(ctx context.Context, parentID, name string) (string, error) {
	body := map[string]any{
		"name":     name,
		"mimeType": "application/vnd.google-apps.folder",
		"parents":  []string{parentID},
	}
	var result googleDriveFile
	if err := u.apiPostJSON(ctx, googleDriveAPIHost+"/drive/v3/files", body, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// googleDriveEscapeQueryValue escapes a value for embedding inside a Drive
// API `q` search expression's single-quoted string literal, per Google's
// documented escaping rules (backslash and single-quote).
func googleDriveEscapeQueryValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func (u *googleDriveUploader) Upload(ctx context.Context, localPath, remoteKey string) error {
	parentID, err := u.resolveFolderChain(ctx, true)
	if err != nil {
		return err
	}

	f, err := os.Open(localPath) // #nosec G304 -- localPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("google_drive: open local file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("google_drive: stat local file: %w", err)
	}

	sessionURI, err := u.startResumableUpload(ctx, remoteKey, parentID, info.Size())
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURI, f)
	if err != nil {
		return fmt.Errorf("google_drive: build resumable upload request: %w", err)
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("google_drive: resumable upload request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return googleDriveAPIError("resumable upload", resp)
	}
	return nil
}

// startResumableUpload issues the resumable-upload initiation call (Google
// recommends this protocol for all file sizes, unlike Dropbox's hard
// per-call cap — spec §2.5) and returns the session URI subsequent PUTs
// target.
func (u *googleDriveUploader) startResumableUpload(ctx context.Context, name, parentID string, size int64) (string, error) {
	metadata := map[string]any{
		"name":    name,
		"parents": []string{parentID},
	}
	bodyJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("google_drive: encode upload metadata: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		googleDriveAPIHost+"/upload/drive/v3/files?uploadType=resumable", bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("google_drive: build resumable session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Type", "application/octet-stream")
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("google_drive: resumable session request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", googleDriveAPIError("upload session start", resp)
	}

	sessionURI := resp.Header.Get("Location")
	if sessionURI == "" {
		return "", fmt.Errorf("google_drive: resumable session response missing Location header")
	}
	return sessionURI, nil
}

func (u *googleDriveUploader) Delete(ctx context.Context, remoteKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, googleDriveAPIHost+"/drive/v3/files/"+url.PathEscape(remoteKey), http.NoBody)
	if err != nil {
		return fmt.Errorf("google_drive: build delete request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("google_drive: delete request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return googleDriveAPIError("delete", resp)
	}
	return nil
}

// googleDriveFile is a single Drive file resource, as returned by
// files.list/files.get/files.create. Size is a string in Drive's own JSON
// schema (int64-safe over the wire), parsed in googleDriveFilesToObjects.
type googleDriveFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         string `json:"size,omitempty"`
	ModifiedTime string `json:"modifiedTime,omitempty"`
}

type googleDriveFileListResponse struct {
	Files         []googleDriveFile `json:"files"`
	NextPageToken string            `json:"nextPageToken,omitempty"`
}

// List enumerates every file directly inside the configured folder,
// following Drive's nextPageToken pagination to completion (R12 — a
// single-page implementation would silently hide retention-pruning
// candidates once a folder exceeds one page). A not-yet-existing folder
// (any segment missing) yields an empty listing, not an error (R8).
func (u *googleDriveUploader) List(ctx context.Context, _ string) ([]RemoteObject, error) {
	leafID, err := u.resolveFolderChain(ctx, false)
	if err != nil {
		if errors.Is(err, errGoogleDriveFolderNotFound) {
			return nil, nil
		}
		return nil, err
	}

	q := fmt.Sprintf("'%s' in parents and trashed=false", googleDriveEscapeQueryValue(leafID))

	var objects []RemoteObject
	pageToken := ""
	for {
		values := url.Values{}
		values.Set("q", q)
		values.Set("fields", "nextPageToken, files(id,name,size,modifiedTime)")
		values.Set("pageSize", googleDriveListPageSize)
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}

		var page googleDriveFileListResponse
		if err := u.apiGet(ctx, "/drive/v3/files?"+values.Encode(), &page); err != nil {
			return nil, err
		}
		objects = append(objects, googleDriveFilesToObjects(page.Files)...)

		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return objects, nil
}

// googleDriveFilesToObjects maps Drive's opaque file ID to RemoteObject.Key
// and the human-readable filename to RemoteObject.Name — the provider where
// Key != Name actually matters (spec §3.2); retention pruning filters
// candidates by Name and deletes by Key.
func googleDriveFilesToObjects(files []googleDriveFile) []RemoteObject {
	var objects []RemoteObject
	for _, file := range files {
		size, _ := strconv.ParseInt(file.Size, 10, 64)
		modified, _ := time.Parse(time.RFC3339, file.ModifiedTime)
		objects = append(objects, RemoteObject{
			Key:          file.ID,
			Name:         file.Name,
			Size:         size,
			LastModified: modified,
		})
	}
	return objects
}

func (u *googleDriveUploader) Test(ctx context.Context) error {
	if _, err := u.resolveFolderChain(ctx, true); err != nil {
		return err
	}
	return u.apiGet(ctx, "/drive/v3/about?fields=user", nil)
}

func (u *googleDriveUploader) apiGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleDriveAPIHost+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("google_drive: build request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("google_drive: request %s: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return googleDriveAPIError(path, resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("google_drive: decode response for %s: %w", path, err)
		}
	}
	return nil
}

func (u *googleDriveUploader) apiPostJSON(ctx context.Context, fullURL string, body any, out any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("google_drive: encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("google_drive: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("google_drive: request %s: %w", fullURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return googleDriveAPIError(fullURL, resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("google_drive: decode response for %s: %w", fullURL, err)
		}
	}
	return nil
}

// googleDriveAPIError formats a non-200 Drive response into an error. The
// response body is Google's own JSON error payload — safe to include in an
// internally-logged/BackupRemoteCopy-recorded error (admin-facing only;
// never echoed into the OAuth callback redirect, which uses only short
// fixed message codes — spec §3.8).
func googleDriveAPIError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("google_drive: %s failed: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))
}
