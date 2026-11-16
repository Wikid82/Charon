package services

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

// monitorInputError marks a monitor request that is invalid. The handler maps it
// to 400; every other create error is a server error.
type monitorInputError struct{ msg string }

func (e *monitorInputError) Error() string { return e.msg }

// invalidMonitorInput returns an error the handler reports as a 400 with msg.
func invalidMonitorInput(msg string) error { return &monitorInputError{msg: msg} }

// IsMonitorInputError reports whether err is a rejected monitor request.
func IsMonitorInputError(err error) bool {
	var target *monitorInputError
	return errors.As(err, &target)
}

// isPolicyRefusal reports whether err is one of the destination policy refusals.
func isPolicyRefusal(err error) bool {
	return errors.Is(err, ErrForwardTargetAddress) || errors.Is(err, ErrOrthrusListenerTarget) || errors.Is(err, ErrAgentForwardTarget)
}

// monitorDestination returns the host and port a TCP, HTTP or HTTPS monitor probes.
// An HTTP or HTTPS URL without a port uses the scheme's default port.
func monitorDestination(monitorType, urlStr string) (host string, port int, err error) {
	if monitorType == "tcp" {
		var portStr string
		host, portStr, err = net.SplitHostPort(strings.TrimPrefix(strings.TrimSpace(urlStr), "tcp://"))
		if err != nil {
			return "", 0, invalidMonitorInput("TCP URL must be in host:port format")
		}
		port, err = parseMonitorPort(portStr)
		return host, port, err
	}
	u, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return "", 0, invalidMonitorInput("invalid URL format")
	}
	port = 80
	if monitorType == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, err = parseMonitorPort(u.Port())
		if err != nil {
			return "", 0, err
		}
	}
	return u.Hostname(), port, nil
}

// parseMonitorPort reads a port number from a monitor URL or address.
func parseMonitorPort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, invalidMonitorInput("monitor port must be between 1 and 65535")
	}
	return port, nil
}

// checkMonitorDestination applies the probe destination rules to a new monitor.
// Monitor types that do not probe a network address are not checked here.
func checkMonitorDestination(db *gorm.DB, monitorType, urlStr string) error {
	switch monitorType {
	case "tcp", "http", "https":
	default:
		return nil
	}
	host, port, err := monitorDestination(monitorType, urlStr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), forwardNameBudget)
	defer cancel()
	return CheckProbeTarget(ctx, db, host, port)
}

// ReportRefusedMonitors writes one warning per enabled monitor whose stored
// destination the probe policy refuses. It checks only destinations that need no
// name lookup: IP literals and names of this machine. Other refusals show when
// the probe runs. The warning names the monitor, never its address.
func (s *UptimeService) ReportRefusedMonitors() {
	if s == nil || s.DB == nil {
		return
	}
	var monitors []models.UptimeMonitor
	if err := s.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		logger.Log().WithError(err).Warn("uptime: could not list monitors for the destination check")
		return
	}
	for _, m := range monitors {
		monitorType := strings.ToLower(strings.TrimSpace(m.Type))
		host, port, err := monitorDestination(monitorType, m.URL)
		if err != nil || !refusedWithoutLookup(s.DB, host, port) {
			continue
		}
		logger.Log().WithFields(map[string]any{
			"monitor_id":   m.ID,
			"monitor_name": util.SanitizeForLog(m.Name),
			"monitor_type": util.SanitizeForLog(m.Type),
		}).Warn("Uptime monitor destination is not allowed; its checks will report down. Recreate it with another address.")
	}
}

// refusedWithoutLookup reports whether a destination is refused by the policy and
// needs no name lookup to decide. A name that is not this machine is never refused
// here.
func refusedWithoutLookup(db *gorm.DB, host string, port int) bool {
	name := normalizeForwardName(host)
	if parseIPOrLegacy(name) == nil && !namesThisMachine(name) {
		return false
	}
	return isPolicyRefusal(refuseForwardTarget(db, host, port, forwardVerdicts{policy: probePolicy()}))
}
