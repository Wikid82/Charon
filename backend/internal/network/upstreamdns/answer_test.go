package upstreamdns

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

// buildQuery returns a query packet for one question.
func buildQuery(t *testing.T, id uint16, name string, typ dnsmessage.Type, class dnsmessage.Class) []byte {
	t.Helper()
	n, err := dnsmessage.NewName(name)
	require.NoError(t, err)
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id, RecursionDesired: true})
	b.EnableCompression()
	require.NoError(t, b.StartQuestions())
	require.NoError(t, b.Question(dnsmessage.Question{Name: n, Type: typ, Class: class}))
	out, err := b.Finish()
	require.NoError(t, err)
	return out
}

// parsedReply is the part of a reply the tests check.
type parsedReply struct {
	header  dnsmessage.Header
	answers []netip.Addr
	raw     []byte
}

func parseReply(t *testing.T, raw []byte) parsedReply {
	t.Helper()
	var p dnsmessage.Parser
	hdr, err := p.Start(raw)
	require.NoError(t, err)
	_, err = p.AllQuestions()
	require.NoError(t, err)
	records, err := p.AllAnswers()
	require.NoError(t, err)
	out := parsedReply{header: hdr, raw: raw}
	for _, r := range records {
		switch body := r.Body.(type) {
		case *dnsmessage.AResource:
			out.answers = append(out.answers, netip.AddrFrom4(body.A))
		case *dnsmessage.AAAAResource:
			out.answers = append(out.answers, netip.AddrFrom16(body.AAAA))
		}
	}
	return out
}

func addrs(t *testing.T, raw ...string) []netip.Addr {
	t.Helper()
	out := make([]netip.Addr, 0, len(raw))
	for _, s := range raw {
		out = append(out, netip.MustParseAddr(s))
	}
	return out
}

func refuseLinkLocal(a netip.Addr) bool { return a.IsLinkLocalUnicast() }

func TestAnswer_Rules(t *testing.T) {
	t.Parallel()
	notFound := &net.DNSError{Err: "no such host", Name: "app.test.", IsNotFound: true}
	tests := map[string]struct {
		qtype     dnsmessage.Type
		res       lookupResult
		filter    Filter
		rcode     dnsmessage.RCode
		answers   []string
		refused   int
		answerRec bool
	}{
		"A all allowed": {
			qtype: dnsmessage.TypeA, res: lookupResult{addrs: addrs(t, "10.0.0.1", "10.0.0.2")},
			rcode: dnsmessage.RCodeSuccess, answers: []string{"10.0.0.1", "10.0.0.2"},
		},
		"A some refused": {
			qtype: dnsmessage.TypeA, res: lookupResult{addrs: addrs(t, "169.254.1.1", "10.0.0.2")}, filter: refuseLinkLocal,
			rcode: dnsmessage.RCodeSuccess, answers: []string{"10.0.0.2"}, refused: 1,
		},
		"A all refused is no data": {
			qtype: dnsmessage.TypeA, res: lookupResult{addrs: addrs(t, "169.254.1.1")}, filter: refuseLinkLocal,
			rcode: dnsmessage.RCodeSuccess, refused: 1,
		},
		"A family empty while AAAA exists is no data": {
			qtype: dnsmessage.TypeA, res: lookupResult{addrs: addrs(t, "fd00::5")},
			rcode: dnsmessage.RCodeSuccess,
		},
		"AAAA allowed": {
			qtype: dnsmessage.TypeAAAA, res: lookupResult{addrs: addrs(t, "fd12::5", "10.0.0.1")},
			rcode: dnsmessage.RCodeSuccess, answers: []string{"fd12::5"},
		},
		"AAAA family empty while A exists is no data": {
			qtype: dnsmessage.TypeAAAA, res: lookupResult{addrs: addrs(t, "10.0.0.1")},
			rcode: dnsmessage.RCodeSuccess,
		},
		"IPv4 mapped address counts as IPv4": {
			qtype: dnsmessage.TypeA, res: lookupResult{addrs: addrs(t, "::ffff:10.0.0.7")},
			rcode: dnsmessage.RCodeSuccess, answers: []string{"10.0.0.7"},
		},
		"not found is NXDOMAIN": {
			qtype: dnsmessage.TypeA, res: lookupResult{err: notFound}, rcode: dnsmessage.RCodeNameError,
		},
		"empty result without error is NXDOMAIN": {
			qtype: dnsmessage.TypeA, res: lookupResult{}, rcode: dnsmessage.RCodeNameError,
		},
		"other lookup error is SERVFAIL": {
			qtype: dnsmessage.TypeA, res: lookupResult{err: errors.New("timeout")}, rcode: dnsmessage.RCodeServerFailure,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			query := buildQuery(t, 0x1234, "app.test.", tc.qtype, dnsmessage.ClassINET)
			reply, refused, ok := answer(query, tc.res, tc.filter, 5)
			require.True(t, ok)
			assert.Equal(t, tc.refused, refused)
			got := parseReply(t, reply)
			assert.Equal(t, uint16(0x1234), got.header.ID)
			assert.True(t, got.header.Response)
			assert.True(t, got.header.RecursionAvailable)
			assert.True(t, got.header.RecursionDesired, "RD is copied from the query")
			assert.False(t, got.header.Authoritative)
			assert.False(t, got.header.Truncated)
			assert.Equal(t, tc.rcode, got.header.RCode)
			var want []netip.Addr
			for _, s := range tc.answers {
				want = append(want, netip.MustParseAddr(s))
			}
			assert.Equal(t, want, got.answers)
		})
	}
}

func TestAnswer_ReplyNamesQuestionByPointer(t *testing.T) {
	t.Parallel()
	query := buildQuery(t, 7, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET)
	reply, _, ok := answer(query, lookupResult{addrs: addrs(t, "10.0.0.1")}, nil, 5)
	require.True(t, ok)
	// Header (12) + question name "app.test." (10) + type and class (4) puts the
	// first answer name at offset 26, where it is a pointer to offset 12.
	require.GreaterOrEqual(t, len(reply), 28)
	assert.Equal(t, []byte{0xC0, 0x0C}, reply[26:28])
}

func TestAnswer_NonAddressQueriesGetNoData(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		typ   dnsmessage.Type
		class dnsmessage.Class
	}{
		"MX":            {dnsmessage.TypeMX, dnsmessage.ClassINET},
		"CHAOS class A": {dnsmessage.TypeA, dnsmessage.ClassCHAOS},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			query := buildQuery(t, 9, "app.test.", tc.typ, tc.class)
			reply, _, ok := answer(query, lookupResult{addrs: addrs(t, "10.0.0.1")}, nil, 5)
			require.True(t, ok)
			got := parseReply(t, reply)
			assert.Equal(t, dnsmessage.RCodeSuccess, got.header.RCode)
			assert.Empty(t, got.answers)
		})
	}
}

func TestAnswer_DropsAndErrorReplies(t *testing.T) {
	t.Parallel()
	valid := buildQuery(t, 3, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET)

	_, _, ok := answer([]byte{0x01, 0x02, 0x03}, lookupResult{}, nil, 5)
	assert.False(t, ok, "an unparsable header is dropped")

	response := append([]byte(nil), valid...)
	response[2] |= 0x80 // QR bit
	_, _, ok = answer(response, lookupResult{}, nil, 5)
	assert.False(t, ok, "a response is dropped")

	status := append([]byte(nil), valid...)
	status[2] = (status[2] & 0x87) | (2 << 3) // OPCODE STATUS
	reply, _, ok := answer(status, lookupResult{}, nil, 5)
	require.True(t, ok)
	assert.Equal(t, dnsmessage.RCodeNotImplemented, parseReply(t, reply).header.RCode)

	two := buildTwoQuestions(t)
	reply, _, ok = answer(two, lookupResult{}, nil, 5)
	require.True(t, ok)
	assert.Equal(t, dnsmessage.RCodeFormatError, parseReply(t, reply).header.RCode)

	truncated := valid[:len(valid)-2]
	reply, _, ok = answer(truncated, lookupResult{}, nil, 5)
	require.True(t, ok)
	assert.Equal(t, dnsmessage.RCodeFormatError, parseReply(t, reply).header.RCode)
}

// buildTwoQuestions returns a packet whose header claims two questions.
func buildTwoQuestions(t *testing.T) []byte {
	t.Helper()
	n, err := dnsmessage.NewName("app.test.")
	require.NoError(t, err)
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: 4, RecursionDesired: true})
	require.NoError(t, b.StartQuestions())
	q := dnsmessage.Question{Name: n, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}
	require.NoError(t, b.Question(q))
	require.NoError(t, b.Question(q))
	out, err := b.Finish()
	require.NoError(t, err)
	return out
}

// longName returns a 253-octet wire name of four 62-character labels.
func longName() string {
	label := strings.Repeat("a", 62)
	return strings.Join([]string{label, label, label, label}, ".") + "."
}

func TestAnswer_TrimsToReplySizeWithoutTC(t *testing.T) {
	t.Parallel()
	var many []netip.Addr
	var manyV6 []netip.Addr
	for i := range 40 {
		many = append(many, netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)}))
		manyV6 = append(manyV6, netip.AddrFrom16([16]byte{0xfd, 15: byte(i + 1)}))
	}
	name := longName()

	query := buildQuery(t, 11, name, dnsmessage.TypeA, dnsmessage.ClassINET)
	reply, _, ok := answer(query, lookupResult{addrs: many}, nil, 5)
	require.True(t, ok)
	assert.LessOrEqual(t, len(reply), maxReplySize)
	got := parseReply(t, reply)
	assert.False(t, got.header.Truncated)
	assert.Len(t, got.answers, 15)

	query = buildQuery(t, 12, name, dnsmessage.TypeAAAA, dnsmessage.ClassINET)
	reply, _, ok = answer(query, lookupResult{addrs: manyV6}, nil, 5)
	require.True(t, ok)
	assert.LessOrEqual(t, len(reply), maxReplySize)
	gotV6 := parseReply(t, reply)
	assert.False(t, gotV6.header.Truncated)
	assert.Len(t, gotV6.answers, 8)
}

func TestAnswer_NilFilterAcceptsEverything(t *testing.T) {
	t.Parallel()
	query := buildQuery(t, 5, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET)
	reply, refused, ok := answer(query, lookupResult{addrs: addrs(t, "169.254.1.1")}, nil, 5)
	require.True(t, ok)
	assert.Zero(t, refused)
	assert.Len(t, parseReply(t, reply).answers, 1)
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()
	assert.True(t, isNotFound(&net.DNSError{IsNotFound: true}))
	assert.False(t, isNotFound(&net.DNSError{IsTimeout: true}))
	assert.False(t, isNotFound(errors.New("other")))
}
