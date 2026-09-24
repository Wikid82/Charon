// Package ratelimit provides a bounded, goroutine-free, per-key token-bucket
// limiter shared by every request throttle in Charon.
//
// Algorithm: each key owns a golang.org/x/time/rate token bucket with capacity
// Burst that refills at Rate tokens per second. A bucket configured as "N
// requests per window W" (see PerWindow) admits at most N + T·N/W requests in
// any interval of length T, so it behaves like a sliding window without the
// boundary doubling of fixed windows, keeps constant state per key, and yields
// an exact Retry-After. Denied requests consume no tokens.
//
// Memory: buckets live in a fixed-size LRU (hashicorp/golang-lru/v2/simplelru)
// capped at Config.MaxKeys (DefaultMaxKeys by default). Buckets idle long
// enough to have fully refilled are swept lazily from the LRU tail during
// Allow, at most once per min(refill time, 1 minute). A fully refilled bucket
// is indistinguishable from a new one, so the sweep never changes a decision.
//
// Lifecycle: the package starts no goroutines, tickers or timers, so a limiter
// needs no shutdown and can be created freely in tests.
package ratelimit
