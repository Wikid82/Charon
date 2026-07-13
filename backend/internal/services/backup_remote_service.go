package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/Wikid82/charon/backend/internal/utils"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// WebDAVConfig is the non-secret configuration for a webdav-type
// RemoteStorageTarget (spec §3.2, Issue #32 Phase 2). Structurally mirrors
// (but is intentionally a separate type from) remotestorage.WebDAVConfig —
// same dual-definition-bridged-by-JSON pattern already used for S3/SFTP
// between this API-facing package and the remotestorage package.
type WebDAVConfig struct {
	URL                string `json:"url"`
	Username           string `json:"username,omitempty"`
	BasePath           string `json:"base_path,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// DropboxConfig is the non-secret configuration for a dropbox-type
// RemoteStorageTarget (spec §3.2). The uploader that consumes this config
// (dropbox.go, remotestorage.New's "dropbox" case) lands in a later commit
// (Issue #32 Phase 2, Commit 3); the config shape is added now so
// RemoteTargetConfig's wire format is stable and additive across commits.
type DropboxConfig struct {
	AppKey     string `json:"app_key"`
	FolderPath string `json:"folder_path,omitempty"`
}

// GoogleDriveConfig is the non-secret configuration for a google_drive-type
// RemoteStorageTarget (spec §3.2). Same "config shape now, uploader later"
// rationale as DropboxConfig above.
type GoogleDriveConfig struct {
	ClientID   string `json:"client_id"`
	FolderPath string `json:"folder_path,omitempty"`
}

// RemoteTargetConfig is the non-secret configuration accepted/returned by
// the remote-targets API (spec §3.3.2). Fields not relevant to Type are
// simply left zero; handlers decode/encode only the fields relevant to a
// target's Type.
type RemoteTargetConfig struct {
	// S3
	Endpoint       string `json:"endpoint,omitempty"`
	Region         string `json:"region,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	PathPrefix     string `json:"path_prefix,omitempty"`
	UseSSL         bool   `json:"use_ssl,omitempty"`
	ForcePathStyle bool   `json:"force_path_style,omitempty"`

	// SFTP
	Host               string `json:"host,omitempty"`
	Port               int    `json:"port,omitempty"`
	Path               string `json:"path,omitempty"`
	Username           string `json:"username,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`

	// WebDAV / Dropbox / Google Drive (spec §3.2, Issue #32 Phase 2) — one
	// nested pointer sub-config per provider, so this struct grows by one
	// field per future provider forever, instead of by every provider's
	// individual settings forever.
	WebDAV      *WebDAVConfig      `json:"webdav,omitempty"`
	Dropbox     *DropboxConfig     `json:"dropbox,omitempty"`
	GoogleDrive *GoogleDriveConfig `json:"google_drive,omitempty"`
}

// RemoteTargetSecrets is the secret blob accepted by the remote-targets API
// and, once merged with any existing secrets, AES-256-GCM-encrypted as a
// whole JSON document into RemoteStorageTarget.SecretsEncrypted. Never
// serialized back to any API response (spec §3.3.2, §3.9).
type RemoteTargetSecrets struct {
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Password        string `json:"password,omitempty"`
	PrivateKeyPEM   string `json:"private_key_pem,omitempty"`
	Passphrase      string `json:"passphrase,omitempty"`

	// WebDAV bearer-auth alternative to Password.
	BearerToken string `json:"bearer_token,omitempty"`
	// Dropbox/Google Drive OAuth2 fields (spec §3.2, Issue #32 Phase 2).
	// OAuthClientSecret is the Dropbox "App secret" / Google Client Secret.
	// OAuthExpiresAt is RFC3339; empty means the OAuth flow hasn't been
	// completed yet. Populated/consumed starting in Commit 3.
	OAuthClientSecret string `json:"oauth_client_secret,omitempty"`
	OAuthAccessToken  string `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken string `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    string `json:"oauth_expires_at,omitempty"`
}

func (s RemoteTargetSecrets) isEmpty() bool {
	return s == RemoteTargetSecrets{}
}

func (s RemoteTargetSecrets) toMap() map[string]string {
	return map[string]string{
		"access_key_id":       s.AccessKeyID,
		"secret_access_key":   s.SecretAccessKey,
		"password":            s.Password,
		"private_key_pem":     s.PrivateKeyPEM,
		"passphrase":          s.Passphrase,
		"bearer_token":        s.BearerToken,
		"oauth_client_secret": s.OAuthClientSecret,
		"oauth_access_token":  s.OAuthAccessToken,
		"oauth_refresh_token": s.OAuthRefreshToken,
		"oauth_expires_at":    s.OAuthExpiresAt,
	}
}

// ErrEncryptionKeyMissing is returned when a remote-target operation needs
// crypto.EncryptionService (to store secrets) but CHARON_ENCRYPTION_KEY is
// unset — mirrors the DNS-provider precedent (routes.go, spec R9).
var ErrEncryptionKeyMissing = fmt.Errorf("encryption key is not configured")

// ErrPublicURLNotConfigured is returned by StartOAuth when the "app.public_url"
// Setting (spec §2.4) has not been configured — without it there is no safe
// base to build an OAuth redirect_uri from (spec R10).
var ErrPublicURLNotConfigured = errors.New("app.public_url setting is not configured")

// oauthProviderTypes lists the RemoteStorageTarget.Type values that go
// through the OAuth2 authorization-code flow (spec §3.5 Commit 3) — s3,
// sftp, and webdav all use static/basic credentials instead.
var oauthProviderTypes = map[string]bool{"dropbox": true, "google_drive": true}

// BackupRemoteService owns remote storage target CRUD, connection testing,
// and the upload/retention orchestration triggered after each backup is
// created (spec §3.7).
type BackupRemoteService struct {
	db         *gorm.DB
	encryption *crypto.EncryptionService
	backupDir  string

	// uploaderFactory defaults to remotestorage.New; overridden by tests in
	// this package (a fake Uploader — required test #10) so upload/retention
	// flow logic can be exercised without a real S3/SFTP endpoint.
	uploaderFactory func(target *models.RemoteStorageTarget, secrets map[string]string, tokenSaver remotestorage.TokenSaver) (remotestorage.Uploader, error)

	// states is the in-memory OAuth CSRF state store (spec §3.4/§3.5) shared
	// by the Dropbox/Google Drive oauth/start + oauth/:provider/callback
	// handlers landing in Commit 3. Constructed once here so its 10-minute
	// TTL entries survive across requests within this process's lifetime.
	states *OAuthStateStore
}

// NewBackupRemoteService constructs a BackupRemoteService. encryption is
// nilable: without CHARON_ENCRYPTION_KEY, remote-target secret storage is
// unavailable and Create/Update return ErrEncryptionKeyMissing (spec R9).
func NewBackupRemoteService(db *gorm.DB, encryption *crypto.EncryptionService, backupDir string) *BackupRemoteService {
	return &BackupRemoteService{
		db:              db,
		encryption:      encryption,
		backupDir:       backupDir,
		uploaderFactory: remotestorage.New,
		states:          NewOAuthStateStore(),
	}
}

// reconcileStuckUploadingCopies transitions any BackupRemoteCopy rows left
// in status "uploading" from a prior process (crash, OOM-kill, forced
// shutdown — no graceful Stop() ran) to "failed" once, so the UI never shows
// a permanently-stuck "uploading" row (spec §3.7). Called from
// NewBackupService.
func reconcileStuckUploadingCopies(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Model(&models.BackupRemoteCopy{}).
		Where("status = ?", "uploading").
		Update("status", "failed").Error; err != nil {
		return fmt.Errorf("reconcile stuck remote upload copies: %w", err)
	}
	return nil
}

// List returns every configured remote target (secrets never populated).
func (s *BackupRemoteService) List() ([]models.RemoteStorageTarget, error) {
	var targets []models.RemoteStorageTarget
	if err := s.db.Order("created_at asc").Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("list remote storage targets: %w", err)
	}
	return targets, nil
}

// Get returns a single remote target by UUID.
func (s *BackupRemoteService) Get(uuid string) (*models.RemoteStorageTarget, error) {
	var target models.RemoteStorageTarget
	if err := s.db.Where("uuid = ?", uuid).First(&target).Error; err != nil {
		return nil, fmt.Errorf("get remote storage target: %w", err)
	}
	return &target, nil
}

// Create persists a new remote storage target. Config is validated against
// the SSRF policy (spec §3.7) before the target is ever saved.
func (s *BackupRemoteService) Create(name, targetType string, enabled bool, config RemoteTargetConfig, secrets RemoteTargetSecrets) (*models.RemoteStorageTarget, error) {
	if s.encryption == nil {
		return nil, ErrEncryptionKeyMissing
	}

	if err := validateRemoteTargetConfig(targetType, config); err != nil {
		return nil, err
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal remote target config: %w", err)
	}

	secretsJSON, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("marshal remote target secrets: %w", err)
	}
	encryptedSecrets, err := s.encryption.Encrypt(secretsJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypt remote target secrets: %w", err)
	}

	target := &models.RemoteStorageTarget{
		Name:             name,
		Type:             targetType,
		Enabled:          enabled,
		ConfigJSON:       string(configJSON),
		SecretsEncrypted: encryptedSecrets,
		KeyVersion:       1,
		LastTestStatus:   "never",
	}
	if oauthProviderTypes[targetType] {
		// R2: created in a "pending" state — no access/refresh token exists
		// yet, and none is required at creation time.
		target.OAuthStatus = "not_connected"
	}

	if err := s.db.Create(target).Error; err != nil {
		return nil, fmt.Errorf("create remote storage target: %w", err)
	}

	// GORM parses the `default:true` tag on Enabled into a schema-level
	// DefaultValueInterface and, on Create, unconditionally substitutes that
	// value for ANY zero Go value (false) before the INSERT is built — this
	// happens during struct-to-values conversion itself, so neither
	// Select("*") nor Omit() can suppress it (verified empirically against
	// the installed gorm.io/gorm version). A caller-supplied enabled:false
	// is therefore silently upgraded to true by Create alone. Update has no
	// equivalent substitution logic, so force the real value with a
	// follow-up statement whenever the caller asked for false.
	if !enabled {
		if err := s.db.Model(target).Update("enabled", false).Error; err != nil {
			return nil, fmt.Errorf("persist enabled=false for remote storage target: %w", err)
		}
	}

	return target, nil
}

// Update applies a partial update to an existing target. Any nil pointer
// field is left unchanged; an empty (zero-value) RemoteTargetSecrets means
// "keep the existing secrets" (spec §3.3.2 — "secret fields optional —
// omitted ⇒ keep existing").
func (s *BackupRemoteService) Update(uuidStr string, name *string, enabled *bool, config *RemoteTargetConfig, secrets *RemoteTargetSecrets) (*models.RemoteStorageTarget, error) {
	target, err := s.Get(uuidStr)
	if err != nil {
		return nil, err
	}

	if name != nil {
		target.Name = *name
	}
	if enabled != nil {
		target.Enabled = *enabled
	}
	if config != nil {
		if err := validateRemoteTargetConfig(target.Type, *config); err != nil {
			return nil, err
		}
		configJSON, err := json.Marshal(*config)
		if err != nil {
			return nil, fmt.Errorf("marshal remote target config: %w", err)
		}
		target.ConfigJSON = string(configJSON)
	}
	if secrets != nil && !secrets.isEmpty() {
		if s.encryption == nil {
			return nil, ErrEncryptionKeyMissing
		}
		secretsJSON, err := json.Marshal(*secrets)
		if err != nil {
			return nil, fmt.Errorf("marshal remote target secrets: %w", err)
		}
		encryptedSecrets, err := s.encryption.Encrypt(secretsJSON)
		if err != nil {
			return nil, fmt.Errorf("encrypt remote target secrets: %w", err)
		}
		target.SecretsEncrypted = encryptedSecrets
	}

	if err := s.db.Save(target).Error; err != nil {
		return nil, fmt.Errorf("update remote storage target: %w", err)
	}

	return target, nil
}

// Delete removes a remote target.
func (s *BackupRemoteService) Delete(uuidStr string) error {
	if err := s.db.Where("uuid = ?", uuidStr).Delete(&models.RemoteStorageTarget{}).Error; err != nil {
		return fmt.Errorf("delete remote storage target: %w", err)
	}
	return nil
}

// decryptSecrets decrypts and unmarshals target.SecretsEncrypted.
func (s *BackupRemoteService) decryptSecrets(target *models.RemoteStorageTarget) (RemoteTargetSecrets, error) {
	var secrets RemoteTargetSecrets
	if target.SecretsEncrypted == "" {
		return secrets, nil
	}
	if s.encryption == nil {
		return secrets, ErrEncryptionKeyMissing
	}
	raw, err := s.encryption.Decrypt(target.SecretsEncrypted)
	if err != nil {
		return secrets, fmt.Errorf("decrypt remote target secrets: %w", err)
	}
	if err := json.Unmarshal(raw, &secrets); err != nil {
		return secrets, fmt.Errorf("parse decrypted remote target secrets: %w", err)
	}
	return secrets, nil
}

// uploaderFor builds a remotestorage.Uploader for target, decrypting its
// secrets first. A remoteTargetTokenSaver bound to this specific target is
// always passed through — s3/sftp/webdav simply ignore it (spec §3.5
// Commit 3); only dropbox/google_drive invoke SaveToken, when a transparent
// mid-flight refresh occurs.
func (s *BackupRemoteService) uploaderFor(target *models.RemoteStorageTarget) (remotestorage.Uploader, error) {
	secrets, err := s.decryptSecrets(target)
	if err != nil {
		return nil, err
	}
	factory := s.uploaderFactory
	if factory == nil {
		factory = remotestorage.New
	}
	uploader, err := factory(target, secrets.toMap(), &remoteTargetTokenSaver{svc: s, target: target})
	if err != nil {
		return nil, fmt.Errorf("construct uploader for target %q: %w", target.Name, err)
	}
	return uploader, nil
}

// remoteTargetTokenSaver implements remotestorage.TokenSaver for a single
// target, persisting a transparently-refreshed OAuth2 token back into that
// target's encrypted secrets blob (spec R5/§3.5 Commit 3). Constructed fresh
// per uploaderFor call — cheap, and avoids holding a stale *target pointer
// across requests.
type remoteTargetTokenSaver struct {
	svc    *BackupRemoteService
	target *models.RemoteStorageTarget
}

// SaveToken implements remotestorage.TokenSaver.
func (t *remoteTargetTokenSaver) SaveToken(_ context.Context, accessToken, refreshToken string, expiresAt time.Time) error {
	if t.svc.encryption == nil {
		return ErrEncryptionKeyMissing
	}

	secrets, err := t.svc.decryptSecrets(t.target)
	if err != nil {
		return err
	}
	secrets.OAuthAccessToken = accessToken
	secrets.OAuthRefreshToken = refreshToken
	secrets.OAuthExpiresAt = expiresAt.UTC().Format(time.RFC3339)

	secretsJSON, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("marshal refreshed oauth secrets: %w", err)
	}
	encrypted, err := t.svc.encryption.Encrypt(secretsJSON)
	if err != nil {
		return fmt.Errorf("encrypt refreshed oauth secrets: %w", err)
	}

	if err := t.svc.db.Model(&models.RemoteStorageTarget{}).
		Where("uuid = ?", t.target.UUID).
		Update("secrets_encrypted", encrypted).Error; err != nil {
		return fmt.Errorf("persist refreshed oauth token: %w", err)
	}
	t.target.SecretsEncrypted = encrypted
	return nil
}

// Test performs the connectivity+auth+write probe for a target (spec
// §3.3.2, §3.7) and records the outcome on the target row.
func (s *BackupRemoteService) Test(ctx context.Context, uuidStr string) error {
	target, err := s.Get(uuidStr)
	if err != nil {
		return err
	}

	uploader, err := s.uploaderFor(target)
	if err != nil {
		s.recordTestOutcome(target, err)
		return err
	}

	testErr := uploader.Test(ctx)
	s.recordTestOutcome(target, testErr)
	return testErr
}

func (s *BackupRemoteService) recordTestOutcome(target *models.RemoteStorageTarget, testErr error) {
	now := time.Now().UTC()
	target.LastTestAt = &now
	if testErr != nil {
		target.LastTestStatus = "failed"
		target.LastError = testErr.Error()
	} else {
		target.LastTestStatus = "ok"
		target.LastError = ""
	}
	if err := s.db.Save(target).Error; err != nil {
		logger.Log().WithError(err).WithField("target", util.SanitizeForLog(target.Name)).
			Warn("failed to record remote target test outcome")
	}
}

// validateRemoteTargetConfig enforces required fields per target type and
// the SSRF policy (spec §3.7) against the resolved host/endpoint.
func validateRemoteTargetConfig(targetType string, config RemoteTargetConfig) error {
	switch targetType {
	case "s3":
		if strings.TrimSpace(config.Endpoint) == "" {
			return fmt.Errorf("s3 endpoint is required")
		}
		if strings.TrimSpace(config.Bucket) == "" {
			return fmt.Errorf("s3 bucket is required")
		}
		host := config.Endpoint
		if idx := strings.LastIndex(host, ":"); idx != -1 && !strings.Contains(host, "]") {
			host = host[:idx]
		}
		if err := remotestorage.ValidateHostSSRF(host); err != nil {
			return fmt.Errorf("s3 endpoint failed SSRF validation: %w", err)
		}
	case "sftp":
		if strings.TrimSpace(config.Host) == "" {
			return fmt.Errorf("sftp host is required")
		}
		if err := remotestorage.ValidateHostSSRF(config.Host); err != nil {
			return fmt.Errorf("sftp host failed SSRF validation: %w", err)
		}
	case "webdav":
		if config.WebDAV == nil || strings.TrimSpace(config.WebDAV.URL) == "" {
			return fmt.Errorf("webdav url is required")
		}
		parsedURL, err := url.Parse(config.WebDAV.URL)
		if err != nil {
			return fmt.Errorf("webdav: invalid url: %w", err)
		}
		host := parsedURL.Hostname()
		if host == "" {
			return fmt.Errorf("webdav: url must include a host")
		}
		if err := remotestorage.ValidateHostSSRF(host); err != nil {
			return fmt.Errorf("webdav url failed SSRF validation: %w", err)
		}
	case "dropbox":
		// No SSRF check — Dropbox's uploader only ever dials fixed vendor
		// hosts (content.dropboxapi.com/api.dropboxapi.com), never a
		// user-supplied value (spec §3.8).
		if config.Dropbox == nil || strings.TrimSpace(config.Dropbox.AppKey) == "" {
			return fmt.Errorf("dropbox app_key is required")
		}
	case "google_drive":
		// No SSRF check — same rationale as dropbox above (fixed
		// www.googleapis.com host).
		if config.GoogleDrive == nil || strings.TrimSpace(config.GoogleDrive.ClientID) == "" {
			return fmt.Errorf("google_drive client_id is required")
		}
	default:
		return fmt.Errorf("unknown remote storage target type %q", targetType)
	}
	return nil
}

// TriggerUpload is the RemoteUploadHook wired into BackupService (spec
// §3.7): for every enabled target, it uploads the newly-created backup and
// prunes remote objects beyond the configured retention count. Upload
// failures never fail the backup itself — they are recorded on the
// BackupRemoteCopy row only.
func (s *BackupRemoteService) TriggerUpload(ctx context.Context, record *models.BackupRecord) {
	if s.db == nil || record == nil {
		return
	}

	var targets []models.RemoteStorageTarget
	if err := s.db.Where("enabled = ?", true).Find(&targets).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to list enabled remote storage targets for upload")
		return
	}

	retentionCount := readBackupSettingInt(s.db, SettingKeyBackupRemoteRetentionCount, defaultRemoteRetentionCount)

	for i := range targets {
		target := targets[i]
		s.uploadToTarget(ctx, record, &target, retentionCount)
	}
}

func (s *BackupRemoteService) uploadToTarget(ctx context.Context, record *models.BackupRecord, target *models.RemoteStorageTarget, retentionCount int) {
	remoteCopy := models.BackupRemoteCopy{
		BackupRecordID: record.ID,
		RemoteTargetID: target.ID,
		Status:         "pending",
	}
	if err := s.db.Create(&remoteCopy).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to create backup remote copy row")
		return
	}

	uploader, err := s.uploaderFor(target)
	if err != nil {
		s.failCopy(&remoteCopy, err)
		return
	}

	var config RemoteTargetConfig
	_ = json.Unmarshal([]byte(target.ConfigJSON), &config)
	remoteKey := joinRemotePrefix(config.PathPrefix, record.Filename)

	remoteCopy.Status = "uploading"
	remoteCopy.RemoteKey = remoteKey
	if err := s.db.Save(&remoteCopy).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to mark backup remote copy as uploading")
	}

	localPath := path.Join(s.backupDir, record.Filename)
	if err := uploader.Upload(ctx, localPath, remoteKey); err != nil {
		s.failCopy(&remoteCopy, err)
		return
	}

	now := time.Now().UTC()
	remoteCopy.Status = "uploaded"
	remoteCopy.UploadedAt = &now
	remoteCopy.ErrorMessage = ""
	if err := s.db.Save(&remoteCopy).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to mark backup remote copy as uploaded")
	}

	s.pruneRemoteRetention(ctx, uploader, config.PathPrefix, retentionCount)
}

func (s *BackupRemoteService) failCopy(remoteCopy *models.BackupRemoteCopy, uploadErr error) {
	remoteCopy.Status = "failed"
	remoteCopy.ErrorMessage = uploadErr.Error()
	if err := s.db.Save(remoteCopy).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to record backup remote copy failure")
	}
}

// pruneRemoteRetention deletes remote objects matching backup_*.zip* beyond
// retentionCount, newest by LastModified first — only Charon-named keys are
// ever considered (spec §3.7).
func (s *BackupRemoteService) pruneRemoteRetention(ctx context.Context, uploader remotestorage.Uploader, pathPrefix string, retentionCount int) {
	if retentionCount < 1 {
		retentionCount = defaultRemoteRetentionCount
	}

	objects, err := uploader.List(ctx, pathPrefix)
	if err != nil {
		logger.Log().WithError(err).Warn("failed to list remote objects for retention pruning")
		return
	}

	var candidates []remotestorage.RemoteObject
	for _, obj := range objects {
		// Filter on Name (human-readable filename), never Key — Key is an
		// opaque, provider-native locator (e.g. a Google Drive file ID,
		// Commit 3) that may have no relationship to the filename at all.
		// See RemoteObject's doc comment (spec §3.2, Issue #32 Phase 2).
		base := obj.Name
		if strings.HasPrefix(base, "backup_") && strings.Contains(base, ".zip") {
			candidates = append(candidates, obj)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastModified.After(candidates[j].LastModified)
	})

	if len(candidates) <= retentionCount {
		return
	}

	for _, obj := range candidates[retentionCount:] {
		if err := uploader.Delete(ctx, obj.Key); err != nil {
			logger.Log().WithError(err).WithField("key", util.SanitizeForLog(obj.Key)).
				Warn("failed to prune old remote backup copy")
		}
	}
}

func joinRemotePrefix(prefix, filename string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return filename
	}
	return prefix + "/" + filename
}

// --- OAuth2 authorization-code flow (dropbox/google_drive only, Commit 3) ---

// oauthCallbackPath is appended to the configured public URL to build the
// redirect_uri passed to both the authorize-URL (StartOAuth) and the
// token-exchange call (CompleteOAuth) — these two MUST be byte-identical,
// as OAuth2 requires (spec §3.3).
func oauthCallbackPath(provider string) string {
	return "/api/v1/backups/remote-targets/oauth/" + provider + "/callback"
}

// ConfiguredPublicURL exposes utils.GetConfiguredPublicURL(db) to handlers
// without giving BackupRemoteHandler its own *gorm.DB (spec §2.4) — used by
// the OAuth callback route to build its redirect target.
func (s *BackupRemoteService) ConfiguredPublicURL() (string, bool) {
	return utils.GetConfiguredPublicURL(s.db)
}

// ConsumeOAuthState is a thin passthrough to the service's in-memory
// OAuthStateStore (spec R3/§3.5), kept here so the handler never needs
// direct access to the store itself.
func (s *BackupRemoteService) ConsumeOAuthState(state string) (targetUUID, provider string, ok bool) {
	return s.states.Consume(state)
}

// StartOAuth issues a fresh CSRF state token and builds the provider's
// authorize URL for target uuidStr (spec §3.3/§3.5). Returns
// ErrPublicURLNotConfigured (→ 400 public_url_not_configured, R10) when the
// "app.public_url" Setting has not been configured — there is no safe
// redirect_uri base to build without it.
func (s *BackupRemoteService) StartOAuth(uuidStr string) (authorizeURL string, err error) {
	target, err := s.Get(uuidStr)
	if err != nil {
		return "", err
	}
	if !oauthProviderTypes[target.Type] {
		return "", fmt.Errorf("oauth is only supported for dropbox/google_drive targets, got %q", target.Type)
	}

	baseURL, ok := utils.GetConfiguredPublicURL(s.db)
	if !ok {
		return "", ErrPublicURLNotConfigured
	}

	var config RemoteTargetConfig
	if unmarshalErr := json.Unmarshal([]byte(target.ConfigJSON), &config); unmarshalErr != nil {
		return "", fmt.Errorf("parse remote target config: %w", unmarshalErr)
	}

	state, err := s.states.Issue(target.UUID, target.Type)
	if err != nil {
		return "", fmt.Errorf("issue oauth state: %w", err)
	}

	redirectURL := strings.TrimSuffix(baseURL, "/") + oauthCallbackPath(target.Type)

	switch target.Type {
	case "dropbox":
		if config.Dropbox == nil {
			return "", fmt.Errorf("dropbox config missing")
		}
		conf := remotestorage.DropboxOAuthConfig(config.Dropbox.AppKey, "", redirectURL)
		return conf.AuthCodeURL(state, remotestorage.DropboxAuthCodeOptions()...), nil
	case "google_drive":
		if config.GoogleDrive == nil {
			return "", fmt.Errorf("google_drive config missing")
		}
		conf := remotestorage.GoogleDriveOAuthConfig(config.GoogleDrive.ClientID, "", redirectURL)
		return conf.AuthCodeURL(state, remotestorage.GoogleDriveAuthCodeOptions()...), nil
	default:
		return "", fmt.Errorf("unsupported oauth provider %q", target.Type)
	}
}

// CompleteOAuth exchanges code for an access+refresh token and persists it
// (spec R4). baseURL must be the exact same configured public URL value
// StartOAuth used to build the authorize URL's redirect_uri (the handler
// passes the same ConfiguredPublicURL() call site for both).
func (s *BackupRemoteService) CompleteOAuth(ctx context.Context, targetUUID, provider, code, baseURL string) error {
	target, err := s.Get(targetUUID)
	if err != nil {
		return err
	}
	if target.Type != provider {
		return fmt.Errorf("oauth callback provider %q does not match target type %q", provider, target.Type)
	}
	if s.encryption == nil {
		return ErrEncryptionKeyMissing
	}

	var config RemoteTargetConfig
	if unmarshalErr := json.Unmarshal([]byte(target.ConfigJSON), &config); unmarshalErr != nil {
		return fmt.Errorf("parse remote target config: %w", unmarshalErr)
	}
	secrets, err := s.decryptSecrets(target)
	if err != nil {
		return err
	}

	redirectURL := strings.TrimSuffix(baseURL, "/") + oauthCallbackPath(provider)

	var tok *oauth2.Token
	switch provider {
	case "dropbox":
		if config.Dropbox == nil {
			return fmt.Errorf("dropbox config missing")
		}
		conf := remotestorage.DropboxOAuthConfig(config.Dropbox.AppKey, secrets.OAuthClientSecret, redirectURL)
		tok, err = conf.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("dropbox: exchange oauth code: %w", err)
		}
	case "google_drive":
		if config.GoogleDrive == nil {
			return fmt.Errorf("google_drive config missing")
		}
		conf := remotestorage.GoogleDriveOAuthConfig(config.GoogleDrive.ClientID, secrets.OAuthClientSecret, redirectURL)
		tok, err = conf.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("google_drive: exchange oauth code: %w", err)
		}
	default:
		return fmt.Errorf("unsupported oauth provider %q", provider)
	}

	secrets.OAuthAccessToken = tok.AccessToken
	secrets.OAuthRefreshToken = tok.RefreshToken
	secrets.OAuthExpiresAt = tok.Expiry.UTC().Format(time.RFC3339)

	secretsJSON, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("marshal oauth secrets: %w", err)
	}
	encrypted, err := s.encryption.Encrypt(secretsJSON)
	if err != nil {
		return fmt.Errorf("encrypt oauth secrets: %w", err)
	}

	now := time.Now().UTC()
	target.SecretsEncrypted = encrypted
	target.OAuthStatus = "connected"
	target.OAuthConnectedAt = &now
	if err := s.db.Save(target).Error; err != nil {
		return fmt.Errorf("persist oauth token: %w", err)
	}
	return nil
}

// DisconnectOAuth clears a target's OAuth token fields and resets its
// oauth_status to "not_connected" (spec §3.3) without deleting the target
// itself — disconnecting is a deliberate, separate action from deleting,
// matching the existing "leave blank to keep current" secret-update
// philosophy.
func (s *BackupRemoteService) DisconnectOAuth(uuidStr string) (*models.RemoteStorageTarget, error) {
	target, err := s.Get(uuidStr)
	if err != nil {
		return nil, err
	}
	if !oauthProviderTypes[target.Type] {
		return nil, fmt.Errorf("oauth disconnect is only supported for dropbox/google_drive targets, got %q", target.Type)
	}

	if target.SecretsEncrypted != "" {
		if s.encryption == nil {
			return nil, ErrEncryptionKeyMissing
		}
		secrets, err := s.decryptSecrets(target)
		if err != nil {
			return nil, err
		}
		secrets.OAuthAccessToken = ""
		secrets.OAuthRefreshToken = ""
		secrets.OAuthExpiresAt = ""

		secretsJSON, err := json.Marshal(secrets)
		if err != nil {
			return nil, fmt.Errorf("marshal oauth secrets: %w", err)
		}
		encrypted, err := s.encryption.Encrypt(secretsJSON)
		if err != nil {
			return nil, fmt.Errorf("encrypt oauth secrets: %w", err)
		}
		target.SecretsEncrypted = encrypted
	}

	target.OAuthStatus = "not_connected"
	target.OAuthConnectedAt = nil
	if err := s.db.Save(target).Error; err != nil {
		return nil, fmt.Errorf("persist oauth disconnect: %w", err)
	}
	return target, nil
}
