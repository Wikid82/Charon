package dnsprovider

import (
	"sort"
	"sync"
)

// Registry is a thread-safe registry of DNS provider plugins.
type Registry struct {
	providers map[string]ProviderPlugin
	mu        sync.RWMutex
}

// globalRegistry is the singleton registry instance.
var globalRegistry = &Registry{
	providers: make(map[string]ProviderPlugin),
}

// Global returns the global provider registry.
func Global() *Registry {
	return globalRegistry
}

// NewRegistry creates a new registry instance for testing purposes.
// Use Global() for production code.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]ProviderPlugin),
	}
}

// Register adds a provider to the registry.
// Returns ErrInvalidPlugin if the provider type is empty,
// or ErrProviderAlreadyRegistered if the type is already registered.
func (r *Registry) Register(provider ProviderPlugin) error {
	if provider == nil {
		return ErrInvalidPlugin
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	providerType := provider.Type()
	if providerType == "" {
		return ErrInvalidPlugin
	}

	if _, exists := r.providers[providerType]; exists {
		return ErrProviderAlreadyRegistered
	}

	r.providers[providerType] = provider
	return nil
}

// Get retrieves a provider by type.
// Returns the provider and true if found, nil and false otherwise.
func (r *Registry) Get(providerType string) (ProviderPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[providerType]
	return provider, ok
}

// List returns all registered providers.
// The returned slice is sorted alphabetically by provider type.
func (r *Registry) List() []ProviderPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]ProviderPlugin, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}

	// Sort by type for consistent ordering
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Type() < providers[j].Type()
	})

	return providers
}

// Types returns all registered provider type identifiers.
// The returned slice is sorted alphabetically.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.providers))
	for t := range r.providers {
		types = append(types, t)
	}

	sort.Strings(types)
	return types
}

// IsSupported checks if a provider type is registered.
func (r *Registry) IsSupported(providerType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[providerType]
	return ok
}

// Unregister removes a provider from the registry.
// Used primarily for plugin unloading during shutdown.
// Safe to call with a type that doesn't exist.
func (r *Registry) Unregister(providerType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, providerType)
}

// Count returns the number of registered providers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// Clear removes all providers from the registry.
// Primarily used for testing.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = make(map[string]ProviderPlugin)
}
