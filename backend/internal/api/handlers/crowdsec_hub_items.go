package handlers

import (
	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// addHubItemsReport adds the hub_items field to an import response when the archive carried a usable
// manifest. It diffs against the items installed right now, so the caller must hold dataMu. If the
// installed items cannot be read the field is left out rather than reporting everything as missing.
func (h *CrowdsecHandler) addHubItemsReport(resp map[string]any, manifest *crowdsec.ArchiveHubItems) {
	if manifest == nil {
		return
	}
	installed, err := crowdsec.InstalledHubItems(h.DataDir)
	if err != nil {
		logger.Log().WithError(err).Warn("crowdsec import: installed hub items could not be read")
		return
	}
	resp["hub_items"] = manifest.Report(installed)
}
