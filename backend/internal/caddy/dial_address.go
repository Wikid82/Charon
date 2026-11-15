package caddy

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DialAddress is one upstream dial from an advanced configuration, parsed the
// way Caddy reads a network address: an optional network prefix, a host and a
// port or port range.
type DialAddress struct {
	Network   string
	Host      string
	PortStart int
	PortEnd   int
}

// ErrUnsupportedDial is returned for a dial Charon cannot check: a placeholder,
// a unix or file-descriptor network, a UDP network, or an address that does not
// parse. Such a dial is refused rather than assumed harmless.
var ErrUnsupportedDial = errors.New("unsupported dial address")

// ParseDial parses one dial address. Only tcp, tcp4 and tcp6 (or no prefix) are
// accepted. The port is a number or an inclusive range such as 2000-2100.
func ParseDial(dial string) (DialAddress, error) {
	if strings.Contains(dial, "{") {
		return DialAddress{}, ErrUnsupportedDial
	}
	network := ""
	rest := dial
	if i := strings.Index(dial, "/"); i >= 0 {
		network = strings.ToLower(dial[:i])
		rest = dial[i+1:]
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return DialAddress{}, ErrUnsupportedDial
		}
	}
	host, portPart, err := net.SplitHostPort(rest)
	if err != nil || host == "" {
		return DialAddress{}, ErrUnsupportedDial
	}
	start, end, err := parsePortRange(portPart)
	if err != nil {
		return DialAddress{}, err
	}
	return DialAddress{Network: network, Host: host, PortStart: start, PortEnd: end}, nil
}

// parsePortRange reads "port" or "start-end" and returns the first and last port.
func parsePortRange(raw string) (first, last int, err error) {
	startStr, endStr, isRange := strings.Cut(raw, "-")
	if !isRange {
		endStr = startStr
	}
	first, err = parsePort(startStr)
	if err != nil {
		return 0, 0, err
	}
	last, err = parsePort(endStr)
	if err != nil {
		return 0, 0, err
	}
	if last < first {
		return 0, 0, ErrUnsupportedDial
	}
	return first, last, nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %q: %w", raw, ErrUnsupportedDial)
	}
	return port, nil
}
