package remotestorage

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config is the non-secret configuration for an s3-type
// RemoteStorageTarget (spec §3.7).
type S3Config struct {
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	PathPrefix     string `json:"path_prefix"`
	UseSSL         bool   `json:"use_ssl"`
	ForcePathStyle bool   `json:"force_path_style"`
}

// S3Secrets is the secret credential pair for an s3-type target.
type S3Secrets struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// s3ConnectionTestKey is the marker object written/deleted by Test (spec
// §3.7 — "Test = BucketExists + put/delete a charon-connection-test marker
// object").
const s3ConnectionTestKey = "charon-connection-test"

// s3Uploader implements Uploader for S3-compatible object storage via
// minio-go/v7 (single module, first-class support for the S3-compatible
// endpoints self-hosters actually use: MinIO, Backblaze B2, Cloudflare R2).
type s3Uploader struct {
	client *minio.Client
	bucket string
}

// newS3Uploader constructs the Uploader for an s3-type target. The
// underlying HTTP transport dials through the SSRF-safe dialer (spec §3.7)
// so every connection — not just the one made when the endpoint was first
// saved — is re-validated at dial time.
func newS3Uploader(cfg S3Config, secrets S3Secrets) (Uploader, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("s3: endpoint is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}

	host := cfg.Endpoint
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	// ssrfValidateHost defaults to ValidateHostSSRF; see ssrf.go for why it
	// is indirected through a var.
	if err := ssrfValidateHost(host); err != nil {
		return nil, fmt.Errorf("s3 endpoint failed SSRF validation: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, dialNetwork, addr string) (net.Conn, error) {
			return dialContext(ctx, dialNetwork, addr, 10*time.Second)
		},
	}

	bucketLookup := minio.BucketLookupAuto
	if cfg.ForcePathStyle {
		bucketLookup = minio.BucketLookupPath
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(secrets.AccessKeyID, secrets.SecretAccessKey, ""),
		Secure:       cfg.UseSSL,
		Region:       cfg.Region,
		Transport:    transport,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: create client: %w", err)
	}

	return &s3Uploader{client: client, bucket: cfg.Bucket}, nil
}

func (u *s3Uploader) Upload(ctx context.Context, localPath, remoteKey string) error {
	f, err := os.Open(localPath) // #nosec G304 -- localPath is a server-controlled backup archive path
	if err != nil {
		return fmt.Errorf("s3: open local file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("s3: stat local file: %w", err)
	}

	if _, err := u.client.PutObject(ctx, u.bucket, remoteKey, f, info.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		return fmt.Errorf("s3: put object: %w", err)
	}
	return nil
}

func (u *s3Uploader) Delete(ctx context.Context, remoteKey string) error {
	if err := u.client.RemoveObject(ctx, u.bucket, remoteKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3: remove object: %w", err)
	}
	return nil
}

func (u *s3Uploader) List(ctx context.Context, prefix string) ([]RemoteObject, error) {
	var objects []RemoteObject
	for obj := range u.client.ListObjects(ctx, u.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("s3: list objects: %w", obj.Err)
		}
		// Name is the human-readable basename, always populated so
		// retention-candidate filtering (BackupRemoteService.pruneRemoteRetention)
		// keeps matching "backup_*.zip*" — see RemoteObject's doc comment
		// (spec §3.2, Issue #32 Phase 2) for why this must never be left
		// empty for an existing, already-shipped provider.
		objects = append(objects, RemoteObject{Key: obj.Key, Name: path.Base(obj.Key), Size: obj.Size, LastModified: obj.LastModified})
	}
	return objects, nil
}

func (u *s3Uploader) Test(ctx context.Context) error {
	exists, err := u.client.BucketExists(ctx, u.bucket)
	if err != nil {
		return fmt.Errorf("s3: check bucket exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("s3: bucket %q does not exist or is not accessible", u.bucket)
	}

	marker := strings.NewReader("charon connection test")
	if _, err := u.client.PutObject(ctx, u.bucket, s3ConnectionTestKey, marker, marker.Size(), minio.PutObjectOptions{
		ContentType: "text/plain",
	}); err != nil {
		return fmt.Errorf("s3: write connection test marker: %w", err)
	}
	if err := u.client.RemoveObject(ctx, u.bucket, s3ConnectionTestKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3: remove connection test marker: %w", err)
	}
	return nil
}
