package dnsprovider

import (
	"sync"
	"testing"
	"time"
)

// mockProvider is a test implementation of ProviderPlugin
type mockProvider struct {
	providerType string
}

func (m *mockProvider) Type() string {
	return m.providerType
}

func (m *mockProvider) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Type:      m.providerType,
		Name:      "Mock Provider",
		Version:   "1.0.0",
		IsBuiltIn: false,
	}
}

func (m *mockProvider) Init() error {
	return nil
}

func (m *mockProvider) Cleanup() error {
	return nil
}

func (m *mockProvider) ValidateCredentials(creds map[string]string) error {
	return nil
}

func (m *mockProvider) TestCredentials(creds map[string]string) error {
	return nil
}

func (m *mockProvider) RequiredCredentialFields() []CredentialFieldSpec {
	return []CredentialFieldSpec{}
}

func (m *mockProvider) OptionalCredentialFields() []CredentialFieldSpec {
	return []CredentialFieldSpec{}
}

func (m *mockProvider) SupportsMultiCredential() bool {
	return false
}

func (m *mockProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{}
}

func (m *mockProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return map[string]any{}
}

func (m *mockProvider) PropagationTimeout() time.Duration {
	return time.Minute
}

func (m *mockProvider) PollingInterval() time.Duration {
	return 2 * time.Second
}

// TestNewRegistry tests creating a new registry instance
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if r.Count() != 0 {
		t.Errorf("expected empty registry, got %d providers", r.Count())
	}
}

// TestGlobal tests the global registry singleton
func TestGlobal(t *testing.T) {
	r1 := Global()
	r2 := Global()

	if r1 != r2 {
		t.Error("Global() should return the same instance")
	}

	if r1 == nil {
		t.Fatal("Global() returned nil")
	}
}

// TestRegister tests registering providers
func TestRegister(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderPlugin
		wantErr  error
	}{
		{
			name:     "successful registration",
			provider: &mockProvider{providerType: "test-provider"},
			wantErr:  nil,
		},
		{
			name:     "nil provider",
			provider: nil,
			wantErr:  ErrInvalidPlugin,
		},
		{
			name:     "empty provider type",
			provider: &mockProvider{providerType: ""},
			wantErr:  ErrInvalidPlugin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(tt.provider)

			if err != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If successful, verify provider was registered
			if err == nil && tt.provider != nil {
				if !r.IsSupported(tt.provider.Type()) {
					t.Errorf("provider %s was not registered", tt.provider.Type())
				}
			}
		})
	}
}

// TestRegister_Duplicate tests duplicate registration
func TestRegister_Duplicate(t *testing.T) {
	r := NewRegistry()

	provider := &mockProvider{providerType: "duplicate-test"}

	// First registration should succeed
	err := r.Register(provider)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = r.Register(provider)
	if err != ErrProviderAlreadyRegistered {
		t.Errorf("expected ErrProviderAlreadyRegistered, got %v", err)
	}

	// Count should still be 1
	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}
}

// TestGet tests retrieving providers
func TestGet(t *testing.T) {
	r := NewRegistry()

	provider := &mockProvider{providerType: "test-get"}
	if err := r.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	tests := []struct {
		name         string
		providerType string
		wantOK       bool
	}{
		{
			name:         "existing provider",
			providerType: "test-get",
			wantOK:       true,
		},
		{
			name:         "non-existent provider",
			providerType: "does-not-exist",
			wantOK:       false,
		},
		{
			name:         "empty provider type",
			providerType: "",
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotOK := r.Get(tt.providerType)

			if gotOK != tt.wantOK {
				t.Errorf("Get() ok = %v, want %v", gotOK, tt.wantOK)
			}

			if tt.wantOK {
				if gotProvider == nil {
					t.Error("expected non-nil provider for existing type")
				}
				if gotProvider.Type() != tt.providerType {
					t.Errorf("got provider type %s, want %s", gotProvider.Type(), tt.providerType)
				}
			} else if gotProvider != nil {
				t.Error("expected nil provider for non-existent type")
			}
		})
	}
}

// TestList tests listing all providers
func TestList(t *testing.T) {
	r := NewRegistry()

	// Test empty registry
	list := r.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d providers", len(list))
	}

	// Register multiple providers
	providers := []*mockProvider{
		{providerType: "provider-c"},
		{providerType: "provider-a"},
		{providerType: "provider-b"},
	}

	for _, p := range providers {
		if err := r.Register(p); err != nil {
			t.Fatalf("failed to register provider %s: %v", p.Type(), err)
		}
	}

	// Get list (should be sorted)
	list = r.List()

	if len(list) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(list))
	}

	// Verify alphabetical ordering
	expectedOrder := []string{"provider-a", "provider-b", "provider-c"}
	for i, p := range list {
		if p.Type() != expectedOrder[i] {
			t.Errorf("list[%d] = %s, want %s", i, p.Type(), expectedOrder[i])
		}
	}
}

// TestTypes tests listing provider types
func TestTypes(t *testing.T) {
	r := NewRegistry()

	// Test empty registry
	types := r.Types()
	if len(types) != 0 {
		t.Errorf("expected empty types, got %d", len(types))
	}

	// Register multiple providers
	providers := []*mockProvider{
		{providerType: "zebra"},
		{providerType: "alpha"},
		{providerType: "beta"},
	}

	for _, p := range providers {
		if err := r.Register(p); err != nil {
			t.Fatalf("failed to register provider %s: %v", p.Type(), err)
		}
	}

	// Get types (should be sorted)
	types = r.Types()

	if len(types) != 3 {
		t.Fatalf("expected 3 types, got %d", len(types))
	}

	// Verify alphabetical ordering
	expectedOrder := []string{"alpha", "beta", "zebra"}
	for i, typ := range types {
		if typ != expectedOrder[i] {
			t.Errorf("types[%d] = %s, want %s", i, typ, expectedOrder[i])
		}
	}
}

// TestIsSupported tests checking provider support
func TestIsSupported(t *testing.T) {
	r := NewRegistry()

	// Register a provider
	provider := &mockProvider{providerType: "supported-test"}
	if err := r.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	tests := []struct {
		name         string
		providerType string
		want         bool
	}{
		{
			name:         "supported provider",
			providerType: "supported-test",
			want:         true,
		},
		{
			name:         "unsupported provider",
			providerType: "unsupported",
			want:         false,
		},
		{
			name:         "empty string",
			providerType: "",
			want:         false,
		},
		{
			name:         "case sensitivity",
			providerType: "SUPPORTED-TEST",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.IsSupported(tt.providerType)
			if got != tt.want {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.providerType, got, tt.want)
			}
		})
	}
}

// TestUnregister tests removing providers
func TestUnregister(t *testing.T) {
	r := NewRegistry()

	// Register a provider
	provider := &mockProvider{providerType: "unregister-test"}
	if err := r.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Verify it's registered
	if !r.IsSupported("unregister-test") {
		t.Fatal("provider not registered")
	}

	// Unregister it
	r.Unregister("unregister-test")

	// Verify it's gone
	if r.IsSupported("unregister-test") {
		t.Error("provider still registered after unregister")
	}

	// Unregister non-existent (should not panic)
	r.Unregister("does-not-exist")

	// Count should be 0
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}
}

// TestCount tests counting registered providers
func TestCount(t *testing.T) {
	r := NewRegistry()

	// Initial count should be 0
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}

	// Register providers
	for i := 1; i <= 5; i++ {
		provider := &mockProvider{providerType: "test-" + string(rune('a'+i-1))}
		if err := r.Register(provider); err != nil {
			t.Fatalf("failed to register provider: %v", err)
		}

		if r.Count() != i {
			t.Errorf("after registering %d providers, count = %d", i, r.Count())
		}
	}

	// Unregister one
	r.Unregister("test-a")
	if r.Count() != 4 {
		t.Errorf("after unregistering, count = %d, want 4", r.Count())
	}
}

// TestClear tests clearing all providers
func TestClear(t *testing.T) {
	r := NewRegistry()

	// Register multiple providers
	for i := 0; i < 3; i++ {
		provider := &mockProvider{providerType: "clear-test-" + string(rune('a'+i))}
		if err := r.Register(provider); err != nil {
			t.Fatalf("failed to register provider: %v", err)
		}
	}

	// Verify count
	if r.Count() != 3 {
		t.Fatalf("expected count 3, got %d", r.Count())
	}

	// Clear registry
	r.Clear()

	// Verify empty
	if r.Count() != 0 {
		t.Errorf("after clear, count = %d, want 0", r.Count())
	}

	// Verify no providers are supported
	if r.IsSupported("clear-test-a") {
		t.Error("provider still supported after clear")
	}

	// List should be empty
	if len(r.List()) != 0 {
		t.Errorf("list not empty after clear, got %d providers", len(r.List()))
	}
}

// TestConcurrency tests thread-safe operations
func TestConcurrency(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			provider := &mockProvider{providerType: "concurrent-" + string(rune('a'+n))}
			_ = r.Register(provider)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_ = r.Types()
			_ = r.IsSupported("concurrent-a")
			_, _ = r.Get("concurrent-a")
		}()
	}

	wg.Wait()

	// Verify final state
	if r.Count() != 10 {
		t.Errorf("expected 10 providers after concurrent registration, got %d", r.Count())
	}
}

// TestRegistry_Operations tests combined operations
func TestRegistry_Operations(t *testing.T) {
	r := NewRegistry()

	// Start with empty registry
	if r.Count() != 0 {
		t.Errorf("new registry should be empty")
	}

	// Register providers
	p1 := &mockProvider{providerType: "provider1"}
	p2 := &mockProvider{providerType: "provider2"}
	p3 := &mockProvider{providerType: "provider3"}

	if err := r.Register(p1); err != nil {
		t.Fatalf("failed to register p1: %v", err)
	}
	if err := r.Register(p2); err != nil {
		t.Fatalf("failed to register p2: %v", err)
	}
	if err := r.Register(p3); err != nil {
		t.Fatalf("failed to register p3: %v", err)
	}

	// Verify all are registered
	for _, p := range []ProviderPlugin{p1, p2, p3} {
		if !r.IsSupported(p.Type()) {
			t.Errorf("provider %s not registered", p.Type())
		}

		retrieved, ok := r.Get(p.Type())
		if !ok {
			t.Errorf("failed to get provider %s", p.Type())
		}
		if retrieved.Type() != p.Type() {
			t.Errorf("retrieved wrong provider: got %s, want %s", retrieved.Type(), p.Type())
		}
	}

	// List should contain all 3
	list := r.List()
	if len(list) != 3 {
		t.Errorf("list length = %d, want 3", len(list))
	}

	// Types should contain all 3
	types := r.Types()
	if len(types) != 3 {
		t.Errorf("types length = %d, want 3", len(types))
	}

	// Unregister one
	r.Unregister("provider2")

	// Verify count decreased
	if r.Count() != 2 {
		t.Errorf("after unregister, count = %d, want 2", r.Count())
	}

	// Verify p2 is gone
	if r.IsSupported("provider2") {
		t.Error("provider2 still supported after unregister")
	}

	// Clear all
	r.Clear()

	// Verify empty
	if r.Count() != 0 {
		t.Errorf("after clear, count = %d, want 0", r.Count())
	}
	if len(r.List()) != 0 {
		t.Error("list not empty after clear")
	}
	if len(r.Types()) != 0 {
		t.Error("types not empty after clear")
	}
}
