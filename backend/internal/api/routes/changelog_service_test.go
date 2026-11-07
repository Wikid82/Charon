package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChangelogService_UsesRunningVersionByDefault(t *testing.T) {
	svc := newChangelogService("0.46.6", "development", "")

	assert.Equal(t, "0.46.6", svc.CurrentVersion())
	assert.False(t, svc.IsDevBuild())
}

func TestNewChangelogService_OverrideAppliesOutsideProduction(t *testing.T) {
	svc := newChangelogService("0.46.6", "development", "1.2.0+dev")

	assert.Equal(t, "1.2.0+dev", svc.CurrentVersion())
	assert.Equal(t, "1.2.0", svc.AnchorVersion())
}

func TestNewChangelogService_OverrideIgnoredInProduction(t *testing.T) {
	svc := newChangelogService("0.46.6", "production", "1.2.0")

	assert.Equal(t, "0.46.6", svc.CurrentVersion())
}

// An unversioned override is still applied (so local UI iteration with
// "dev" keeps working) but it must be reported: it turns What's New off.
func TestNewChangelogService_UnversionedOverrideIsApplied(t *testing.T) {
	svc := newChangelogService("0.46.6", "development", "nightly-abc")

	assert.Equal(t, "nightly-abc", svc.CurrentVersion())
	assert.True(t, svc.IsDevBuild())
}
