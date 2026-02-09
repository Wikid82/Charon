package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrSecurityConfigNotFound = errors.New("security config not found")
	ErrInvalidAdminCIDR       = errors.New("invalid admin whitelist CIDR")
	ErrBreakGlassInvalid      = errors.New("break-glass token invalid")
)

type SecurityService struct {
	db        *gorm.DB
	auditChan chan *models.SecurityAudit
	done      chan struct{}  // Channel to signal goroutine to stop
	wg        sync.WaitGroup // WaitGroup to track goroutine completion
	closed    bool           // Flag to prevent double-close
	mu        sync.Mutex     // Mutex to protect closed flag
}

// NewSecurityService returns a SecurityService using the provided DB
func NewSecurityService(db *gorm.DB) *SecurityService {
	s := &SecurityService{
		db:        db,
		auditChan: make(chan *models.SecurityAudit, 100), // Buffered channel with capacity 100
		done:      make(chan struct{}),
	}
	// Start background goroutine to process audit events asynchronously
	s.wg.Add(1)
	go s.processAuditEvents()
	return s
}

// Close gracefully stops the SecurityService and waits for audit processing to complete
func (s *SecurityService) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return // Already closed
	}
	s.closed = true
	s.mu.Unlock()

	close(s.done)      // Signal the goroutine to stop
	close(s.auditChan) // Close the audit channel
	s.wg.Wait()        // Wait for the goroutine to finish
}

// Flush processes all pending audit logs synchronously (useful for testing)
func (s *SecurityService) Flush() {
	// Wait for all pending audits to be processed
	// In practice, we wait for the channel to be empty and then a bit more
	// to ensure the database write completes
	for i := 0; i < 20; i++ { // Max 200ms wait
		if len(s.auditChan) == 0 {
			time.Sleep(10 * time.Millisecond) // Extra wait for DB write
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Get returns the first SecurityConfig row (singleton config)
func (s *SecurityService) Get() (*models.SecurityConfig, error) {
	var cfg models.SecurityConfig
	// Prefer the canonical singleton row named "default".
	// Some environments may contain multiple rows (e.g., tests or prior data);
	// returning an arbitrary "first" row can break break-glass token validation.
	if err := s.db.Where("name = ?", "default").First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Backward compatibility: if there is no explicit "default" row,
			// fall back to the first row if any exists.
			if err2 := s.db.First(&cfg).Error; err2 != nil {
				if errors.Is(err2, gorm.ErrRecordNotFound) {
					return nil, ErrSecurityConfigNotFound
				}
				return nil, err2
			}
		} else {
			return nil, err
		}
	}
	return &cfg, nil
}

// Upsert validates and saves a security config
func (s *SecurityService) Upsert(cfg *models.SecurityConfig) error {
	// Validate AdminWhitelist - comma-separated list of CIDRs
	if cfg.AdminWhitelist != "" {
		parts := strings.Split(cfg.AdminWhitelist, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// Validate as IP or CIDR using the same helper as AccessListService
			if !isValidCIDR(p) {
				return ErrInvalidAdminCIDR
			}
		}
	}

	// If a breakglass token is present in BreakGlassHash as empty string,
	// do not overwrite it here. Token generation should be done explicitly.

	// Validate CrowdSec mode on input prior to any DB operations: only 'local' or 'disabled' supported
	if cfg.CrowdSecMode != "" && cfg.CrowdSecMode != "local" && cfg.CrowdSecMode != "disabled" {
		return fmt.Errorf("invalid crowdsec mode: %s", cfg.CrowdSecMode)
	}

	// Upsert behaviour: try to find existing record
	var existing models.SecurityConfig
	if err := s.db.Where("name = ?", cfg.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// New record
			return s.db.Create(cfg).Error
		}
		return err
	}

	// Preserve existing BreakGlassHash if not provided
	if cfg.BreakGlassHash == "" {
		cfg.BreakGlassHash = existing.BreakGlassHash
	}
	existing.Enabled = cfg.Enabled
	existing.AdminWhitelist = cfg.AdminWhitelist
	// Validate CrowdSec mode: only 'local' or 'disabled' supported. Reject external/remote values.
	if cfg.CrowdSecMode != "" && cfg.CrowdSecMode != "local" && cfg.CrowdSecMode != "disabled" {
		return fmt.Errorf("invalid crowdsec mode: %s", cfg.CrowdSecMode)
	}
	existing.CrowdSecMode = cfg.CrowdSecMode
	existing.CrowdSecAPIURL = cfg.CrowdSecAPIURL
	existing.WAFMode = cfg.WAFMode
	existing.WAFRulesSource = cfg.WAFRulesSource
	existing.WAFLearning = cfg.WAFLearning
	existing.WAFParanoiaLevel = cfg.WAFParanoiaLevel
	existing.WAFExclusions = cfg.WAFExclusions
	existing.RateLimitEnable = cfg.RateLimitEnable
	existing.RateLimitBurst = cfg.RateLimitBurst
	existing.RateLimitRequests = cfg.RateLimitRequests
	existing.RateLimitWindowSec = cfg.RateLimitWindowSec
	existing.RateLimitBypassList = cfg.RateLimitBypassList

	return s.db.Save(&existing).Error
}

// GenerateBreakGlassToken generates a token, stores its bcrypt hash, and returns the plaintext token
func (s *SecurityService) GenerateBreakGlassToken(name string) (string, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	var cfg models.SecurityConfig
	if err := s.db.Where("name = ?", name).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = models.SecurityConfig{Name: name, BreakGlassHash: string(hash)}
			if createErr := s.db.Create(&cfg).Error; createErr != nil {
				return "", createErr
			}
			return token, nil
		}
		return "", err
	}

	cfg.BreakGlassHash = string(hash)
	if err := s.db.Save(&cfg).Error; err != nil {
		return "", err
	}
	return token, nil
}

// VerifyBreakGlassToken validates a provided token against the stored hash
func (s *SecurityService) VerifyBreakGlassToken(name, token string) (bool, error) {
	var cfg models.SecurityConfig
	if err := s.db.Where("name = ?", name).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrSecurityConfigNotFound
		}
		return false, err
	}
	if cfg.BreakGlassHash == "" {
		return false, ErrBreakGlassInvalid
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.BreakGlassHash), []byte(token)); err != nil {
		return false, ErrBreakGlassInvalid
	}
	return true, nil
}

// LogDecision stores a security decision record
func (s *SecurityService) LogDecision(d *models.SecurityDecision) error {
	if d == nil {
		return nil
	}
	if d.UUID == "" {
		d.UUID = uuid.NewString()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	return s.db.Create(d).Error
}

// ListDecisions returns recent security decisions, ordered by created_at desc
func (s *SecurityService) ListDecisions(limit int) ([]models.SecurityDecision, error) {
	var res []models.SecurityDecision
	q := s.db.Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// LogAudit stores an audit entry asynchronously via buffered channel
func (s *SecurityService) LogAudit(a *models.SecurityAudit) error {
	if a == nil {
		return nil
	}
	if a.UUID == "" {
		a.UUID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}

	// Non-blocking send to avoid blocking main operations
	select {
	case s.auditChan <- a:
		return nil
	default:
		// If channel is full, log the event but don't block
		// In production, consider incrementing a dropped events metric
		return errors.New("audit channel full, event dropped")
	}
}

// processAuditEvents processes audit events from the channel in the background
func (s *SecurityService) processAuditEvents() {
	defer s.wg.Done() // Mark goroutine as done when it exits

	for {
		select {
		case audit, ok := <-s.auditChan:
			if !ok {
				// Channel closed, exit goroutine
				return
			}
			if err := s.db.Create(audit).Error; err != nil {
				// Silently ignore errors from closed databases (common in tests)
				// Only log for other types of errors
				errMsg := err.Error()
				if !strings.Contains(errMsg, "no such table") &&
					!strings.Contains(errMsg, "database is closed") {
					fmt.Printf("Failed to write audit log: %v\n", err)
				}
			}
		case <-s.done:
			// Service is shutting down - drain remaining audit events before exiting
			for audit := range s.auditChan {
				if err := s.db.Create(audit).Error; err != nil {
					errMsg := err.Error()
					if !strings.Contains(errMsg, "no such table") &&
						!strings.Contains(errMsg, "database is closed") {
						fmt.Printf("Failed to write audit log: %v\n", err)
					}
				}
			}
			return
		}
	}
}

// AuditLogFilter represents filtering criteria for audit log queries
type AuditLogFilter struct {
	Actor         string
	Action        string
	EventCategory string
	ResourceUUID  string
	StartDate     *time.Time
	EndDate       *time.Time
}

// ListAuditLogs retrieves audit logs with pagination and filtering
func (s *SecurityService) ListAuditLogs(filter AuditLogFilter, page, limit int) ([]models.SecurityAudit, int64, error) {
	var audits []models.SecurityAudit
	var total int64

	// Build query with filters
	query := s.db.Model(&models.SecurityAudit{})

	if filter.Actor != "" {
		query = query.Where("actor = ?", filter.Actor)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.EventCategory != "" {
		query = query.Where("event_category = ?", filter.EventCategory)
	}
	if filter.ResourceUUID != "" {
		query = query.Where("resource_uuid = ?", filter.ResourceUUID)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&audits).Error; err != nil {
		return nil, 0, err
	}

	return audits, total, nil
}

// GetAuditLogByUUID retrieves a single audit log by its UUID
func (s *SecurityService) GetAuditLogByUUID(auditUUID string) (*models.SecurityAudit, error) {
	var audit models.SecurityAudit
	if err := s.db.Where("uuid = ?", auditUUID).First(&audit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("audit log not found")
		}
		return nil, err
	}
	return &audit, nil
}

// ListAuditLogsByProvider retrieves audit logs for a specific DNS provider with pagination
func (s *SecurityService) ListAuditLogsByProvider(providerID uint, page, limit int) ([]models.SecurityAudit, int64, error) {
	var audits []models.SecurityAudit
	var total int64

	query := s.db.Model(&models.SecurityAudit{}).
		Where("event_category = ? AND resource_id = ?", "dns_provider", providerID)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&audits).Error; err != nil {
		return nil, 0, err
	}

	return audits, total, nil
}

// UpsertRuleSet saves or updates a ruleset content
func (s *SecurityService) UpsertRuleSet(r *models.SecurityRuleSet) error {
	if r == nil {
		return nil
	}
	// Basic validations
	if r.Name == "" {
		return fmt.Errorf("rule set name required")
	}
	// Prevent huge payloads from being stored in DB (e.g., limit 2MB)
	if len(r.Content) > 2*1024*1024 {
		return fmt.Errorf("ruleset content too large")
	}
	var existing models.SecurityRuleSet
	if err := s.db.Where("name = ?", r.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if r.UUID == "" {
				r.UUID = uuid.NewString()
			}
			if r.LastUpdated.IsZero() {
				r.LastUpdated = time.Now()
			}
			return s.db.Create(r).Error
		}
		return err
	}
	existing.SourceURL = r.SourceURL
	existing.Content = r.Content
	existing.Mode = r.Mode
	existing.LastUpdated = r.LastUpdated
	return s.db.Save(&existing).Error
}

// DeleteRuleSet removes a ruleset by id
func (s *SecurityService) DeleteRuleSet(id uint) error {
	var rs models.SecurityRuleSet
	if err := s.db.Where("id = ?", id).First(&rs).Error; err != nil {
		return err
	}
	return s.db.Delete(&rs).Error
}

// ListRuleSets returns all known rulesets
func (s *SecurityService) ListRuleSets() ([]models.SecurityRuleSet, error) {
	var res []models.SecurityRuleSet
	if err := s.db.Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// helper: reused from access_list_service validation for CIDR/IP parsing
func isValidCIDR(cidr string) bool {
	// Try parsing as single IP
	if ip := net.ParseIP(cidr); ip != nil {
		return true
	}
	// Try parsing as CIDR
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}
