package services

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordedInterfaceAllowed is the outcome recorded for the interface check that the management lists replaced. It
// applied the local-network option first (for every list type, geo included), then the country lists (with no GeoIP
// database they admitted), then the IP rules, with no admin allow-list exemption. The table below pins Evaluate, the
// proxy model, to that outcome wherever the two are meant to agree.
func recordedInterfaceAllowed(listType string, local bool, ip netip.Addr, rules []netip.Prefix) bool {
	if local {
		return ip.IsPrivate() || ip.IsLoopback()
	}
	if isGeo(listType) {
		return true // no GeoIP database: country lists admit
	}
	matched := false
	for _, p := range rules {
		if p.Contains(ip) {
			matched = true
		}
	}
	if listType == "whitelist" {
		return matched
	}
	return !matched
}

// parityCase is one caller address and list configuration.
type parityCase struct {
	listType string
	local    bool
	ip       string
	// admin is the admin allow-list entry for the run (empty when none).
	admin string
}

func isGeo(listType string) bool {
	return listType == "geo_whitelist" || listType == "geo_blacklist"
}

// TestAccessListParity_RecordedInterfaceExpectations: Evaluate matches the recorded interface outcome for every list
// type, local-network option, caller address and admin allow-list, except the documented differences: an admin
// allow-list address is exempt in Evaluate (the proxy model), and a geo list with the local-network option is decided
// by its country first (and admits without a database).
func TestAccessListParity_RecordedInterfaceExpectations(t *testing.T) {
	const listedIP, unlistedIP, privateIP, loopbackIP, adminIP = "198.51.100.7", "203.0.113.9", "10.1.2.3", "127.0.0.1", "192.0.2.10"

	var cases []parityCase
	for _, typ := range []string{"whitelist", "blacklist", "geo_whitelist", "geo_blacklist"} {
		for _, local := range []bool{false, true} {
			for _, ip := range []string{listedIP, unlistedIP, privateIP, loopbackIP, adminIP} {
				cases = append(cases, parityCase{listType: typ, local: local, ip: ip})
			}
			cases = append(cases, parityCase{listType: typ, local: local, ip: adminIP, admin: adminIP + "/32"})
		}
	}

	rules := []netip.Prefix{netip.MustParsePrefix("198.51.100.7/32")}
	for _, c := range cases {
		name := fmt.Sprintf("%s/local=%t/ip=%s/admin=%q", c.listType, c.local, c.ip, c.admin)
		t.Run(name, func(t *testing.T) {
			db := managementTestDB(t)
			acl := &models.AccessList{
				UUID:             "parity",
				Name:             "parity",
				Type:             c.listType,
				Enabled:          true,
				LocalNetworkOnly: c.local,
				IPRules:          fmt.Sprintf(`[{"cidr":%q}]`, listedIP+"/32"),
			}
			require.NoError(t, db.Create(acl).Error)
			svc := NewAccessListService(db)

			addr := parseTestAddr(t, c.ip)
			var admin []string
			if c.admin != "" {
				admin = []string{c.admin}
			}
			decision, err := svc.Evaluate(acl, addr, admin)
			require.NoError(t, err)
			recorded := recordedInterfaceAllowed(c.listType, c.local, addr, rules)

			switch {
			case isGeo(c.listType) && c.local:
				assert.True(t, decision.Allowed, "Evaluate decides the country first and admits without a database")
			case c.admin != "":
				assert.True(t, decision.Allowed, "Evaluate exempts the admin allow-list address")
				noAdmin, err := svc.Evaluate(acl, addr, nil)
				require.NoError(t, err)
				assert.Equal(t, recorded, noAdmin.Allowed, "without the exemption the recorded outcome holds")
			default:
				assert.Equal(t, recorded, decision.Allowed, "reason %s", decision.Reason)
			}
		})
	}
}

func parseTestAddr(t *testing.T, ip string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(ip)
	require.NoError(t, err)
	return addr
}
