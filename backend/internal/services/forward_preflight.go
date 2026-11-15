package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// Budgets for the name lookups of one save, taken before the write lock.
const (
	// forwardNameBudget bounds all lookups of one save.
	forwardNameBudget = 5 * time.Second
	// forwardNameTimeout bounds one lookup.
	forwardNameTimeout = 2 * time.Second
	// forwardNameConcurrency bounds the parallel lookups of one save.
	forwardNameConcurrency = 4
	// forwardNamePositiveTTL and forwardNameNegativeTTL bound how long a verdict is reused.
	forwardNamePositiveTTL = 30 * time.Second
	forwardNameNegativeTTL = 10 * time.Second
)

// nameVerdict is the outcome of one name lookup. A verdict with err set means the
// name's addresses are unknown; policy checks then accept it, and reserved-port
// checks refuse it.
type nameVerdict struct {
	addrs []netip.Addr
	err   error
}

// forwardVerdicts is everything the in-lock checks need: the probe policy as it
// was when the save started, and the verdict of every name the save uses.
type forwardVerdicts struct {
	policy network.AddressPolicy
	names  map[string]nameVerdict
}

// errNoAddresses is the verdict of a name that resolved to nothing.
var errNoAddresses = errors.New("name resolved to no addresses")

// cachedVerdict is a verdict with its expiry.
type cachedVerdict struct {
	verdict nameVerdict
	expires time.Time
}

// verdictCache reuses name verdicts for a short time, so repeated saves and
// imports do not repeat the same lookups. Failures are reused for less time.
type verdictCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]cachedVerdict
}

// forwardNameCache is the process cache of name verdicts.
var forwardNameCache = &verdictCache{now: time.Now, entries: make(map[string]cachedVerdict)}

func (c *verdictCache) get(name string) (nameVerdict, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[name]
	if !ok || !c.now().Before(entry.expires) {
		delete(c.entries, name)
		return nameVerdict{}, false
	}
	return entry.verdict, true
}

func (c *verdictCache) put(name string, v nameVerdict) {
	ttl := forwardNamePositiveTTL
	if v.err != nil {
		ttl = forwardNameNegativeTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[name] = cachedVerdict{verdict: v, expires: c.now().Add(ttl)}
}

// forwardNamesOf returns the raw forward hosts a proxy host uses: its own target,
// every location target and the dial hosts of its advanced configuration.
func forwardNamesOf(host *models.ProxyHost) []string {
	raws := []string{host.ForwardHost}
	for i := range host.Locations {
		raws = append(raws, host.Locations[i].ForwardHost)
	}
	return append(raws, advancedDialHosts(host.AdvancedConfig)...)
}

// advancedDialHosts returns the host of each dial in an advanced configuration that
// parses. Other dials are refused by the save path itself, so they are skipped here.
func advancedDialHosts(raw string) []string {
	if raw == "" {
		return nil
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil
	}
	var hosts []string
	_ = caddy.WalkDials(doc, func(dial string) error {
		if addr, err := caddy.ParseDial(dial); err == nil {
			hosts = append(hosts, addr.Host)
		}
		return nil
	})
	return hosts
}

// preflightHostNames runs the name lookups of one proxy host save within the
// save budget. It must run before the write lock is taken.
func preflightHostNames(host *models.ProxyHost) forwardVerdicts {
	ctx, cancel := context.WithTimeout(context.Background(), forwardNameBudget)
	defer cancel()
	return preflightForwardNames(ctx, forwardNamesOf(host))
}

// preflightForwardNames collects the verdict of every name in rawHosts. Literals,
// the agent-name form and names that are this machine need no lookup. Cached
// verdicts are reused. The rest are resolved concurrently. Once ctx is done, the
// remaining names get a context error as their verdict and are not cached.
func preflightForwardNames(ctx context.Context, rawHosts []string) forwardVerdicts {
	v := forwardVerdicts{policy: probePolicy(), names: make(map[string]nameVerdict)}
	var pending []string
	for _, raw := range rawHosts {
		if isAgentForwardTarget(raw) {
			continue
		}
		name := normalizeForwardName(forwardTargetHost(raw))
		if name == "" || parseIPOrLegacy(name) != nil || namesThisMachine(name) {
			continue
		}
		if _, done := v.names[name]; done {
			continue
		}
		if cached, ok := forwardNameCache.get(name); ok {
			v.names[name] = cached
			continue
		}
		v.names[name] = nameVerdict{}
		pending = append(pending, name)
	}
	if len(pending) == 0 {
		return v
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, forwardNameConcurrency)
	for _, name := range pending {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			verdict := lookupNameVerdict(ctx, name)
			mu.Lock()
			v.names[name] = verdict
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	return v
}

// lookupNameVerdict resolves one name under the lookup timeout and the save budget,
// and caches the result unless the budget ran out.
func lookupNameVerdict(ctx context.Context, name string) nameVerdict {
	if err := ctx.Err(); err != nil {
		return nameVerdict{err: fmt.Errorf("name lookup skipped: %w", err)}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, forwardNameTimeout)
	defer cancel()
	ips, err := lookupForwardHost(lookupCtx, name)
	var verdict nameVerdict
	switch {
	case err != nil:
		verdict.err = fmt.Errorf("name lookup failed: %w", err)
	case len(ips) == 0:
		verdict.err = errNoAddresses
	default:
		for _, a := range ips {
			if ip, ok := netip.AddrFromSlice(a.IP); ok {
				verdict.addrs = append(verdict.addrs, ip.Unmap())
			}
		}
	}
	if ctx.Err() == nil {
		forwardNameCache.put(name, verdict)
	}
	return verdict
}

// refuseAdvancedConfigTargets applies the forward target rules to every dial in a
// host's advanced configuration. A dynamic upstream list, a custom resolver, a
// dial that cannot be checked, or a dial that reaches a refused address or a
// reserved port on this machine is refused. It runs inside the write lock.
func refuseAdvancedConfigTargets(tx *gorm.DB, raw string, v forwardVerdicts) error {
	if raw == "" {
		return nil
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return fmt.Errorf("invalid advanced_config JSON: %w", err)
	}
	if caddy.HasDynamicUpstreams(doc) || caddy.HasTransportResolver(doc) {
		return ErrForwardTargetAddress
	}
	return caddy.WalkDials(doc, func(dial string) error {
		target, err := caddy.ParseDial(dial)
		if err != nil {
			return ErrForwardTargetAddress
		}
		if refusal := refuseForwardTarget(tx, target.Host, 0, v); refusal != nil {
			return refusal
		}
		ports, err := reservedListenerPorts(tx)
		if err != nil {
			return fmt.Errorf("check advanced config dial: %w", err)
		}
		for port := range ports {
			if port < target.PortStart || port > target.PortEnd {
				continue
			}
			if refusal := refuseForwardTarget(tx, target.Host, port, v); refusal != nil {
				return refusal
			}
		}
		return nil
	})
}
