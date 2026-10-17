package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// maxCSCLIDecisions bounds how many decisions one listing returns; further decisions are left out.
const maxCSCLIDecisions = 100_000

// cscliDecisionEntry is one element of the `cscli decisions list -o json` array. Current cscli prints alerts
// that carry their decisions in "decisions"; older cscli and the local API print the decisions directly. The
// nested decisions stay raw so they can be read one at a time.
type cscliDecisionEntry struct {
	cscliDecision
	Decisions json.RawMessage `json:"decisions"`
}

// decisionCollector accumulates decisions up to maxCSCLIDecisions.
type decisionCollector struct {
	decisions []CrowdSecDecision
	skipped   int
	truncated bool
}

// add records one decision, taking the scenario and creation time from the alert when the decision has none.
// It reports false once the cap is hit and no further row is wanted.
func (c *decisionCollector) add(d cscliDecision, scenario, createdAt string) bool {
	if d.Value == "" {
		c.skipped++
		return true
	}
	if len(c.decisions) >= maxCSCLIDecisions {
		c.truncated = true
		return false
	}
	if scenario == "" {
		scenario = d.Scenario
	}
	if createdAt == "" {
		createdAt = d.CreatedAt
	}
	var created time.Time
	if createdAt != "" {
		created, _ = time.Parse(time.RFC3339, createdAt)
	}
	c.decisions = append(c.decisions, CrowdSecDecision{
		ID:        d.ID,
		Origin:    d.Origin,
		Type:      d.Type,
		Scope:     d.Scope,
		Value:     d.Value,
		Duration:  d.Duration,
		Scenario:  scenario,
		CreatedAt: created,
		Until:     d.Until,
	})
	return true
}

// addEntry records the decisions of one array element and reports false once the cap is hit.
func (c *decisionCollector) addEntry(e *cscliDecisionEntry) (bool, error) {
	dec := json.NewDecoder(bytes.NewReader(e.Decisions))
	if tok, err := dec.Token(); err != nil || tok == nil {
		// absent or null: the element is itself a decision (an empty raw message reads as EOF)
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("parse nested decisions: %w", err)
		}
		return c.add(e.cscliDecision, "", ""), nil
	} else if d, ok := tok.(json.Delim); !ok || d != '[' {
		return false, errors.New("parse nested decisions: not an array")
	}
	if !dec.More() {
		return c.add(e.cscliDecision, "", ""), nil
	}
	var nested cscliDecision
	for dec.More() {
		nested = cscliDecision{}
		if err := dec.Decode(&nested); err != nil {
			return false, fmt.Errorf("parse nested decisions: %w", err)
		}
		if !c.add(nested, e.Scenario, e.CreatedAt) {
			return false, nil
		}
	}
	return true, nil
}

// parseCSCLIDecisions converts the output of `cscli decisions list -o json` into decisions. It accepts the
// alert-shaped output (each nested decision carries the alert's scenario and creation time) and the flat
// shape. Rows without a value are never listed; their number is returned as skipped. The document is read
// one row at a time and reading stops at maxCSCLIDecisions decisions; truncated reports that further
// decisions were left out.
func parseCSCLIDecisions(raw []byte) (decisions []CrowdSecDecision, skipped int, truncated bool, err error) {
	c := &decisionCollector{decisions: []CrowdSecDecision{}}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || string(trimmed) == "null" {
		return c.decisions, 0, false, nil
	}
	var doc json.RawMessage
	if err := crowdsec.UnmarshalCSCLIOutput(raw, &doc); err != nil {
		return nil, 0, false, fmt.Errorf("parse cscli decisions: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(doc))
	if tok, err := dec.Token(); err != nil {
		return nil, 0, false, fmt.Errorf("parse cscli decisions: %w", err)
	} else if d, ok := tok.(json.Delim); !ok || d != '[' {
		return nil, 0, false, errors.New("parse cscli decisions: not an array")
	}
	var e cscliDecisionEntry
	for dec.More() {
		e = cscliDecisionEntry{}
		if err := dec.Decode(&e); err != nil {
			return nil, 0, false, fmt.Errorf("parse cscli decisions: %w", err)
		}
		more, err := c.addEntry(&e)
		if err != nil {
			return nil, 0, false, fmt.Errorf("parse cscli decisions: %w", err)
		}
		if !more {
			break
		}
	}
	return c.decisions, c.skipped, c.truncated, nil
}
