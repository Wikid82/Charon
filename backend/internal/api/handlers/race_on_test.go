//go:build race

package handlers

// raceOverheadFactor scales latency thresholds under the race detector, which
// slows execution of instrumented code several-fold (measured ~2-3x on P95).
const raceOverheadFactor = 3.0
