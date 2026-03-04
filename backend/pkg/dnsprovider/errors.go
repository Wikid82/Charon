package dnsprovider

import "errors"

// Common errors returned by the plugin system.
var (
	// ErrProviderNotFound is returned when a requested provider type is not registered.
	ErrProviderNotFound = errors.New("dns provider not found")

	// ErrProviderAlreadyRegistered is returned when attempting to register
	// a provider with a type that is already registered.
	ErrProviderAlreadyRegistered = errors.New("dns provider already registered")

	// ErrInvalidPlugin is returned when a plugin doesn't meet requirements
	// (e.g., nil plugin, empty type, missing required symbol).
	ErrInvalidPlugin = errors.New("invalid plugin: missing required fields or interface")

	// ErrSignatureMismatch is returned when a plugin's signature doesn't match
	// the expected signature in the allowlist.
	ErrSignatureMismatch = errors.New("plugin signature does not match allowlist")

	// ErrPluginNotInAllowlist is returned when attempting to load a plugin
	// that isn't in the configured allowlist.
	ErrPluginNotInAllowlist = errors.New("plugin not in allowlist")

	// ErrInterfaceVersionMismatch is returned when a plugin was built against
	// a different interface version than the host application.
	ErrInterfaceVersionMismatch = errors.New("plugin interface version mismatch")

	// ErrPluginLoadFailed is returned when the Go plugin system fails to load
	// a .so file (e.g., missing symbol, incompatible Go version).
	ErrPluginLoadFailed = errors.New("failed to load plugin")

	// ErrPluginInitFailed is returned when a plugin's Init() method returns an error.
	ErrPluginInitFailed = errors.New("plugin initialization failed")

	// ErrPluginDisabled is returned when attempting to use a disabled plugin.
	ErrPluginDisabled = errors.New("plugin is disabled")

	// ErrCredentialsInvalid is returned when credential validation fails.
	ErrCredentialsInvalid = errors.New("invalid credentials")

	// ErrCredentialsTestFailed is returned when credential testing fails.
	ErrCredentialsTestFailed = errors.New("credential test failed")
)
