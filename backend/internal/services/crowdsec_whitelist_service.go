package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

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

// CrowdSecWhitelistService manages the CrowdSec IP/CIDR whitelist.
type CrowdSecWhitelistService struct {
	db      *gorm.DB
	dataDir string
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

// WriteYAML renders and atomically writes the CrowdSec whitelist YAML file.
// It is a no-op when dataDir is empty (unit-test mode).
func (s *CrowdSecWhitelistService) WriteYAML(ctx context.Context) error {
	if s.dataDir == "" {
		return nil
	}

	var entries []models.CrowdSecWhitelist
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&entries).Error; err != nil {
		return fmt.Errorf("write whitelist yaml: query entries: %w", err)
	}

	var ips, cidrs []string
	for _, e := range entries {
		if strings.Contains(e.IPOrCIDR, "/") {
			cidrs = append(cidrs, e.IPOrCIDR)
		} else {
			ips = append(ips, e.IPOrCIDR)
		}
	}

	dir := filepath.Join(s.dataDir, "config", "parsers", "s02-enrich")
	target := filepath.Join(dir, "charon-whitelist.yaml")

	// CrowdSec rejects a whitelist parser whose ip and cidr lists are both empty
	// ("Node is empty" fatal). When there are no whitelist entries, remove the
	// file so CrowdSec starts cleanly; it handles a missing file gracefully.
	if len(ips) == 0 && len(cidrs) == 0 {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("write whitelist yaml: remove empty: %w", err)
		}
		return nil
	}

	content := buildWhitelistYAML(ips, cidrs)

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("write whitelist yaml: create dir: %w", err)
	}

	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, content, 0o640); err != nil { //nolint:gosec // G306: 0640 grants group-read for CrowdSec config files
		return fmt.Errorf("write whitelist yaml: write temp: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write whitelist yaml: rename: %w", err)
	}

	return nil
}

// normalizeIPOrCIDR validates and normalizes an IP address or CIDR block.
// For CIDRs, the network address is returned (e.g. "10.0.0.1/8" → "10.0.0.0/8").
func normalizeIPOrCIDR(raw string) (string, error) {
	if strings.Contains(raw, "/") {
		ip, network, err := net.ParseCIDR(raw)
		if err != nil {
			return "", err
		}
		_ = ip
		return network.String(), nil
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return "", fmt.Errorf("invalid IP: %q", raw)
	}

	return ip.String(), nil
}

// buildWhitelistYAML constructs the YAML content for the CrowdSec whitelist parser.
func buildWhitelistYAML(ips, cidrs []string) []byte {
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
