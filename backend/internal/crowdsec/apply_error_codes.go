package crowdsec

import (
	"errors"

	"github.com/Wikid82/charon/backend/internal/models"
)

// ApplyErrorCode maps a failed hub preset apply to its stable error code, or "" when the failure has none.
// The synchronous preset handler and the background install job share it so both report the same codes.
func ApplyErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrPresetCacheStale):
		return models.CrowdsecApplyErrCacheStale
	case errors.Is(err, ErrPresetHubUpdating):
		return models.CrowdsecApplyErrHubUpdating
	case errors.Is(err, ErrPresetLocalModified):
		return models.CrowdsecApplyErrLocalModified
	case errors.Is(err, ErrPresetApplyTimeout):
		return models.CrowdsecApplyErrTimeout
	case errors.Is(err, ErrHubUnreachable):
		return models.CrowdsecApplyErrHubUnreachable
	case errors.Is(err, ErrCSCLIUnavailable):
		return models.CrowdsecApplyErrCSCLIUnavailable
	}
	return ""
}
