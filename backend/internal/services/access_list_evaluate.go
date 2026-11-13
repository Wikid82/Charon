package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// Decision reasons. They are stable identifiers; the tester maps each one to text.
const (
	ReasonListDisabled      = "list_disabled"
	ReasonAdminWhitelisted  = "admin_whitelisted"
	ReasonAllowed           = "allowed"
	ReasonGeoIPUnavailable  = "geoip_unavailable"
	ReasonCountryNotAllowed = "country_not_allowed"
	ReasonCountryBlocked    = "country_blocked"
	ReasonNoCountry         = "no_country"
	ReasonNotLocalNetwork   = "not_local_network"
	ReasonIPNotAllowed      = "ip_not_allowed"
	ReasonIPBlocked         = "ip_blocked"
)

// Decision is the outcome of Evaluate for one address.
type Decision struct {
	Allowed bool
	Reason  string
	// CountryCode is the country the lookup found. It is empty when none was found or the lookup failed.
	CountryCode string
	// AdminExempt reports that the address is in the admin whitelist.
	AdminExempt bool
	// GeoIPAvailable reports whether the GeoIP database was usable when the decision was made.
	GeoIPAvailable bool
}

// countryLookup is the GeoIP lookup Evaluate reads. *GeoIPService satisfies it; tests supply a fake.
type countryLookup interface {
	Available() bool
	LookupCountry(ip string) (string, error)
}

// Evaluate returns the decision the proxy makes for a request from ip under the access list acl, given the admin
// whitelist entries. It mirrors the proxy configuration built for the list: the admin whitelist is checked first for
// every list type, country lists are decided before local-network-only (which they ignore), and an unusable GeoIP
// database leaves country lists unenforced, so they allow the address with reason ReasonGeoIPUnavailable.
//
// An error means the stored list or admin whitelist holds an entry the proxy configuration cannot use.
func (s *AccessListService) Evaluate(acl *models.AccessList, ip netip.Addr, adminRanges []string) (Decision, error) {
	if acl == nil {
		return Decision{}, errors.New("evaluate access list: nil access list")
	}

	d := Decision{GeoIPAvailable: s.countryAvailable()}
	if !acl.Enabled {
		d.Allowed, d.Reason = true, ReasonListDisabled
		return d, nil
	}

	exempt, err := addrInAdminRanges(adminRanges, ip)
	if err != nil {
		return Decision{}, fmt.Errorf("evaluate access list %d: %w", acl.ID, err)
	}
	if exempt {
		d.Allowed, d.Reason, d.AdminExempt = true, ReasonAdminWhitelisted, true
		return d, nil
	}

	if strings.HasPrefix(acl.Type, "geo_") {
		return s.evaluateCountry(acl, ip, d), nil
	}

	if acl.LocalNetworkOnly {
		if localNetworkContains(ip) {
			d.Allowed, d.Reason = true, ReasonAllowed
		} else {
			d.Reason = ReasonNotLocalNetwork
		}
		return d, nil
	}

	return evaluateIPRules(acl, ip, d)
}

// AdminWhitelistRanges returns the admin whitelist entries the proxy configuration exempts. The proxy reads the same
// row and, when it cannot, generates its configuration with no whitelist; this does the same, and logs the read error.
func (s *AccessListService) AdminWhitelistRanges() []string {
	cfg, err := models.FindDefaultSecurityConfig(s.db)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Log().WithError(err).Warn("failed to read admin whitelist for access list test; no admin exemption applied")
		}
		return nil
	}
	return network.SplitAddressList(cfg.AdminWhitelist)
}

// countryAvailable reports whether country lookups can be served now. A service without a lookup has none.
func (s *AccessListService) countryAvailable() bool {
	return s.lookup != nil && s.lookup.Available()
}

// evaluateCountry decides a country list. The proxy's country placeholder is empty both when no country is found and
// when the lookup fails, so both reach here as no country: an allow-list refuses that, a block-list admits it.
func (s *AccessListService) evaluateCountry(acl *models.AccessList, ip netip.Addr, d Decision) Decision {
	if !d.GeoIPAvailable {
		// Fail open: with no usable database the proxy builds no country matcher for the list, so the address is allowed.
		d.Allowed, d.Reason = true, ReasonGeoIPUnavailable
		return d
	}

	country, lookupErr := s.lookup.LookupCountry(ip.String())
	if lookupErr != nil {
		country = ""
	}
	d.CountryCode = country
	listed := country != "" && countryInList(acl.CountryCodes, country)

	if acl.Type == "geo_whitelist" {
		switch {
		case country == "":
			d.Reason = ReasonNoCountry
		case listed:
			d.Allowed, d.Reason = true, ReasonAllowed
		default:
			d.Reason = ReasonCountryNotAllowed
		}
		return d
	}

	switch {
	case country == "":
		d.Allowed, d.Reason = true, ReasonNoCountry
	case listed:
		d.Reason = ReasonCountryBlocked
	default:
		d.Allowed, d.Reason = true, ReasonAllowed
	}
	return d
}

// evaluateIPRules decides an IP allow-list or block-list. An allow-list whose rules cannot be read or are empty
// refuses everyone, and a block-list whose rules cannot be read blocks nobody, as the proxy configuration does.
func evaluateIPRules(acl *models.AccessList, ip netip.Addr, d Decision) (Decision, error) {
	var rules []models.AccessListRule
	if acl.IPRules != "" {
		if err := json.Unmarshal([]byte(acl.IPRules), &rules); err != nil {
			rules = nil
		}
	}
	cidrs := make([]string, 0, len(rules))
	for _, rule := range rules {
		if !validAccessListEntry(rule.CIDR) {
			return Decision{}, fmt.Errorf("access list rule %q is not an IP address or CIDR", rule.CIDR)
		}
		cidrs = append(cidrs, rule.CIDR)
	}

	matched := false
	for _, cidr := range cidrs {
		if addrInEntry(cidr, ip) {
			matched = true
			break
		}
	}

	switch acl.Type {
	case "whitelist":
		if matched {
			d.Allowed, d.Reason = true, ReasonAllowed
		} else {
			d.Reason = ReasonIPNotAllowed
		}
	case "blacklist":
		if matched {
			d.Reason = ReasonIPBlocked
		} else {
			d.Allowed, d.Reason = true, ReasonAllowed
		}
	default:
		d.Allowed, d.Reason = true, ReasonAllowed
	}
	return d, nil
}

// countryInList reports whether country (upper-case ISO code) is one of the comma-separated codes.
func countryInList(codes, country string) bool {
	for _, code := range parseCountryCodes(codes) {
		if strings.EqualFold(code, country) {
			return true
		}
	}
	return false
}

// localNetworkContains reports whether addr is in one of the RFC1918 local networks.
func localNetworkContains(addr netip.Addr) bool {
	for _, cidr := range network.LocalNetworkCIDRs() {
		if addrInEntry(cidr, addr) {
			return true
		}
	}
	return false
}

// addrInAdminRanges reports whether addr is in one of the admin whitelist entries. A malformed entry is an error.
func addrInAdminRanges(ranges []string, addr netip.Addr) (bool, error) {
	for _, entry := range ranges {
		if !validAccessListEntry(entry) {
			return false, fmt.Errorf("admin whitelist entry %q is not an IP address or CIDR", entry)
		}
		if addrInEntry(entry, addr) {
			return true, nil
		}
	}
	return false, nil
}

// validAccessListEntry reports whether entry is a single IP address or a CIDR range.
func validAccessListEntry(entry string) bool {
	if _, err := netip.ParsePrefix(entry); err == nil {
		return true
	}
	_, err := netip.ParseAddr(entry)
	return err == nil
}

// addrInEntry reports whether addr is the single IP address or inside the CIDR range entry. An entry that does not
// parse matches nothing. IPv4-mapped IPv6 addresses compare as their IPv4 form.
func addrInEntry(entry string, addr netip.Addr) bool {
	addr = addr.Unmap()
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		return prefix.Contains(addr)
	}
	if single, err := netip.ParseAddr(entry); err == nil {
		return single.Unmap() == addr
	}
	return false
}
