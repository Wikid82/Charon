package upstreamdns

import (
	"errors"
	"net"
	"net/netip"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	// maxReplySize is the largest reply sent: the classic DNS limit, so a reply is
	// valid without an OPT record. Longer answer sets are trimmed, never marked TC.
	maxReplySize = 512
	// maxAddressesPerReply bounds the first try when trimming. A 512-byte reply
	// holds far fewer records, so the loop in buildAnswers stops quickly.
	maxAddressesPerReply = 64
)

// opCodeQuery is the standard query opcode (RFC 1035 section 4.1.1).
const opCodeQuery dnsmessage.OpCode = 0

// lookupResult is the outcome of one lookup of a rooted name.
type lookupResult struct {
	addrs []netip.Addr
	err   error
}

// request is a query the answerer serves: the header fields a reply copies and the
// only question.
type request struct {
	id       uint16
	recurse  bool
	question dnsmessage.Question
}

// parseQuery checks a packet. ok is false when the packet is dropped (unparsable
// header or a response). A non-nil early reply (NOTIMP, FORMERR, or an empty NOERROR
// for a class or type the answerer does not serve) is sent as it is. With neither,
// req names the question to look up.
func parseQuery(packet []byte) (req request, early []byte, ok bool) {
	var p dnsmessage.Parser
	hdr, err := p.Start(packet)
	if err != nil || hdr.Response {
		return request{}, nil, false
	}
	req = request{id: hdr.ID, recurse: hdr.RecursionDesired}
	if hdr.OpCode != opCodeQuery {
		return req, buildReply(req, dnsmessage.RCodeNotImplemented, nil, nil, 0), true
	}
	questions, err := p.AllQuestions()
	if err != nil || len(questions) != 1 {
		return req, buildReply(req, dnsmessage.RCodeFormatError, nil, nil, 0), true
	}
	req.question = questions[0]
	if q := req.question; q.Class != dnsmessage.ClassINET || (q.Type != dnsmessage.TypeA && q.Type != dnsmessage.TypeAAAA) {
		return req, buildReply(req, dnsmessage.RCodeSuccess, &q, nil, 0), true
	}
	return req, nil, true
}

// answer builds the reply for a packet from the result of its lookup. The filter
// removes addresses forward targets may not use; refused counts them. ok is false when
// the packet is dropped. The rules are the answer table of the design (D-34b-4).
func answer(query []byte, res lookupResult, filter Filter, ttl uint32) (reply []byte, refused int, ok bool) {
	req, early, ok := parseQuery(query)
	if !ok {
		return nil, 0, false
	}
	if early != nil {
		return early, 0, true
	}
	q := req.question
	if res.err != nil {
		if isNotFound(res.err) {
			return buildReply(req, dnsmessage.RCodeNameError, &q, nil, 0), 0, true
		}
		return buildReply(req, dnsmessage.RCodeServerFailure, &q, nil, 0), 0, true
	}
	asked, other := splitFamilies(res.addrs, q.Type == dnsmessage.TypeA)
	if len(asked) == 0 && len(other) == 0 {
		return buildReply(req, dnsmessage.RCodeNameError, &q, nil, 0), 0, true
	}
	if len(asked) == 0 {
		return buildReply(req, dnsmessage.RCodeSuccess, &q, nil, 0), 0, true
	}
	allowed := make([]netip.Addr, 0, len(asked))
	for _, a := range asked {
		if filter != nil && filter(a) {
			refused++
			continue
		}
		allowed = append(allowed, a)
	}
	return buildAnswers(req, allowed, ttl), refused, true
}

// splitFamilies sorts addresses into the asked family and the other one. An
// IPv4-mapped IPv6 address counts as IPv4.
func splitFamilies(addrs []netip.Addr, wantV4 bool) (asked, other []netip.Addr) {
	for _, a := range addrs {
		a = a.Unmap()
		if a.Is4() == wantV4 {
			asked = append(asked, a)
		} else {
			other = append(other, a)
		}
	}
	return asked, other
}

// buildAnswers writes as many addresses as fit in maxReplySize. It starts from a
// bounded count and drops one address at a time until the reply fits.
func buildAnswers(req request, addrs []netip.Addr, ttl uint32) []byte {
	n := min(len(addrs), maxAddressesPerReply)
	for {
		reply := buildReply(req, dnsmessage.RCodeSuccess, &req.question, addrs[:n], ttl)
		if (reply != nil && len(reply) <= maxReplySize) || n == 0 {
			return reply
		}
		n--
	}
}

// buildReply writes a response with the request id, the question and the given
// addresses as answers. The header sets QR and RA, copies RD, and never sets AA or TC.
// Name compression is on, so each answer names the question by a pointer. A builder
// failure returns nil, which the caller treats as a dropped reply.
func buildReply(req request, rcode dnsmessage.RCode, q *dnsmessage.Question, addrs []netip.Addr, ttl uint32) []byte {
	b := dnsmessage.NewBuilder(make([]byte, 0, maxReplySize), dnsmessage.Header{
		ID:                 req.id,
		Response:           true,
		RecursionDesired:   req.recurse,
		RecursionAvailable: true,
		RCode:              rcode,
	})
	b.EnableCompression()
	if b.StartQuestions() != nil {
		return nil
	}
	var name dnsmessage.Name
	if q != nil {
		if b.Question(*q) != nil {
			return nil
		}
		name = q.Name
	}
	if b.StartAnswers() != nil {
		return nil
	}
	for _, a := range addrs {
		if !writeAddressRecord(&b, name, a, ttl) {
			return nil
		}
	}
	out, err := b.Finish()
	if err != nil {
		return nil
	}
	return out
}

// writeAddressRecord appends one A or AAAA record.
func writeAddressRecord(b *dnsmessage.Builder, name dnsmessage.Name, a netip.Addr, ttl uint32) bool {
	hdr := dnsmessage.ResourceHeader{Name: name, Class: dnsmessage.ClassINET, TTL: ttl}
	if a.Is4() {
		return b.AResource(hdr, dnsmessage.AResource{A: a.As4()}) == nil
	}
	return b.AAAAResource(hdr, dnsmessage.AAAAResource{AAAA: a.As16()}) == nil
}

// isNotFound reports whether a lookup error means the name does not exist.
func isNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}
