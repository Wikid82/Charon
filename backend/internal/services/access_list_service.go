package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAccessListNotFound    = errors.New("access list not found")
	ErrInvalidAccessListType = errors.New("invalid access list type")
	ErrInvalidIPAddress      = errors.New("invalid IP address or CIDR")
	ErrInvalidCountryCode    = errors.New("invalid country code")
	ErrAccessListInUse       = errors.New("access list is in use by proxy hosts")
	// ErrAccessListManagementInUse refuses deleting a list that protects Charon's own interface.
	ErrAccessListManagementInUse = errors.New("access list is used by Charon's own interface")
	// ErrAccessListRulesRequired rejects an enabled allow-list that would admit nobody. A block-list
	// with no rules is allowed because it blocks nobody.
	ErrAccessListRulesRequired = errors.New("an enabled allow-list needs at least one IP rule or the local network option")
)

// ValidAccessListTypes defines allowed access list types
var ValidAccessListTypes = []string{"whitelist", "blacklist", "geo_whitelist", "geo_blacklist"}

// ISO 3166-1 alpha-2 country codes (comprehensive list for validation)
var validCountryCodes = map[string]bool{
	// North America
	"US": true, "CA": true, "MX": true,
	// Europe
	"GB": true, "DE": true, "FR": true, "IT": true, "ES": true, "NL": true, "BE": true,
	"SE": true, "NO": true, "DK": true, "FI": true, "PL": true, "CZ": true, "AT": true,
	"CH": true, "IE": true, "PT": true, "GR": true, "HU": true, "RO": true, "BG": true,
	"HR": true, "SI": true, "SK": true, "LT": true, "LV": true, "EE": true, "IS": true,
	"LU": true, "MT": true, "CY": true, "UA": true, "BY": true,
	// Asia
	"JP": true, "CN": true, "IN": true, "KR": true, "SG": true, "MY": true, "TH": true,
	"ID": true, "PH": true, "VN": true, "TW": true, "HK": true, "PK": true, "BD": true,
	"KP": true, "IR": true, "IQ": true, "SY": true, "AF": true, "LK": true, "MM": true,
	// Middle East
	"TR": true, "IL": true, "SA": true, "AE": true, "QA": true, "KW": true, "OM": true,
	"BH": true, "JO": true, "LB": true, "YE": true,
	// Africa
	"EG": true, "ZA": true, "NG": true, "KE": true, "ET": true, "TZ": true, "MA": true,
	"DZ": true, "SD": true, "UG": true, "GH": true,
	// South America
	"BR": true, "AR": true, "CL": true, "CO": true, "PE": true, "VE": true, "EC": true,
	"BO": true, "PY": true, "UY": true,
	// Caribbean / Central America
	"CU": true, "DO": true, "PR": true, "JM": true, "HT": true, "PA": true, "CR": true,
	// Oceania
	"AU": true, "NZ": true,
	// Russia & CIS
	"RU": true, "KZ": true, "UZ": true, "AZ": true, "GE": true, "AM": true,
}

// AccessListService handles access list CRUD and IP testing operations.
type AccessListService struct {
	db       *gorm.DB
	geoipSvc *GeoIPService
	// lookup is the country lookup Evaluate reads. It is the same service as geoipSvc, behind an interface so tests
	// can supply a fake. TestIP keeps reading geoipSvc directly.
	lookup countryLookup
}

// NewAccessListService creates a new AccessListService.
func NewAccessListService(db *gorm.DB) *AccessListService {
	return &AccessListService{db: db}
}

// SetGeoIPService sets the GeoIP service for geo-based access list lookups.
// This method allows optional injection of the GeoIP service.
func (s *AccessListService) SetGeoIPService(geoipSvc *GeoIPService) {
	s.geoipSvc = geoipSvc
	s.lookup = geoipSvc
}

// GetGeoIPService returns the configured GeoIP service (may be nil).
func (s *AccessListService) GetGeoIPService() *GeoIPService {
	return s.geoipSvc
}

// Create creates a new access list with validation
func (s *AccessListService) Create(acl *models.AccessList) error {
	if err := s.validateAccessList(acl); err != nil {
		return err
	}

	acl.CountryCodes = models.NormalizeCountryCodes(acl.CountryCodes)
	acl.UUID = uuid.New().String()
	return s.db.Create(acl).Error
}

// GetByID retrieves an access list by ID
func (s *AccessListService) GetByID(id uint) (*models.AccessList, error) {
	var acl models.AccessList
	// Use Find to avoid GORM 'record not found' log noise
	result := s.db.Where("id = ?", id).Limit(1).Find(&acl)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAccessListNotFound
	}
	return &acl, nil
}

// GetByUUID retrieves an access list by UUID
func (s *AccessListService) GetByUUID(uuidStr string) (*models.AccessList, error) {
	var acl models.AccessList
	// Use Find to avoid GORM 'record not found' log noise
	result := s.db.Where("uuid = ?", uuidStr).Limit(1).Find(&acl)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrAccessListNotFound
	}
	return &acl, nil
}

// List retrieves all access lists sorted by updated_at desc
func (s *AccessListService) List() ([]models.AccessList, error) {
	var acls []models.AccessList
	if err := s.db.Order("updated_at desc, id desc").Find(&acls).Error; err != nil {
		return nil, err
	}
	return acls, nil
}

// Update updates an existing access list with validation
// ProposedUpdate returns the list as it would be after Update with updates, without saving it. Management guards judge
// this state before the write.
func (s *AccessListService) ProposedUpdate(id uint, updates *models.AccessList) (*models.AccessList, error) {
	acl, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	applyAccessListUpdates(acl, updates)
	return acl, nil
}

// applyAccessListUpdates copies the editable fields of updates onto acl.
func applyAccessListUpdates(acl, updates *models.AccessList) {
	acl.Name = updates.Name
	acl.Description = updates.Description
	acl.Type = updates.Type
	acl.IPRules = updates.IPRules
	acl.CountryCodes = updates.CountryCodes
	acl.LocalNetworkOnly = updates.LocalNetworkOnly
	acl.Enabled = updates.Enabled
}

// Update saves the editable fields of the list with the given ID after validation.
func (s *AccessListService) Update(id uint, updates *models.AccessList) error {
	acl, err := s.ProposedUpdate(id, updates)
	if err != nil {
		return err
	}

	if err := s.validateAccessList(acl); err != nil {
		return err
	}
	acl.CountryCodes = models.NormalizeCountryCodes(acl.CountryCodes)

	return s.db.Save(acl).Error
}

// InUse reports whether the access list is part of a configuration: a proxy host uses it, or it protects Charon's own
// interface (see ManagementUses).
func (s *AccessListService) InUse(id uint) (bool, error) {
	proxyUse, err := s.proxyUses(id)
	if err != nil || proxyUse {
		return proxyUse, err
	}
	return s.ManagementUses(id)
}

// proxyUses reports whether any proxy host uses the access list.
func (s *AccessListService) proxyUses(id uint) (bool, error) {
	var count int64
	if err := s.db.Model(&models.ProxyHost{}).Where("access_list_id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("count proxy hosts using access list %d: %w", id, err)
	}
	return count > 0, nil
}

// ManagementUses reports whether the access list is in the management set. A missing list is not in use.
func (s *AccessListService) ManagementUses(id uint) (bool, error) {
	acl, err := s.GetByID(id)
	if errors.Is(err, ErrAccessListNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	uuids, err := ManagementListUUIDs(s.db)
	if err != nil {
		return false, err
	}
	for _, uuid := range uuids {
		if uuid == acl.UUID {
			return true, nil
		}
	}
	return false, nil
}

// Delete deletes an access list if no configuration uses it.
func (s *AccessListService) Delete(id uint) error {
	proxyUse, err := s.proxyUses(id)
	if err != nil {
		return err
	}
	if proxyUse {
		return ErrAccessListInUse
	}
	managed, err := s.ManagementUses(id)
	if err != nil {
		return err
	}
	if managed {
		return ErrAccessListManagementInUse
	}

	result := s.db.Delete(&models.AccessList{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAccessListNotFound
	}
	return nil
}

// parseCountryCodes parses a comma-separated list of country codes.
func parseCountryCodes(codes string) []string {
	if codes == "" {
		return nil
	}
	parts := strings.Split(codes, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(strings.ToUpper(part))
		if code != "" {
			result = append(result, code)
		}
	}
	return result
}

// validateAccessList validates access list fields
func (s *AccessListService) validateAccessList(acl *models.AccessList) error {
	if strings.TrimSpace(acl.Name) == "" {
		return errors.New("name is required")
	}
	return s.ValidateDraft(acl)
}

// ValidateDraft checks an access list's type, IP rules and country codes exactly as Create does, without the name.
// An unsaved draft, such as one sent to the self-check, carries no name, so it is checked with this.
func (s *AccessListService) ValidateDraft(acl *models.AccessList) error {
	// Validate type
	if !s.isValidType(acl.Type) {
		return ErrInvalidAccessListType
	}

	// Validate IP rules
	var rules []models.AccessListRule
	if acl.IPRules != "" {
		if err := json.Unmarshal([]byte(acl.IPRules), &rules); err != nil {
			return fmt.Errorf("invalid IP rules JSON: %w", err)
		}

		for _, rule := range rules {
			if !s.isValidCIDR(rule.CIDR) {
				return fmt.Errorf("%w: %s", ErrInvalidIPAddress, rule.CIDR)
			}
		}
	}

	// An enabled allow-list with nothing to admit would deny every request. The local-network option counts as a rule.
	if acl.Enabled && acl.Type == "whitelist" && !acl.LocalNetworkOnly && len(rules) == 0 {
		return ErrAccessListRulesRequired
	}

	// Validate country codes for geo types
	// Codes are checked in the same normalized form that gets stored, so stray spaces, case and empty entries are
	// not reported as invalid codes.
	if strings.HasPrefix(acl.Type, "geo_") {
		normalized := models.NormalizeCountryCodes(acl.CountryCodes)
		if normalized == "" {
			return errors.New("country codes are required for geo-blocking")
		}
		for _, code := range strings.Split(normalized, ",") {
			if !s.isValidCountryCode(code) {
				return fmt.Errorf("%w: %s", ErrInvalidCountryCode, code)
			}
		}
	}

	return nil
}

// isValidType checks if access list type is valid
func (s *AccessListService) isValidType(aclType string) bool {
	for _, valid := range ValidAccessListTypes {
		if aclType == valid {
			return true
		}
	}
	return false
}

// isValidCIDR validates IP address or CIDR notation
func (s *AccessListService) isValidCIDR(cidr string) bool {
	// Try parsing as single IP
	if ip := net.ParseIP(cidr); ip != nil {
		return true
	}

	// Try parsing as CIDR
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// isValidCountryCode validates ISO 3166-1 alpha-2 country code
func (s *AccessListService) isValidCountryCode(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return false
	}
	matched, _ := regexp.MatchString("^[A-Z]{2}$", code)
	return matched && validCountryCodes[code]
}

// GetTemplates returns predefined ACL templates
func (s *AccessListService) GetTemplates() []map[string]any {
	return []map[string]any{
		{
			"id":                 "local-network",
			"name":               "Local Network Only",
			"description":        "Allow only RFC1918 private network IPs (home/office networks)",
			"type":               "whitelist",
			"local_network_only": true,
			"category":           "security",
		},
		{
			"id":            "us-only",
			"name":          "US Only",
			"description":   "Allow only United States IPs",
			"type":          "geo_whitelist",
			"country_codes": "US",
			"category":      "security",
		},
		{
			"id":            "eu-only",
			"name":          "EU Only",
			"description":   "Allow only European Union IPs",
			"type":          "geo_whitelist",
			"country_codes": "AT,BE,BG,HR,CY,CZ,DK,EE,FI,FR,DE,GR,HU,IE,IT,LV,LT,LU,MT,NL,PL,PT,RO,SK,SI,ES,SE",
			"category":      "security",
		},
		{
			"id":            "high-risk-countries",
			"name":          "Block High-Risk Countries",
			"description":   "Block OFAC sanctioned countries and known attack sources",
			"type":          "geo_blacklist",
			"country_codes": "RU,CN,KP,IR,BY,SY,VE,CU,SD",
			"category":      "security",
		},
		{
			"id":            "expanded-threat-countries",
			"name":          "Block Expanded Threat List",
			"description":   "Block high-risk countries plus additional bot/spam sources",
			"type":          "geo_blacklist",
			"country_codes": "RU,CN,KP,IR,BY,SY,VE,CU,SD,PK,BD,NG,UA,VN,ID",
			"category":      "security",
		},
		// IP-based presets removed: IP blocklists and scanner ranges
		// These are better handled by CrowdSec, WAF, or rate limiting.
	}
}
