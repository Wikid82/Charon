package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnforcementReport_GeoIPUnavailableReasonValue(t *testing.T) {
	assert.Equal(t, "geoip_unavailable", ReasonGeoIPUnavailable)
}
