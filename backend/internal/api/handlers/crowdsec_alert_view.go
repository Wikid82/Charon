package handlers

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// alertView is the alert the dashboard endpoint returns. It holds only the fields the UI reads
// (frontend/src/api/crowdsecDashboard.ts, CrowdSecAlert); everything else cscli prints about an alert
// (machine and alert identifiers, events, labels, the raw source and decisions) is not passed on.
type alertView struct {
	ID          int64  `json:"id"`
	Scenario    string `json:"scenario"`
	IP          string `json:"ip"`
	Message     string `json:"message"`
	EventsCount int    `json:"events_count"`
	StartAt     string `json:"start_at"`
	StopAt      string `json:"stop_at"`
	CreatedAt   string `json:"created_at"`
	Duration    string `json:"duration"`
	Type        string `json:"type"`
	Origin      string `json:"origin"`
}

// cscliAlert is one element of `cscli alerts list -o json`; only the fields alertView uses are decoded.
type cscliAlert struct {
	ID          int64  `json:"id"`
	Scenario    string `json:"scenario"`
	Message     string `json:"message"`
	EventsCount int    `json:"events_count"`
	StartAt     string `json:"start_at"`
	StopAt      string `json:"stop_at"`
	CreatedAt   string `json:"created_at"`
	Source      struct {
		IP    string `json:"ip"`
		Value string `json:"value"`
	} `json:"source"`
	Decisions []struct {
		Duration string `json:"duration"`
		Type     string `json:"type"`
		Origin   string `json:"origin"`
	} `json:"decisions"`
}

// view maps the alert to its response form. The type, origin and duration come from the first decision, which
// is absent for alerts that led to none.
func (a *cscliAlert) view() alertView {
	v := alertView{
		ID:          a.ID,
		Scenario:    a.Scenario,
		IP:          a.Source.Value,
		Message:     a.Message,
		EventsCount: a.EventsCount,
		StartAt:     a.StartAt,
		StopAt:      a.StopAt,
		CreatedAt:   a.CreatedAt,
	}
	if v.IP == "" {
		v.IP = a.Source.IP
	}
	if len(a.Decisions) > 0 {
		v.Duration = a.Decisions[0].Duration
		v.Type = a.Decisions[0].Type
		v.Origin = a.Decisions[0].Origin
	}
	return v
}

// parseCSCLIAlerts converts the output of `cscli alerts list -o json` into at most limit alert views.
func parseCSCLIAlerts(raw []byte, limit int) ([]alertView, error) {
	var alerts []cscliAlert
	if err := crowdsec.UnmarshalCSCLIOutput(raw, &alerts); err != nil {
		return nil, fmt.Errorf("parse cscli alerts: %w", err)
	}
	if len(alerts) > limit {
		alerts = alerts[:limit]
	}
	views := make([]alertView, len(alerts))
	for i := range alerts {
		views[i] = alerts[i].view()
	}
	return views, nil
}
