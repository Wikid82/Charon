// Package remotestorage defines the interface used to upload backup
// archives to off-host destinations (S3-compatible object storage, SFTP).
//
// This file (Commit 2 of Issue #32) lands only the interface, the shared
// RemoteObject type, and the New factory — no behavior. The factory
// currently returns a "not yet implemented" error for every known target
// type so callers get a clear, typed failure rather than a nil-pointer panic
// if wired up prematurely. Concrete implementations (s3.go, sftp.go) land in
// Commit 3, at which point New dispatches to them instead of erroring.
//
// Kept as its own package (rather than living directly in internal/services)
// so the uploader contract can be tested — and later faked — without pulling
// in GORM or the rest of the services package (spec §3.7).
package remotestorage

import (
	"context"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Uploader is implemented by each remote storage backend (S3, SFTP).
type Uploader interface {
	// Upload copies the file at localPath to remoteKey on the remote target.
	Upload(ctx context.Context, localPath, remoteKey string) error
	// Delete removes remoteKey from the remote target.
	Delete(ctx context.Context, remoteKey string) error
	// List enumerates remote objects under prefix, for retention pruning.
	List(ctx context.Context, prefix string) ([]RemoteObject, error)
	// Test performs a cheap connectivity+auth+write probe.
	Test(ctx context.Context) error
}

// RemoteObject describes a single object/file present on a remote target.
type RemoteObject struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// New constructs the Uploader implementation for target.Type ("s3" or
// "sftp"), decrypting/using secrets as needed.
//
// TODO(Commit 3, Issue #32): dispatch to s3.New / sftp.New once those land;
// today every branch returns an explicit "not yet implemented" error so this
// package compiles and is importable by Commit 3 without restructuring.
func New(target *models.RemoteStorageTarget, secrets map[string]string) (Uploader, error) {
	if target == nil {
		return nil, fmt.Errorf("remotestorage: target is required")
	}

	switch target.Type {
	case "s3":
		return nil, fmt.Errorf("remotestorage: s3 uploader not yet implemented")
	case "sftp":
		return nil, fmt.Errorf("remotestorage: sftp uploader not yet implemented")
	default:
		return nil, fmt.Errorf("remotestorage: unknown remote storage target type %q", target.Type)
	}
}
