package ratelimit

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"golang.org/x/time/rate"
)

// DefaultMaxKeys is the number of distinct keys a limiter tracks when
// Config.MaxKeys is zero.
const DefaultMaxKeys = 10_000

// maxSweepInterval bounds how long idle buckets may linger before a sweep.
const maxSweepInterval = time.Minute

// Config configures a KeyedLimiter.
type Config struct {
	Rate    rate.Limit       // tokens per second; must be > 0 and finite
	Burst   int              // bucket capacity; must be >= 1
	MaxKeys int              // hard cap on tracked keys; 0 => DefaultMaxKeys; < 0 invalid
	Now     func() time.Time // clock; nil => time.Now (tests inject a fake)
}

// Decision is the outcome of a single Allow call.
type Decision struct {
	Allowed     bool
	RetryAfter  time.Duration // > 0 iff !Allowed
	FirstDenial bool          // first denial since this key was last allowed ("episode start")
}

type bucket struct {
	lim      *rate.Limiter
	lastSeen time.Time
	denied   bool
}

// KeyedLimiter is a concurrency-safe set of per-key token buckets with a hard
// memory bound. It starts no goroutines.
type KeyedLimiter struct {
	mu         sync.Mutex
	rate       rate.Limit
	burst      int
	maxKeys    int
	now        func() time.Time
	lru        *simplelru.LRU[string, *bucket]
	idleTTL    time.Duration
	sweepEvery time.Duration
	lastSweep  time.Time
}

func validateRate(r rate.Limit, burst int) error {
	f := float64(r)
	if math.IsNaN(f) || math.IsInf(f, 0) || r == rate.Inf || f <= 0 {
		return fmt.Errorf("ratelimit: rate must be positive and finite, got %v", f)
	}
	if burst < 1 {
		return fmt.Errorf("ratelimit: burst must be >= 1, got %d", burst)
	}
	return nil
}

// NewKeyedLimiter validates cfg and returns a ready limiter.
func NewKeyedLimiter(cfg Config) (*KeyedLimiter, error) {
	if err := validateRate(cfg.Rate, cfg.Burst); err != nil {
		return nil, err
	}
	if cfg.MaxKeys < 0 {
		return nil, errors.New("ratelimit: max keys must not be negative")
	}
	maxKeys := cfg.MaxKeys
	if maxKeys == 0 {
		maxKeys = DefaultMaxKeys
	}
	store, err := simplelru.NewLRU[string, *bucket](maxKeys, nil)
	if err != nil {
		return nil, fmt.Errorf("ratelimit: create store: %w", err)
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	k := &KeyedLimiter{maxKeys: maxKeys, now: now, lru: store, lastSweep: now()}
	k.setRate(cfg.Rate, cfg.Burst)
	return k, nil
}

// MustNewKeyedLimiter exists only because (*Cerberus).RateLimitMiddleware()
// builds its limiter from compile-time constants and has no error return;
// every other caller uses NewKeyedLimiter. It panics on an invalid config.
func MustNewKeyedLimiter(cfg Config) *KeyedLimiter {
	k, err := NewKeyedLimiter(cfg)
	if err != nil {
		panic(err)
	}
	return k
}

// setRate stores the budget and derives the sweep parameters. Caller holds mu
// (or is the constructor).
func (k *KeyedLimiter) setRate(r rate.Limit, burst int) {
	k.rate = r
	k.burst = burst
	// idleTTL is the full-refill time: after it, any bucket is back to Burst.
	secs := math.Ceil(float64(burst) / float64(r) * float64(time.Second))
	if secs >= math.MaxInt64 {
		k.idleTTL = time.Duration(math.MaxInt64)
	} else {
		k.idleTTL = time.Duration(secs)
	}
	k.sweepEvery = min(k.idleTTL, maxSweepInterval)
}

// Allow reports whether one request for key may proceed now. Denied requests
// consume nothing.
func (k *KeyedLimiter) Allow(key string) Decision {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := k.now()
	k.sweepLocked(now)

	b, ok := k.lru.Get(key)
	if !ok {
		b = &bucket{lim: rate.NewLimiter(k.rate, k.burst)}
		k.lru.Add(key, b)
	}
	b.lastSeen = now

	if b.lim.AllowN(now, 1) {
		b.denied = false
		return Decision{Allowed: true}
	}

	first := !b.denied
	b.denied = true
	return Decision{Allowed: false, RetryAfter: k.retryAfter(b.lim, now), FirstDenial: first}
}

func (k *KeyedLimiter) retryAfter(lim *rate.Limiter, now time.Time) time.Duration {
	missing := 1 - lim.TokensAt(now)
	d := time.Duration(missing / float64(k.rate) * float64(time.Second))
	if d <= 0 {
		return time.Nanosecond
	}
	return d
}

// sweepLocked removes buckets idle for at least idleTTL, walking from the LRU
// tail. LRU order equals lastSeen order, so the walk stops at the first key
// that is still recent. Caller holds mu.
func (k *KeyedLimiter) sweepLocked(now time.Time) {
	if now.Sub(k.lastSweep) < k.sweepEvery {
		return
	}
	k.lastSweep = now
	for {
		_, b, ok := k.lru.GetOldest()
		if !ok || now.Sub(b.lastSeen) < k.idleTTL {
			return
		}
		k.lru.RemoveOldest()
	}
}

// Reconfigure changes the budget. It is a no-op when unchanged; otherwise all
// buckets are reset.
func (k *KeyedLimiter) Reconfigure(r rate.Limit, burst int) error {
	if err := validateRate(r, burst); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if r == k.rate && burst == k.burst {
		return nil
	}
	k.setRate(r, burst)
	k.lru.Purge()
	return nil
}

// Len returns the number of tracked keys.
func (k *KeyedLimiter) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lru.Len()
}

// PerWindow converts "requests per window" into a token-bucket rate and burst.
// A non-positive window yields an infinite rate, which NewKeyedLimiter rejects.
func PerWindow(requests int, window time.Duration) (limit rate.Limit, burst int) {
	return rate.Limit(float64(requests) / window.Seconds()), requests
}
