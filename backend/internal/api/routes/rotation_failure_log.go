package routes

import (
	"errors"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// logRotationServiceFailure records why the rotation service could not be built.
// A key registry or database failure is an error. A key set that cannot be loaded
// is a warning, logged only when a current key was configured.
func logRotationServiceFailure(err error, keyConfigured bool) {
	switch {
	case errors.Is(err, crypto.ErrKeyRegistryUnavailable):
		logger.Log().WithError(err).Error("Encryption key registry could not be read or written; encrypted features are unavailable")
	case keyConfigured:
		logger.Log().WithError(err).Warn("Encryption keys could not be loaded; encrypted features are unavailable")
	}
}
