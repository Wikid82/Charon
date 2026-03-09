# Create Technical Spike

Create a time-boxed technical spike document for: **$ARGUMENTS**

Spikes research critical questions that must be answered before development can proceed. Each spike focuses on a specific technical decision with clear deliverables and timelines.

## Output File

Save to `docs/spikes/` directory. Name using pattern: `[category]-[short-description]-spike.md`

Examples:
- `api-copilot-integration-spike.md`
- `performance-realtime-audio-spike.md`
- `architecture-voice-pipeline-design-spike.md`

## Spike Document Template

```md
---
title: "[Spike Title]"
category: "Technical"
status: "Not Started"
priority: "High"
timebox: "1 week"
created: [YYYY-MM-DD]
updated: [YYYY-MM-DD]
owner: "[Owner]"
tags: ["technical-spike", "research"]
---

# [Spike Title]

## Summary

**Spike Objective:** [Clear, specific question or decision that needs resolution]

**Why This Matters:** [Impact on development/architecture decisions]

**Timebox:** [How much time allocated]

**Decision Deadline:** [When this must be resolved to avoid blocking development]

## Research Question(s)

**Primary Question:** [Main technical question that needs answering]

**Secondary Questions:**
- [Related question 1]
- [Related question 2]

## Investigation Plan

### Research Tasks

- [ ] [Specific research task 1]
- [ ] [Specific research task 2]
- [ ] [Create proof of concept/prototype]
- [ ] [Document findings and recommendations]

### Success Criteria

**This spike is complete when:**
- [ ] [Specific criteria 1]
- [ ] [Clear recommendation documented]
- [ ] [Proof of concept completed (if applicable)]

## Technical Context

**Related Components:** [System components affected by this decision]
**Dependencies:** [Other spikes or decisions that depend on resolving this]
**Constraints:** [Known limitations or requirements]

## Research Findings

### Investigation Results

[Document research findings, test results, evidence gathered]

### Prototype/Testing Notes

[Results from prototypes or technical experiments]

### External Resources

- [Link to relevant documentation]
- [Link to API references]

## Decision

### Recommendation

[Clear recommendation based on research findings]

### Rationale

[Why this approach was chosen over alternatives]

### Implementation Notes

[Key considerations for implementation]

### Follow-up Actions

- [ ] [Action item 1]
- [ ] [Update architecture documents]
- [ ] [Create implementation tasks]

## Status History

| Date   | Status         | Notes                    |
| ------ | -------------- | ------------------------ |
| [Date] | Not Started    | Spike created and scoped |
```

## Research Strategy

### Phase 1: Information Gathering
1. Search existing documentation and codebase
2. Analyse existing patterns and constraints
3. Research external resources (APIs, libraries, examples)

### Phase 2: Validation & Testing
1. Create focused prototypes to test hypotheses
2. Run targeted experiments
3. Document test results with evidence

### Phase 3: Decision & Documentation
1. Synthesise findings into clear recommendations
2. Document implementation guidance
3. Create follow-up tasks

## Categories

- **API Integration**: Third-party capabilities, auth, rate limits
- **Architecture & Design**: System decisions, design patterns
- **Performance & Scalability**: Bottlenecks, resource utilisation
- **Platform & Infrastructure**: Deployment, hosting considerations
- **Security & Compliance**: Auth, compliance constraints
- **User Experience**: Interaction patterns, accessibility
