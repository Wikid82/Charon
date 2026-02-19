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
)

// ValidAccessListTypes defines allowed access list types
var ValidAccessListTypes = []string{"whitelist", "blacklist", "geo_whitelist", "geo_blacklist"}

// RFC1918PrivateNetworks defines private IP ranges
var RFC1918PrivateNetworks = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",    // localhost
	"169.254.0.0/16", // link-local
	"fc00::/7",       // IPv6 ULA
	"fe80::/10",      // IPv6 link-local
	"::1/128",        // IPv6 localhost
}

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
}

// NewAccessListService creates a new AccessListService.
func NewAccessListService(db *gorm.DB) *AccessListService {
	return &AccessListService{db: db}
}

// SetGeoIPService sets the GeoIP service for geo-based access list lookups.
// This method allows optional injection of the GeoIP service.
func (s *AccessListService) SetGeoIPService(geoipSvc *GeoIPService) {
	s.geoipSvc = geoipSvc
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
func (s *AccessListService) Update(id uint, updates *models.AccessList) error {
	acl, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// Apply updates
	acl.Name = updates.Name
	acl.Description = updates.Description
	acl.Type = updates.Type
	acl.IPRules = updates.IPRules
	acl.CountryCodes = updates.CountryCodes
	acl.LocalNetworkOnly = updates.LocalNetworkOnly
	acl.Enabled = updates.Enabled

	if err := s.validateAccessList(acl); err != nil {
		return err
	}

	return s.db.Save(acl).Error
}

// Delete deletes an access list if not in use
func (s *AccessListService) Delete(id uint) error {
	// Check if ACL is in use by any proxy hosts
	var count int64
	if err := s.db.Model(&models.ProxyHost{}).Where("access_list_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrAccessListInUse
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

// TestIP tests if an IP address would be allowed/blocked by the access list
func (s *AccessListService) TestIP(aclID uint, ipAddress string) (allowed bool, reason string, err error) {
	acl, err := s.GetByID(aclID)
	if err != nil {
		return false, "", err
	}

	if !acl.Enabled {
		return true, "Access list is disabled - all traffic allowed", nil
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false, "", ErrInvalidIPAddress
	}

	// Test local network only
	if acl.LocalNetworkOnly {
		if !s.isPrivateIP(ip) {
			return false, "Not a private network IP (RFC1918)", nil
		}
		return true, "Allowed by local network only rule", nil
	}

	// Handle geo-based access lists
	if strings.HasPrefix(acl.Type, "geo_") {
		return s.testGeoIP(acl, ipAddress)
	}

	// Test IP rules
	if acl.IPRules != "" {
		var rules []models.AccessListRule
		if err := json.Unmarshal([]byte(acl.IPRules), &rules); err == nil {
			for _, rule := range rules {
				if s.ipMatchesCIDR(ip, rule.CIDR) {
					if acl.Type == "whitelist" {
						return true, fmt.Sprintf("Allowed by whitelist rule: %s", rule.CIDR), nil
					}
					if acl.Type == "blacklist" {
						return false, fmt.Sprintf("Blocked by blacklist rule: %s", rule.CIDR), nil
					}
				}
			}
		}
	}

	// Default behavior based on type
	if acl.Type == "whitelist" {
		return false, "Not in whitelist", nil
	}
	return true, "Not in blacklist", nil
}

// testGeoIP tests an IP against geo-based access list rules.
func (s *AccessListService) testGeoIP(acl *models.AccessList, ipAddress string) (allowed bool, reason string, err error) {
	// Check if GeoIP service is available
	if s.geoipSvc == nil || !s.geoipSvc.IsLoaded() {
		// Graceful degradation: if GeoIP is not available, allow traffic with warning
		return true, "GeoIP database not available - allowing by default", nil
	}

	// Look up the country for this IP
	country, err := s.geoipSvc.LookupCountry(ipAddress)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, ErrCountryNotFound) {
			// IP has no associated country (e.g., private IP, reserved range)
			if acl.Type == "geo_whitelist" {
				return false, "No country found for IP - not in geo whitelist", nil
			}
			// For blacklist, unknown country means not blocked
			return true, "No country found for IP - not in geo blacklist", nil
		}
		// Other errors: graceful degradation
		return true, fmt.Sprintf("GeoIP lookup error: %v - allowing by default", err), nil
	}

	// Parse the country codes from the ACL
	allowedCountries := s.parseCountryCodes(acl.CountryCodes)

	// Check if the country is in the list
	countryInList := false
	for _, code := range allowedCountries {
		if strings.EqualFold(code, country) {
			countryInList = true
			break
		}
	}

	// Apply whitelist/blacklist logic
	if acl.Type == "geo_whitelist" {
		if countryInList {
			return true, fmt.Sprintf("Allowed by geo whitelist: IP is from %s", country), nil
		}
		return false, fmt.Sprintf("Blocked by geo whitelist: IP is from %s (not in allowed countries)", country), nil
	}

	// geo_blacklist
	if countryInList {
		return false, fmt.Sprintf("Blocked by geo blacklist: IP is from %s", country), nil
	}
	return true, fmt.Sprintf("Allowed: IP is from %s (not in blocked countries)", country), nil
}

// parseCountryCodes parses a comma-separated list of country codes.
func (s *AccessListService) parseCountryCodes(codes string) []string {
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
	// Validate name
	if strings.TrimSpace(acl.Name) == "" {
		return errors.New("name is required")
	}

	// Validate type
	if !s.isValidType(acl.Type) {
		return ErrInvalidAccessListType
	}

	// Validate IP rules
	if acl.IPRules != "" {
		var rules []models.AccessListRule
		if err := json.Unmarshal([]byte(acl.IPRules), &rules); err != nil {
			return fmt.Errorf("invalid IP rules JSON: %w", err)
		}

		for _, rule := range rules {
			if !s.isValidCIDR(rule.CIDR) {
				return fmt.Errorf("%w: %s", ErrInvalidIPAddress, rule.CIDR)
			}
		}
	}

	// Validate country codes for geo types
	if strings.HasPrefix(acl.Type, "geo_") {
		if acl.CountryCodes == "" {
			return errors.New("country codes are required for geo-blocking")
		}
		codes := strings.Split(acl.CountryCodes, ",")
		for _, code := range codes {
			code = strings.TrimSpace(strings.ToUpper(code))
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

// ipMatchesCIDR checks if an IP matches a CIDR block
func (s *AccessListService) ipMatchesCIDR(ip net.IP, cidr string) bool {
	// Check if it's a single IP
	if singleIP := net.ParseIP(cidr); singleIP != nil {
		return ip.Equal(singleIP)
	}

	// Check CIDR range
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return ipNet.Contains(ip)
}

// isPrivateIP checks if an IP is in RFC1918 private ranges
func (s *AccessListService) isPrivateIP(ip net.IP) bool {
	for _, cidr := range RFC1918PrivateNetworks {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
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
