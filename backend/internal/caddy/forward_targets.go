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
	return advancedConfigRefused(guard, h.AdvancedConfig)
}

// AdvancedDialGuard is optionally implemented by a ForwardTargetGuard. It decides
// whether one dial inside a host's advanced configuration must be left out.
type AdvancedDialGuard interface {
	RefusesAdvancedDial(dial string) (bool, error)
}

// advancedConfigRefused reports whether a host's advanced configuration reaches a
// refused destination. An unreadable configuration, dynamic upstreams and custom
// resolvers are refused outright. Each dial is then checked when the guard
// implements AdvancedDialGuard.
func advancedConfigRefused(guard ForwardTargetGuard, raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return true, nil
	}
	if HasDynamicUpstreams(doc) || HasTransportResolver(doc) || NameUpstreamWithoutResolver(doc) {
		return true, nil
	}
	adv, ok := guard.(AdvancedDialGuard)
	if !ok {
		return false, nil
	}
	err := WalkAdvancedDials(doc, func(dial string) error {
		refused, checkErr := adv.RefusesAdvancedDial(dial)
		if checkErr != nil {
			return checkErr
		}
		if refused {
			return errDialRefused
		}
		return nil
	})
	if errors.Is(err, errDialRefused) {
		return true, nil
	}
	return false, err
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

// errDialRefused stops a dial walk at the first refused dial.
var errDialRefused = errors.New("dial refused")

// walkDialsRefused checks every "dial" string under node against the guard. A dial
// that does not split into host and port cannot name a listener and is skipped.
func walkDialsRefused(guard ForwardTargetGuard, node any) (bool, error) {
	err := WalkDials(node, func(dial string) error {
		host, portStr, splitErr := net.SplitHostPort(dial)
		if splitErr != nil {
			return nil
		}
		port, convErr := strconv.Atoi(portStr)
		if convErr != nil {
			return nil
		}
		refused, checkErr := guard.RefusesForwardTarget(host, port)
		if checkErr != nil {
			return checkErr
		}
		if refused {
			return errDialRefused
		}
		return nil
	})
	if errors.Is(err, errDialRefused) {
		return true, nil
	}
	return false, err
}
