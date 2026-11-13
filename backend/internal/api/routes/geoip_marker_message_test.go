package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeoIPBuildMarkerMessage_FollowsDatabaseState(t *testing.T) {
	assert.Contains(t, geoIPBuildMarkerMessage(false), "country access lists stay refused")
	assert.Contains(t, geoIPBuildMarkerMessage(false), "CHARON_GEOIP_DB_PATH")

	loaded := geoIPBuildMarkerMessage(true)
	assert.NotContains(t, loaded, "stay refused", "a loaded database means country lists are not refused")
	assert.Contains(t, loaded, "is in use")
}
