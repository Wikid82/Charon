package services

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// parityCase is one caller address and list configuration. The table runs TestIP (the Charon interface check) and
// Evaluate (the proxy model) over every combination and compares them.
type parityCase struct {
	listType string
	local    bool
	ip       string
	// adminExempt is the admin allow-list entry for the run (empty when none).
	admin string
}

// isGeo reports whether listType is a country list.
func isGeo(listType string) bool {
	return listType == "geo_whitelist" || listType == "geo_blacklist"
}

func TestAccessListParity_TestIPMatchesEvaluate(t *testing.T) {
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

			testIPAllowed, _, err := svc.TestIP(acl.ID, c.ip)
			require.NoError(t, err)

			var admin []string
			if c.admin != "" {
				admin = []string{c.admin}
				require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: c.admin}).Error)
			}
			addr := parseTestAddr(t, c.ip)
			decision, err := svc.Evaluate(acl, addr, admin)
			require.NoError(t, err)

			switch {
			case isGeo(c.listType) && c.local:
				// TestIP checks the local network first; Evaluate decides the country first, and with no database admits.
				assert.Equal(t, localNetworkContains(addr), testIPAllowed, "TestIP applies the local-network check")
				assert.True(t, decision.Allowed, "Evaluate admits without a database")
			case c.admin != "":
				// TestIP has no admin exemption; Evaluate exempts the admin address.
				noAdmin, err := svc.Evaluate(acl, addr, nil)
				require.NoError(t, err)
				assert.Equal(t, noAdmin.Allowed, testIPAllowed, "TestIP equals Evaluate without the exemption")
				assert.True(t, decision.Allowed, "Evaluate exempts the admin address")
			default:
				assert.Equal(t, testIPAllowed, decision.Allowed, "reason %s", decision.Reason)
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
