package handlers

import (
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

type dashboardCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

func newDashboardCache() *dashboardCache {
	return &dashboardCache{
		entries: make(map[string]*cacheEntry),
	}
}

func (c *dashboardCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.data, true
}

func (c *dashboardCache) Set(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *dashboardCache) Invalidate(prefixes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				delete(c.entries, key)
				break
			}
		}
	}
}
