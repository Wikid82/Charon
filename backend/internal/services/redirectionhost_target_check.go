package services

import (
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// WarnRefusedRedirectTargets logs one warning per stored redirection host whose
// target_url would now be refused by validation. Those rows keep working: the
// generator writes their target as a literal. The warning names the host and
// its UUID, never the stored value. It returns the number of rows flagged, or 0
// when the rows cannot be read.
func WarnRefusedRedirectTargets(db *gorm.DB) int {
	var rows []models.RedirectionHost
	if err := db.Select("uuid", "domain_names", "target_url").Find(&rows).Error; err != nil {
		logger.Log().WithError(err).Warn("Failed to check stored redirection targets")
		return 0
	}

	refused := 0
	for i := range rows {
		if !strings.ContainsAny(rows[i].TargetURL, "{}") {
			continue
		}
		refused++
		logger.Log().
			WithField("host", rows[i].DomainNames).
			WithField("redirection_host_uuid", rows[i].UUID).
			Warn("Stored redirection target contains literal braces and is no longer accepted; it is served as a literal. Edit the target to percent-encode them.")
	}
	return refused
}
