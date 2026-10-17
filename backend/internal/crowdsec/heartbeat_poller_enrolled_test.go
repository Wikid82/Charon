package crowdsec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeartbeatPoller_IsEnrolledJSONDocument(t *testing.T) {
	t.Parallel()
	p := &HeartbeatPoller{}
	cases := map[string]bool{
		`{"console":{"enrolled":true}}`:                         true,
		`{"console":{"enrolled":false}}`:                        false,
		`{"console":{"enrolled":null}}`:                         false,
		`{"console":{"enrolled":"yes"}}`:                        false,
		`{"console":{"enrolled":1}}`:                            false,
		`{"console":{}}`:                                        false,
		`{"console":{"plan":"enrolled and connected console"}}`: false,
		// not a document with a console object: the text match decides
		`{"console":null}`:                                  false,
		`{"other":"enrolled and connected"}`:                true,
		"Your engine is enrolled and connected to console.": true,
		"You are not enrolled to the console":               false,
	}
	for in, want := range cases {
		assert.Equal(t, want, p.isEnrolled([]byte(in)), in)
	}
}
