package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for CrowdSecWhitelistService operations.
var (
	ErrWhitelistNotFound = errors.New("whitelist entry not found")
	ErrInvalidIPOrCIDR   = errors.New("invalid IP address or CIDR notation")
	ErrDuplicateEntry    = errors.New("entry already exists in whitelist")
)

const whitelistYAMLHeader = `name: charon-whitelist
description: "Charon-managed IP/CIDR whitelist"
filter: "evt.Meta.service == 'http'"
whitelist:
  reason: "Charon managed whitelist"
`

// whitelistPlaceholderYAML is the file written while the allow list is empty. CrowdSec rejects a whitelist parser
// whose ip and cidr lists are both empty ("Node is empty"), so the placeholder carries an expression that is never
// true instead: it matches no event. The file is never removed, because removing it while the engine runs can kill
// the engine on its next reload.
const whitelistPlaceholderYAML = `name: charon-whitelist
description: "Charon-managed IP/CIDR whitelist"
filter: "evt.Meta.service == 'http'"
whitelist:
  reason: "Charon managed whitelist (no entries)"
  expression:
    - "false"
`

// whitelistQueryTimeout bounds the entries query of one file write. The query runs detached from the request
// context (see whitelistLists), so it needs a limit of its own.
const whitelistQueryTimeout = 5 * time.Second

// whitelistStaleTempAge is how old a leftover temporary file must be before a later write removes it.
const whitelistStaleTempAge = 10 * time.Minute

// writeWhitelistFile replaces the whitelist file atomically. It is a seam so tests can observe the write options and
// force a failure.
var writeWhitelistFile = crowdsec.WriteFileAtomic

// removeLegacyTemp deletes the legacy temp file (see removeLegacyWhitelistTemp). It is a seam so tests can force a
// failure.
var removeLegacyTemp = os.Remove

// whitelistInPlaceWarned makes the in-place warning fire once per process; logWhitelistInPlace logs it. Both are
// seams for the tests.
var (
	whitelistInPlaceWarned atomic.Bool
	logWhitelistInPlace    = func() {
		logger.Log().Warn("CrowdSec whitelist file was written in place because its rename was refused. " +
			"A crash during that write can leave the file partly written; the engine reads it at its next reload.")
	}
)

// CrowdSecWhitelistService manages the CrowdSec IP/CIDR whitelist.
type CrowdSecWhitelistService struct {
	db      *gorm.DB
	dataDir string
	// writeMu serialises WriteYAML, so one change's entries query and file write finish before the next change's
	// query starts. The file then always ends up with the last committed list.
	writeMu sync.Mutex
}

// NewCrowdSecWhitelistService creates a new CrowdSecWhitelistService.
func NewCrowdSecWhitelistService(db *gorm.DB, dataDir string) *CrowdSecWhitelistService {
	return &CrowdSecWhitelistService{db: db, dataDir: dataDir}
}

// List returns all whitelist entries ordered by creation time.
func (s *CrowdSecWhitelistService) List(ctx context.Context) ([]models.CrowdSecWhitelist, error) {
	var entries []models.CrowdSecWhitelist
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list whitelist entries: %w", err)
	}
	return entries, nil
}

// Add validates and persists a new whitelist entry, then regenerates the YAML file.
// Returns ErrInvalidIPOrCIDR for malformed input and ErrDuplicateEntry for conflicts.
func (s *CrowdSecWhitelistService) Add(ctx context.Context, ipOrCIDR, reason string) (*models.CrowdSecWhitelist, error) {
	normalized, err := normalizeIPOrCIDR(strings.TrimSpace(ipOrCIDR))
	if err != nil {
		return nil, ErrInvalidIPOrCIDR
	}

	entry := models.CrowdSecWhitelist{
		UUID:     uuid.New().String(),
		IPOrCIDR: normalized,
		Reason:   reason,
	}

	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateEntry
		}
		return nil, fmt.Errorf("add whitelist entry: %w", err)
	}

	if err := s.WriteYAML(ctx); err != nil {
		logger.Log().WithError(err).Warn("failed to write CrowdSec whitelist YAML after add (non-fatal)")
	}

	return &entry, nil
}

// Delete removes a whitelist entry by UUID and regenerates the YAML file.
// Returns ErrWhitelistNotFound if the UUID does not exist.
func (s *CrowdSecWhitelistService) Delete(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Where("uuid = ?", id).Delete(&models.CrowdSecWhitelist{})
	if result.Error != nil {
		return fmt.Errorf("delete whitelist entry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWhitelistNotFound
	}

	if err := s.WriteYAML(ctx); err != nil {
		logger.Log().WithError(err).Warn("failed to write CrowdSec whitelist YAML after delete (non-fatal)")
	}

	return nil
}

// WriteYAML renders and atomically writes the CrowdSec whitelist YAML file from the current entries. An empty list
// writes the placeholder (see whitelistPlaceholderYAML) rather than removing the file, so the file is in place
// whenever an engine may reload it. It is a no-op when dataDir is empty (unit-test mode). Writes are serialised, and
// the entries query ignores the cancellation of ctx, so a client that disconnects after its DB commit still leaves
// the file in step with the list.
func (s *CrowdSecWhitelistService) WriteYAML(ctx context.Context) error {
	if s.dataDir == "" {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	target := filepath.Join(s.dataDir, filepath.FromSlash(crowdsec.WhitelistRelPath))
	dir := filepath.Dir(target)
	removeLegacyWhitelistTemp(target)

	ips, cidrs, err := s.whitelistLists(ctx)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("write whitelist yaml: create dir: %w", err)
	}

	// The owner of the live file is kept (CrowdSec reads it), and a refused rename falls back to an in-place write.
	// NoFollow refuses a symlink planted at the target, so it is never written through.
	res, err := writeWhitelistFile(target, buildWhitelistYAML(ips, cidrs), 0o640, crowdsec.AtomicWriteOptions{
		KeepOwner:           true,
		InPlaceFallback:     true,
		NoFollow:            true,
		SweepTempsOlderThan: whitelistStaleTempAge,
	})
	if err != nil {
		return fmt.Errorf("write whitelist yaml: write file: %w", err)
	}
	if res.InPlace {
		noteWhitelistInPlace()
	}

	return nil
}

// noteWhitelistInPlace logs, once per process, that the whitelist file was written in place rather than renamed.
func noteWhitelistInPlace() {
	if whitelistInPlaceWarned.CompareAndSwap(false, true) {
		logWhitelistInPlace()
	}
}

// whitelistLists returns the IP and CIDR entries in creation order. The query runs on a context detached from ctx's
// cancellation, bounded by whitelistQueryTimeout, so a disconnect after the DB commit cannot stop the file write.
func (s *CrowdSecWhitelistService) whitelistLists(ctx context.Context) (ips, cidrs []string, err error) {
	queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), whitelistQueryTimeout)
	defer cancel()

	var entries []models.CrowdSecWhitelist
	err = s.db.WithContext(queryCtx).Order("created_at ASC").Find(&entries).Error
	if err != nil {
		return nil, nil, fmt.Errorf("write whitelist yaml: query entries: %w", err)
	}

	for _, e := range entries {
		if strings.Contains(e.IPOrCIDR, "/") {
			cidrs = append(cidrs, e.IPOrCIDR)
		} else {
			ips = append(ips, e.IPOrCIDR)
		}
	}
	return ips, cidrs, nil
}

// removeLegacyWhitelistTemp removes the fixed-name temporary file that earlier versions wrote next to the whitelist
// ("<target>.tmp") when it is a regular file. Anything else at that name is left alone. A failed removal is logged
// and never fails the write.
func removeLegacyWhitelistTemp(target string) {
	legacy := target + ".tmp"
	info, err := os.Lstat(legacy)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	if err := removeLegacyTemp(legacy); err != nil {
		logger.Log().WithError(err).Debug("could not remove a legacy CrowdSec whitelist temporary file")
	}
}

// normalizeIPOrCIDR validates and normalizes an IP address or CIDR block.
// For CIDRs, the network address is returned (e.g. "10.0.0.1/8" → "10.0.0.0/8").
func normalizeIPOrCIDR(raw string) (string, error) {
	if strings.Contains(raw, "/") {
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return "", err
		}
		return network.String(), nil
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return "", fmt.Errorf("invalid IP: %q", raw)
	}

	return ip.String(), nil
}

// buildWhitelistYAML constructs the YAML content for the CrowdSec whitelist parser. With no entries it returns the
// placeholder.
func buildWhitelistYAML(ips, cidrs []string) []byte {
	if len(ips) == 0 && len(cidrs) == 0 {
		return []byte(whitelistPlaceholderYAML)
	}
	var sb strings.Builder
	sb.WriteString(whitelistYAMLHeader)

	sb.WriteString("  ip:")
	if len(ips) == 0 {
		sb.WriteString(" []\n")
	} else {
		sb.WriteString("\n")
		for _, ip := range ips {
			sb.WriteString("    - \"")
			sb.WriteString(ip)
			sb.WriteString("\"\n")
		}
	}

	sb.WriteString("  cidr:")
	if len(cidrs) == 0 {
		sb.WriteString(" []\n")
	} else {
		sb.WriteString("\n")
		for _, cidr := range cidrs {
			sb.WriteString("    - \"")
			sb.WriteString(cidr)
			sb.WriteString("\"\n")
		}
	}

	return []byte(sb.String())
}
