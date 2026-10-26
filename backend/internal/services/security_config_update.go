package services

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// SecurityConfigUpdate is the partial update accepted by the security config save endpoint.
// A nil field means the request did not carry that key (or sent null), so the stored value is kept.
// An explicit empty string, zero or false is applied. Keys mirror the JSON names of models.SecurityConfig.
// The name selects the row and is not a column. crowdsec_mode is validated but never stored; the CrowdSec
// lifecycle owns that column.
type SecurityConfigUpdate struct {
	Name                *string `json:"name"`
	Enabled             *bool   `json:"enabled"`
	AdminWhitelist      *string `json:"admin_whitelist"`
	CrowdSecMode        *string `json:"crowdsec_mode"`
	CrowdSecAPIURL      *string `json:"crowdsec_api_url"`
	WAFMode             *string `json:"waf_mode"`
	WAFRulesSource      *string `json:"waf_rules_source"`
	WAFLearning         *bool   `json:"waf_learning"`
	WAFParanoiaLevel    *int    `json:"waf_paranoia_level"`
	WAFExclusions       *string `json:"waf_exclusions"`
	RateLimitMode       *string `json:"rate_limit_mode"`
	RateLimitEnable     *bool   `json:"rate_limit_enable"`
	RateLimitBurst      *int    `json:"rate_limit_burst"`
	RateLimitRequests   *int    `json:"rate_limit_requests"`
	RateLimitWindowSec  *int    `json:"rate_limit_window_sec"`
	RateLimitBypassList *string `json:"rate_limit_bypass_list"`
}

// validate refuses the update before anything is written. Only the values the request carries are checked.
func (u SecurityConfigUpdate) validate() error {
	if u.AdminWhitelist != nil {
		if err := ValidateAdminWhitelist(*u.AdminWhitelist); err != nil {
			return err
		}
	}
	if u.CrowdSecMode != nil {
		if err := ValidateCrowdSecMode(*u.CrowdSecMode); err != nil {
			return err
		}
	}
	return nil
}

// applyTo copies the fields present in u onto row and returns the database column of each field it wrote.
// The rate-limit mode rule (ResolveRateLimitMode) decides rate_limit_mode from the enable flag and the mode key:
// an enable flag always sets the mode, and a mode alone is stored as sent.
func (u SecurityConfigUpdate) applyTo(row *models.SecurityConfig) []string {
	var columns []string
	if u.Enabled != nil {
		row.Enabled = *u.Enabled
		columns = append(columns, "enabled")
	}
	if u.AdminWhitelist != nil {
		row.AdminWhitelist = *u.AdminWhitelist
		columns = append(columns, "admin_whitelist")
	}
	if u.CrowdSecAPIURL != nil {
		row.CrowdSecAPIURL = *u.CrowdSecAPIURL
		columns = append(columns, "crowd_sec_api_url") // GORM's column name for CrowdSecAPIURL
	}
	if u.WAFMode != nil {
		row.WAFMode = *u.WAFMode
		columns = append(columns, "waf_mode")
	}
	if u.WAFRulesSource != nil {
		row.WAFRulesSource = *u.WAFRulesSource
		columns = append(columns, "waf_rules_source")
	}
	if u.WAFLearning != nil {
		row.WAFLearning = *u.WAFLearning
		columns = append(columns, "waf_learning")
	}
	if u.WAFParanoiaLevel != nil {
		row.WAFParanoiaLevel = *u.WAFParanoiaLevel
		columns = append(columns, "waf_paranoia_level")
	}
	if u.WAFExclusions != nil {
		row.WAFExclusions = *u.WAFExclusions
		columns = append(columns, "waf_exclusions")
	}
	switch {
	case u.RateLimitEnable != nil:
		mode := ""
		if u.RateLimitMode != nil {
			mode = *u.RateLimitMode
		}
		row.RateLimitEnable = *u.RateLimitEnable
		row.RateLimitMode = ResolveRateLimitMode(*u.RateLimitEnable, mode)
		columns = append(columns, "rate_limit_enable", "rate_limit_mode")
	case u.RateLimitMode != nil:
		row.RateLimitMode = *u.RateLimitMode
		columns = append(columns, "rate_limit_mode")
	}
	if u.RateLimitBurst != nil {
		row.RateLimitBurst = *u.RateLimitBurst
		columns = append(columns, "rate_limit_burst")
	}
	if u.RateLimitRequests != nil {
		row.RateLimitRequests = *u.RateLimitRequests
		columns = append(columns, "rate_limit_requests")
	}
	if u.RateLimitWindowSec != nil {
		row.RateLimitWindowSec = *u.RateLimitWindowSec
		columns = append(columns, "rate_limit_window_sec")
	}
	if u.RateLimitBypassList != nil {
		row.RateLimitBypassList = *u.RateLimitBypassList
		columns = append(columns, "rate_limit_bypass_list")
	}
	return columns
}

// newSecurityConfigRow returns a row for name with the model defaults that a new row starts from.
func newSecurityConfigRow(name string) models.SecurityConfig {
	return models.SecurityConfig{
		Name:             name,
		RateLimitMode:    "disabled",
		WAFParanoiaLevel: 1,
	}
}

// ApplyUpdate applies only the fields present in u to the named security config row and returns the stored row.
// It validates the update first, then runs inside a write transaction: every query goes through tx, because the
// connection pool holds a single connection. A missing row is created from the model defaults. An existing row is
// updated in the present columns only, so columns the request does not carry keep their stored values. An update
// with no present column issues no UPDATE.
func (s *SecurityService) ApplyUpdate(name string, u SecurityConfigUpdate) (*models.SecurityConfig, error) {
	if err := u.validate(); err != nil {
		return nil, err
	}

	var stored models.SecurityConfig
	err := WithWriteLock(s.db, &models.SecurityConfig{}, func(tx *gorm.DB) error {
		var row models.SecurityConfig
		findErr := tx.Where("name = ?", name).First(&row).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			row = newSecurityConfigRow(name)
			u.applyTo(&row)
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create security config %q: %w", name, err)
			}
		case findErr != nil:
			return fmt.Errorf("find security config %q: %w", name, findErr)
		default:
			// An empty column list would make GORM write every non-zero field of row, so it is skipped.
			if columns := u.applyTo(&row); len(columns) > 0 {
				if err := tx.Model(&row).Select(columns).Updates(&row).Error; err != nil {
					return fmt.Errorf("update security config %q: %w", name, err)
				}
			}
		}
		if err := tx.Where("name = ?", name).First(&stored).Error; err != nil {
			return fmt.Errorf("read security config %q: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &stored, nil
}
