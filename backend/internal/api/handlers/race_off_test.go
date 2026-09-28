//go:build !race

package handlers

// raceOverheadFactor is 1 without the race detector: default limits stay strict.
const raceOverheadFactor = 1.0
