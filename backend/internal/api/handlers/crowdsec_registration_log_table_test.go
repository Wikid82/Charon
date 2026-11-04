package handlers

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// registrationLogWant is one log line expected from handleBouncerRegistration: its level and a substring of its message.
type registrationLogWant struct {
	level    logrus.Level
	contains string
}

// TestHandleBouncerRegistration_LogArms runs every outcome and registration-error arm of handleBouncerRegistration with
// a hook listener, and checks that exactly the expected lines are logged at the expected levels. The key text is
// set on every result and must never appear in the output.
func TestHandleBouncerRegistration_LogArms(t *testing.T) {
	registerErr := errors.New("bouncer registration failed")
	saveErr := errors.New("disk full")

	tests := []struct {
		name   string
		res    bouncerkey.EnsureResult
		regErr error
		want   []registrationLogWant
	}{
		{
			name:   "registration failed with a rejected env key",
			res:    bouncerkey.EnsureResult{EnvKey: logCaptureFullKey, EnvKeyRejected: true},
			regErr: registerErr,
			want: []registrationLogWant{
				{logrus.WarnLevel, "could not register a new key with CrowdSec"},
				{logrus.WarnLevel, "Failed to register bouncer"},
			},
		},
		{
			name:   "registration failed without a rejected env key",
			res:    bouncerkey.EnsureResult{EnvKey: logCaptureFullKey},
			regErr: registerErr,
			want:   []registrationLogWant{{logrus.WarnLevel, "Failed to register bouncer"}},
		},
		{
			name: "registered with the save succeeding",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeRegistered},
			want: []registrationLogWant{{logrus.InfoLevel, "CrowdSec Bouncer Registered Successfully"}},
		},
		{
			name: "registered over a rejected env key with the save succeeding",
			res: bouncerkey.EnsureResult{
				Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeRegistered, EnvKey: logCaptureFullKey, EnvKeyRejected: true,
			},
			want: []registrationLogWant{
				{logrus.WarnLevel, "Charon uses the key saved in"},
				{logrus.InfoLevel, "CrowdSec Bouncer Registered Successfully"},
			},
		},
		{
			name: "registered with the save failing",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeRegistered, SaveErr: saveErr},
			want: []registrationLogWant{
				{logrus.ErrorLevel, "Failed to save bouncer key to file"},
				{logrus.InfoLevel, "CrowdSec Bouncer Registered Successfully"},
			},
		},
		{
			name: "env key saved with the save succeeding",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeEnvKeySaved, EnvKey: logCaptureFullKey},
			want: []registrationLogWant{{logrus.InfoLevel, "validated and saved to key file"}},
		},
		{
			name: "env key saved with the save failing",
			res: bouncerkey.EnsureResult{
				Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeEnvKeySaved, EnvKey: logCaptureFullKey, SaveErr: saveErr,
			},
			want: []registrationLogWant{{logrus.ErrorLevel, "Failed to save bouncer key to file"}},
		},
		{
			name: "saved key kept with the env variable ignored",
			res: bouncerkey.EnsureResult{
				Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeSavedKeyValid, EnvKey: logCaptureFullKey, EnvKeyIgnored: true,
			},
			want: []registrationLogWant{{logrus.InfoLevel, "the key variable is ignored"}},
		},
		{
			name: "saved key kept with no env variable ignored",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeSavedKeyValid},
			want: nil,
		},
		{
			name: "unverified key",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeUnverified},
			want: []registrationLogWant{{logrus.WarnLevel, bouncerkey.UnverifiedMessage}},
		},
		{
			name: "held key valid with the held save succeeding",
			res:  bouncerkey.EnsureResult{Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeHeldKeyValid},
			want: []registrationLogWant{{logrus.InfoLevel, bouncerkey.HeldKeyInUseMessage(true)}},
		},
		{
			name: "held key valid with the held save failing",
			res: bouncerkey.EnsureResult{
				Key: logCaptureFullKey, Outcome: bouncerkey.OutcomeHeldKeyValid, HeldSaveErr: saveErr,
			},
			want: []registrationLogWant{{logrus.InfoLevel, bouncerkey.HeldKeyInUseMessage(false)}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
			buf, entries := captureDebugLogs(t)

			h.handleBouncerRegistration(tc.res, tc.regErr)

			got := drainLogEntries(entries)
			require.Len(t, got, len(tc.want), "unexpected log lines: %v", entryMessages(got))
			for i, w := range tc.want {
				assert.Equal(t, w.level, got[i].Level, "level of line %d", i)
				assert.Contains(t, got[i].Message, w.contains, "message of line %d", i)
			}
			assert.NotContains(t, buf.String(), logCaptureFullKey, "log output must not carry key text")
			for _, e := range got {
				assert.NotContains(t, e.Message, logCaptureFullKey, "a log message carries key text")
			}
		})
	}
}

// drainLogEntries returns every entry queued on the listener without blocking.
func drainLogEntries(entries <-chan *logrus.Entry) []*logrus.Entry {
	var out []*logrus.Entry
	for {
		select {
		case e := <-entries:
			out = append(out, e)
		default:
			return out
		}
	}
}

// entryMessages lists the messages of the given entries, for failure output.
func entryMessages(entries []*logrus.Entry) []string {
	msgs := make([]string, 0, len(entries))
	for _, e := range entries {
		msgs = append(msgs, e.Message)
	}
	return msgs
}
