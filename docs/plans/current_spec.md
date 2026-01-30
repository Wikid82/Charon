# Reddit Feedback Implementation Plan: Logs UI, Caddy Import, Settings 400 Errors

**Version:** 1.0
**Status:** Research Complete - Ready for Implementation
**Priority:** HIGH
**Created:** 2026-01-29
**Source:** Reddit user feedback

> **Note:** Previous active plan (E2E Test Architecture Fix) archived to [e2e_architecture_port80_spec.md](./e2e_architecture_port80_spec.md)

---

## Active Plan

See **[reddit_feedback_spec.md](./reddit_feedback_spec.md)** for the complete specification.

---

## Quick Reference

### Three Issues Addressed

1. **Logs UI on widescreen** - Fixed `h-96` height, multi-span entries
2. **Caddy import not working** - Silent skipping, cryptic errors
3. **Settings 400 errors** - CIDR/URL validation, unfriendly messages

### Key Files

| Issue | Primary File | Line |
|-------|-------------|------|
| Logs UI | `frontend/src/components/LiveLogViewer.tsx` | 435 |
| Import | `backend/internal/api/handlers/import_handler.go` | 297 |
| Settings | `backend/internal/api/handlers/settings_handler.go` | 84 |

### Implementation Timeline

- **Day 1:** Quick wins (responsive height, error messages, normalization)
- **Day 2:** Core features (compact mode, skipped hosts, validation)
- **Day 3:** Polish (density control, import directive UI, inline validation)

---

## Executive Summary

Three user-reported issues from Reddit:
1. **Logs UI** - Fixed height wastes screen space, entries wrap across multiple lines
2. **Caddy Import** - Silent failures, cryptic errors, missing feedback on skipped sites
3. **Settings 400** - Validation errors not user-friendly, missing auto-correction

**Root Causes Identified:**
- LiveLogViewer uses `h-96` fixed height, multi-span entries
- Import handler silently skips hosts without `reverse_proxy`
- Settings handler returns raw Go validation errors

**Solution:** Responsive UI, enhanced error messages, input normalization

---

*For full specification, see [reddit_feedback_spec.md](./reddit_feedback_spec.md)*
*Previous E2E plan archived to [e2e_architecture_port80_spec.md](./e2e_architecture_port80_spec.md)*
