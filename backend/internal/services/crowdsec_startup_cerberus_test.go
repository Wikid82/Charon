package services

import (
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestReconcileCrowdSecOnStartup_CerberusCheck pins the boot note: the engine follows the CrowdSec setting only,
// and one Info line says so when the proxy will leave CrowdSec out because the security suite is off.
func TestReconcileCrowdSecOnStartup_CerberusCheck(t *testing.T) {
	tests := []struct {
		name       string
		crowdSec   string // CrowdSecMode of the default SecurityConfig row
		withOption bool
		cerberusOn bool
		wantStart  bool
		wantLog    int
	}{
		{name: "suite off logs once after a start", crowdSec: "local", withOption: true, cerberusOn: false, wantStart: true, wantLog: 1},
		{name: "suite on does not log", crowdSec: "local", withOption: true, cerberusOn: true, wantStart: true, wantLog: 0},
		{name: "option absent does not log", crowdSec: "local", withOption: false, wantStart: true, wantLog: 0},
		{name: "engine off does not start or log", crowdSec: "disabled", withOption: true, cerberusOn: false, wantStart: false, wantLog: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := setupCrowdsecTestDB(t)
			binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
			defer cleanup()
			require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, CrowdSecMode: tc.crowdSec}).Error)

			var opts []ReconcileOption
			if tc.withOption {
				opts = append(opts, WithCerberusCheck(func() bool { return tc.cerberusOn }))
			}

			hook := logtest.NewLocal(logger.Log().Logger)
			exec := &smartMockCrowdsecExecutor{startPid: 99999}
			ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{}, opts...)

			assert.Equal(t, tc.wantStart, exec.startCalled)
			assert.Equal(t, tc.wantLog, countLogs(hook, crowdSecProxySuiteOffMessage))
		})
	}
}

// TestReconcileCrowdSecOnStartup_CerberusCheckNotLoggedWhenStartFails pins that the note follows a verified start
// only: a start that does not verify logs nothing about the suite.
func TestReconcileCrowdSecOnStartup_CerberusCheckNotLoggedWhenStartFails(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	hook := logtest.NewLocal(logger.Log().Logger)
	exec := &verificationFailExecutor{startPid: 99999}
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{},
		WithCerberusCheck(func() bool { return false }))

	assert.True(t, exec.startCalled)
	assert.True(t, exec.verifyFailed)
	assert.Zero(t, countLogs(hook, crowdSecProxySuiteOffMessage))
}

// TestProxyCerberusCheck pins that boot asks the same Cerberus decision the proxy applies, reading the default row
// itself, and that the boot note is written for a default row whose global switch is off.
func TestProxyCerberusCheck(t *testing.T) {
	tests := []struct {
		name     string
		hasRow   bool
		rowOn    bool
		static   bool
		wantOnOK bool
	}{
		{name: "row on", hasRow: true, rowOn: true, static: false, wantOnOK: true},
		{name: "row off overrides static on", hasRow: true, rowOn: false, static: true, wantOnOK: false},
		{name: "no row falls back to static on", hasRow: false, static: true, wantOnOK: true},
		{name: "no row and static off", hasRow: false, static: false, wantOnOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := setupCrowdsecTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}))
			if tc.hasRow {
				require.NoError(t, db.Create(&models.SecurityConfig{
					Name: models.DefaultSecurityConfigName, CrowdSecMode: "local", Enabled: tc.rowOn,
				}).Error)
			}

			check := ProxyCerberusCheck(db, config.SecurityConfig{CerberusEnabled: tc.static})

			assert.Equal(t, tc.wantOnOK, check())
		})
	}
}

// TestReconcileCrowdSecOnStartup_ProxyCerberusCheckLogsWhenSuiteOff runs boot with the real proxy check: the
// default row's global switch is off, so the engine starts and one note is written.
func TestReconcileCrowdSecOnStartup_ProxyCerberusCheckLogsWhenSuiteOff(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name: models.DefaultSecurityConfigName, CrowdSecMode: "local", Enabled: false,
	}).Error)

	hook := logtest.NewLocal(logger.Log().Logger)
	exec := &smartMockCrowdsecExecutor{startPid: 99999}
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{},
		WithCerberusCheck(ProxyCerberusCheck(db, config.SecurityConfig{CerberusEnabled: true})))

	assert.True(t, exec.startCalled)
	assert.Equal(t, 1, countLogs(hook, crowdSecProxySuiteOffMessage))
}
