package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services/remotestorage"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

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
}

func (s RemoteTargetSecrets) isEmpty() bool {
	return s == RemoteTargetSecrets{}
}

func (s RemoteTargetSecrets) toMap() map[string]string {
	return map[string]string{
		"access_key_id":     s.AccessKeyID,
		"secret_access_key": s.SecretAccessKey,
		"password":          s.Password,
		"private_key_pem":   s.PrivateKeyPEM,
		"passphrase":        s.Passphrase,
	}
}

// ErrEncryptionKeyMissing is returned when a remote-target operation needs
// crypto.EncryptionService (to store secrets) but CHARON_ENCRYPTION_KEY is
// unset — mirrors the DNS-provider precedent (routes.go, spec R9).
var ErrEncryptionKeyMissing = fmt.Errorf("encryption key is not configured")

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
	uploaderFactory func(target *models.RemoteStorageTarget, secrets map[string]string) (remotestorage.Uploader, error)
}

// NewBackupRemoteService constructs a BackupRemoteService. encryption is
// nilable: without CHARON_ENCRYPTION_KEY, remote-target secret storage is
// unavailable and Create/Update return ErrEncryptionKeyMissing (spec R9).
func NewBackupRemoteService(db *gorm.DB, encryption *crypto.EncryptionService, backupDir string) *BackupRemoteService {
	return &BackupRemoteService{db: db, encryption: encryption, backupDir: backupDir, uploaderFactory: remotestorage.New}
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

	if err := s.db.Create(target).Error; err != nil {
		return nil, fmt.Errorf("create remote storage target: %w", err)
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
// secrets first.
func (s *BackupRemoteService) uploaderFor(target *models.RemoteStorageTarget) (remotestorage.Uploader, error) {
	secrets, err := s.decryptSecrets(target)
	if err != nil {
		return nil, err
	}
	factory := s.uploaderFactory
	if factory == nil {
		factory = remotestorage.New
	}
	uploader, err := factory(target, secrets.toMap())
	if err != nil {
		return nil, fmt.Errorf("construct uploader for target %q: %w", target.Name, err)
	}
	return uploader, nil
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
		base := path.Base(obj.Key)
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
