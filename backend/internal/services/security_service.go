package services

import (
    "crypto/rand"
    "encoding/hex"
    "errors"
    "strings"
    "net"
    "time"

    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"

    "github.com/Wikid82/charon/backend/internal/models"
    "gorm.io/gorm"
)

var (
    ErrSecurityConfigNotFound = errors.New("security config not found")
    ErrInvalidAdminCIDR        = errors.New("invalid admin whitelist CIDR")
    ErrBreakGlassInvalid       = errors.New("break-glass token invalid")
)

type SecurityService struct {
    db *gorm.DB
}

// NewSecurityService returns a SecurityService using the provided DB
func NewSecurityService(db *gorm.DB) *SecurityService {
    return &SecurityService{db: db}
}

// Get returns the first SecurityConfig row (singleton config)
func (s *SecurityService) Get() (*models.SecurityConfig, error) {
    var cfg models.SecurityConfig
    if err := s.db.First(&cfg).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrSecurityConfigNotFound
        }
        return nil, err
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
    existing.CrowdSecMode = cfg.CrowdSecMode
    existing.WAFMode = cfg.WAFMode
    existing.RateLimitEnable = cfg.RateLimitEnable
    existing.RateLimitBurst = cfg.RateLimitBurst

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
            if err := s.db.Create(&cfg).Error; err != nil {
                return "", err
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

// LogAudit stores an audit entry
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
    return s.db.Create(a).Error
}

// UpsertRuleSet saves or updates a ruleset content
func (s *SecurityService) UpsertRuleSet(r *models.SecurityRuleSet) error {
    if r == nil {
        return nil
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
