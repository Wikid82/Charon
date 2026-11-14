package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// ForwardTargetGuard decides whether a stored, literal forward target must be
// left out of the generated config. It returns an error only when the check
// itself cannot run.
type ForwardTargetGuard interface {
	RefusesForwardTarget(host string, port int) (bool, error)
}

// dropRefusedForwardTargets returns the enabled hosts whose targets the guard
// accepts. A host with a refused target, on the host itself or on any location,
// is left out and a warning names it. The other hosts still apply, so one stored
// row cannot stop the whole config. Disabled hosts are not generated and are not
// checked. A nil guard keeps every host.
func dropRefusedForwardTargets(guard ForwardTargetGuard, hosts []models.ProxyHost) ([]models.ProxyHost, error) {
	if guard == nil {
		return hosts, nil
	}
	kept := make([]models.ProxyHost, 0, len(hosts))
	for _, h := range hosts {
		if !h.Enabled {
			kept = append(kept, h)
			continue
		}
		refused, err := hostHasRefusedTarget(guard, h)
		if err != nil {
			return nil, fmt.Errorf("check forward targets of host %s: %w", h.UUID, err)
		}
		if refused {
			logger.Log().WithField("host", h.UUID).
				Warn("Skipping proxy host: its forward target is not permitted by this proxy")
			continue
		}
		kept = append(kept, h)
	}
	return kept, nil
}

// hostHasRefusedTarget reports whether the host's own target or any location's
// target is refused.
func hostHasRefusedTarget(guard ForwardTargetGuard, h models.ProxyHost) (bool, error) {
	refused, err := guard.RefusesForwardTarget(h.ForwardHost, h.ForwardPort)
	if err != nil || refused {
		return refused, err
	}
	for _, loc := range h.Locations {
		refused, err := guard.RefusesForwardTarget(loc.ForwardHost, loc.ForwardPort)
		if err != nil || refused {
			return refused, err
		}
	}
	return false, nil
}

// errSnapshotRefusedTarget is returned by rollback when the snapshot routes a
// forward target that the guard refuses. Reinstating it would undo the filter.
var errSnapshotRefusedTarget = errors.New("snapshot routes a forward target this proxy no longer accepts")

// snapshotRoutesRefusedTarget reports whether any upstream dial in a generated
// config is refused by the guard. The config is walked as generic JSON, so every
// reverse proxy upstream is checked wherever it sits.
func snapshotRoutesRefusedTarget(guard ForwardTargetGuard, conf *Config) (bool, error) {
	if guard == nil || conf == nil {
		return false, nil
	}
	raw, err := json.Marshal(conf)
	if err != nil {
		return false, fmt.Errorf("encode snapshot: %w", err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, fmt.Errorf("decode snapshot: %w", err)
	}
	return walkDialsRefused(guard, doc)
}

// walkDialsRefused checks every "dial" string under node against the guard. A dial
// that does not split into host and port cannot name a listener and is skipped.
func walkDialsRefused(guard ForwardTargetGuard, node any) (bool, error) {
	switch v := node.(type) {
	case map[string]any:
		if dial, ok := v["dial"].(string); ok {
			host, portStr, err := net.SplitHostPort(dial)
			if port, convErr := strconv.Atoi(portStr); err == nil && convErr == nil {
				refused, checkErr := guard.RefusesForwardTarget(host, port)
				if checkErr != nil || refused {
					return refused, checkErr
				}
			}
		}
		for _, child := range v {
			if refused, err := walkDialsRefused(guard, child); err != nil || refused {
				return refused, err
			}
		}
	case []any:
		for _, child := range v {
			if refused, err := walkDialsRefused(guard, child); err != nil || refused {
				return refused, err
			}
		}
	}
	return false, nil
}
