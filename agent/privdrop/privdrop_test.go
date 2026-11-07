package privdrop

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCreds simulates process credentials. setuid/setgid mutate the stored
// IDs like the kernel does for an unprivileged caller: once euid is non-zero,
// setuid can only set it back to the real uid, so regaining root fails.
type fakeCreds struct {
	uid, euid, gid, egid int
	groups               []int
	groupsSet            bool

	setgroupsErr error
	setgidErr    error
	setuidErr    error
	noopSetuid   bool // setuid reports success but changes nothing
	allowRegain  bool // setuid(0) succeeds, simulating a broken drop
}

func (f *fakeCreds) Getuid() int  { return f.uid }
func (f *fakeCreds) Geteuid() int { return f.euid }
func (f *fakeCreds) Getgid() int  { return f.gid }
func (f *fakeCreds) Getegid() int { return f.egid }

func (f *fakeCreds) Setgroups(gids []int) error {
	if f.setgroupsErr != nil {
		return f.setgroupsErr
	}
	f.groups = gids
	f.groupsSet = true
	return nil
}

func (f *fakeCreds) Setgid(gid int) error {
	if f.setgidErr != nil {
		return f.setgidErr
	}
	if f.euid != 0 && gid != f.gid {
		return errors.New("operation not permitted")
	}
	f.gid, f.egid = gid, gid
	return nil
}

func (f *fakeCreds) Setuid(uid int) error {
	if f.setuidErr != nil {
		return f.setuidErr
	}
	if f.noopSetuid {
		return nil
	}
	if f.euid != 0 && uid != f.uid && !f.allowRegain {
		return errors.New("operation not permitted")
	}
	f.uid, f.euid = uid, uid
	return nil
}

func newLogger() (*logrus.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := logrus.New()
	l.SetOutput(buf)
	return l, buf
}

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestApplyNonRootIsNoop(t *testing.T) {
	creds := &fakeCreds{uid: 1000, euid: 1000, gid: 1000, egid: 1000}
	log, _ := newLogger()

	err := apply(envMap(nil), creds, log)

	require.NoError(t, err)
	assert.False(t, creds.groupsSet)
	assert.Equal(t, 1000, creds.euid)
}

func TestApplyNonRootIgnoresInvalidAndDifferingIDs(t *testing.T) {
	creds := &fakeCreds{uid: 1000, euid: 1000, gid: 1000, egid: 1000}
	log, buf := newLogger()

	err := apply(envMap(map[string]string{"PUID": "abc", "PGID": "5"}), creds, log)

	require.NoError(t, err)
	assert.False(t, creds.groupsSet)
	assert.Contains(t, buf.String(), "PUID/PGID are ignored")
}

func TestApplyNonRootMatchingIDsNoWarning(t *testing.T) {
	creds := &fakeCreds{uid: 1000, euid: 1000, gid: 1000, egid: 1000}
	log, buf := newLogger()

	err := apply(envMap(map[string]string{"PUID": "1000", "PGID": "1000"}), creds, log)

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "ignored")
}

func TestApplyRootDefaultsToNobody(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(nil), creds, log)

	require.NoError(t, err)
	assert.Equal(t, defaultID, creds.euid)
	assert.Equal(t, defaultID, creds.uid)
	assert.Equal(t, defaultID, creds.egid)
	assert.Equal(t, defaultID, creds.gid)
	assert.True(t, creds.groupsSet)
	assert.Empty(t, creds.groups)
}

func TestApplyRootEmptyValuesUseDefaults(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "", "PGID": ""}), creds, log)

	require.NoError(t, err)
	assert.Equal(t, defaultID, creds.euid)
	assert.Equal(t, defaultID, creds.egid)
}

func TestApplyRootExplicitValues(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": " 1000 ", "PGID": "999"}), creds, log)

	require.NoError(t, err)
	assert.Equal(t, 1000, creds.euid)
	assert.Equal(t, 1000, creds.uid)
	assert.Equal(t, 999, creds.egid)
	assert.Equal(t, 999, creds.gid)
}

func TestApplyRootRefusesExplicitZeroPUID(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "0", "PGID": "1000"}), creds, log)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORTHRUS_ALLOW_ROOT")
	assert.False(t, creds.groupsSet)
	assert.Equal(t, 0, creds.euid)
}

func TestApplyRootRefusesExplicitZeroPGID(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "1000", "PGID": "0"}), creds, log)

	require.Error(t, err)
	assert.False(t, creds.groupsSet)
}

func TestApplyRootAllowedWhenExplicitZeroAndOverride(t *testing.T) {
	creds := &fakeCreds{}
	log, buf := newLogger()

	err := apply(envMap(map[string]string{"PUID": "0", "PGID": "0", envAllowRoot: "true"}), creds, log)

	require.NoError(t, err)
	assert.False(t, creds.groupsSet, "must not drop privileges when root is explicitly allowed")
	assert.Equal(t, 0, creds.euid)
	assert.Contains(t, buf.String(), "running as root")
}

func TestApplyAllowRootOnlyExactTrue(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "0", envAllowRoot: "yes"}), creds, log)

	require.Error(t, err)
}

func TestApplyAllowRootWithDefaultIDsStillDrops(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{envAllowRoot: "true"}), creds, log)

	require.NoError(t, err)
	assert.Equal(t, defaultID, creds.euid)
}

func TestApplyRootInvalidValuesAreFatal(t *testing.T) {
	cases := map[string]map[string]string{
		"non-numeric PUID": {"PUID": "abc"},
		"negative PUID":    {"PUID": "-1"},
		"float PGID":       {"PGID": "1.5"},
		"PUID above int32": {"PUID": "2147483648"},
		"PUID too large":   {"PUID": "4294967295"},
		"PGID too large":   {"PGID": "99999999999999999999"},
		"invalid with allow root": {
			"PUID": "x", envAllowRoot: "true",
		},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			creds := &fakeCreds{}
			log, _ := newLogger()

			err := apply(envMap(env), creds, log)

			require.Error(t, err)
			assert.False(t, creds.groupsSet)
		})
	}
}

func TestApplyMaxValidID(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": strconv.Itoa(math.MaxInt32), "PGID": strconv.Itoa(math.MaxInt32)}), creds, log)

	require.NoError(t, err)
	assert.Equal(t, math.MaxInt32, creds.euid)
	assert.Equal(t, math.MaxInt32, creds.egid)
}

func TestParseID(t *testing.T) {
	maxMsg := fmt.Sprintf("must be between 0 and %d", math.MaxInt32)
	cases := map[string]struct {
		raw     string
		want    int
		wantErr string
	}{
		"empty uses default":         {raw: "", want: defaultID},
		"whitespace uses default":    {raw: "   ", want: defaultID},
		"zero":                       {raw: "0", want: 0},
		"typical id":                 {raw: " 1000 ", want: 1000},
		"max int32 is valid":         {raw: strconv.Itoa(math.MaxInt32), want: math.MaxInt32},
		"one above max int32":        {raw: "2147483648", wantErr: maxMsg},
		"uint32 max rejected":        {raw: "4294967295", wantErr: maxMsg},
		"overflows int64":            {raw: "99999999999999999999", wantErr: maxMsg},
		"negative overflows int32":   {raw: "-2147483649", wantErr: maxMsg},
		"negative in range rejected": {raw: "-1", wantErr: maxMsg},
		"non-numeric":                {raw: "abc", wantErr: "must be a numeric id"},
		"float rejected":             {raw: "1.5", wantErr: "must be a numeric id"},
		"hex rejected":               {raw: "0x10", wantErr: "must be a numeric id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseID("PUID", tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Equal(t, 0, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestApplyDropFailures(t *testing.T) {
	cases := map[string]*fakeCreds{
		"setgroups fails": {setgroupsErr: errors.New("boom")},
		"setgid fails":    {setgidErr: errors.New("boom")},
		"setuid fails":    {setuidErr: errors.New("boom")},
	}
	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			log, _ := newLogger()

			err := apply(envMap(nil), creds, log)

			require.Error(t, err)
		})
	}
}

func TestApplyVerificationFailsWhenIDsDoNotChange(t *testing.T) {
	creds := &fakeCreds{noopSetuid: true}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "1000", "PGID": "1000"}), creds, log)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification")
}

func TestApplyFailsWhenRootCanBeRegained(t *testing.T) {
	creds := &fakeCreds{allowRegain: true}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "1000", "PGID": "1000"}), creds, log)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "regain root")
}

func TestApplySucceedsWhenRootRegainIsRejected(t *testing.T) {
	creds := &fakeCreds{}
	log, _ := newLogger()

	err := apply(envMap(map[string]string{"PUID": "1000", "PGID": "1000"}), creds, log)

	require.NoError(t, err)
}

func TestDropPrivilegesNonRootIsNoop(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires an unprivileged process; running as root would drop the test process")
	}
	log, _ := newLogger()

	require.NoError(t, DropPrivileges(log))
}
