# Security Headers Investigation Report

**Date**: December 18, 2025
**Issue**: Quick Start Presets Not Applying / Score Not Showing + Potential Redundancy

---

## Executive Summary

After tracing the complete data flow from frontend → API → backend → database, I identified **two root causes** and a **design clarity issue**:

1. **Root Cause #1 - Type Mismatch**: The frontend API expects `SecurityHeaderPreset` type with a `score` field, but the backend returns `SecurityHeaderProfile[]` with a `security_score` field
2. **Root Cause #2 - Presets ARE Working**: The presets actually DO work, but the UI displays `preset.score` which doesn't exist on the backend response
3. **Redundancy Question**: Quick Start and System Presets serve DIFFERENT purposes (one creates new profiles, one shows existing system presets), but the UX doesn't make this clear

---

## Complete Data Flow Trace

### 1. Frontend Component Flow (`SecurityHeaders.tsx`)

```
Component Mounts
    ↓
useSecurityHeaderProfiles() → GET /api/v1/security/headers/profiles
    → Returns ALL profiles (custom + system presets)
    → Splits into: presetProfiles (is_preset=true) + customProfiles (is_preset=false)
    ↓
useSecurityHeaderPresets() → GET /api/v1/security/headers/presets
    → Returns preset TEMPLATES (not saved profiles)
    → Used for "Quick Start Presets" section
```

**Key Insight**: There are TWO separate data sources:

1. **Profiles** (`/profiles`) - Saved profiles in the database (includes system presets with `is_preset=true`)
2. **Presets** (`/presets`) - Template definitions for creating new profiles (not stored in DB)

### 2. Quick Start Presets Section (Lines 120-145)

```tsx
// Frontend expects this type:
export interface SecurityHeaderPreset {
  type: 'basic' | 'strict' | 'paranoid';
  name: string;
  description: string;
  score: number;           // <-- FRONTEND EXPECTS "score"
  config: Partial<SecurityHeaderProfile>;
}

// Renders:
{presets.map((preset) => (
  <div className="text-2xl font-bold">{preset.score}</div>  // <-- Uses preset.score
  <Button onClick={() => handleApplyPreset(preset.type)}>Apply Preset</Button>
))}
```

### 3. Backend GetPresets Handler (Lines 220-224)

```go
func (h *SecurityHeadersHandler) GetPresets(c *gin.Context) {
    presets := h.service.GetPresets()  // Returns []models.SecurityHeaderProfile
    c.JSON(http.StatusOK, gin.H{"presets": presets})
}
```

### 4. Backend Service GetPresets (security_headers_service.go)

```go
func (s *SecurityHeadersService) GetPresets() []models.SecurityHeaderProfile {
    return []models.SecurityHeaderProfile{
        {
            Name:           "Basic Security",
            PresetType:     "basic",
            SecurityScore:  65,    // <-- BACKEND RETURNS "security_score"
            // ...
        },
        // ...
    }
}
```

### 5. Type Field Mismatch

| Frontend Expects | Backend Returns | Result |
|------------------|-----------------|--------|
| `preset.score` | `preset.security_score` | Score shows as `undefined` |
| `preset.type` | `preset.preset_type` | Works (mapped correctly) |
| `preset.config` | Not returned | `undefined` (not critical) |

---

## Issue #1: Score Not Showing

### Root Cause

The frontend TypeScript interface defines `score: number`, but the backend Go struct has:

```go
SecurityScore int `json:"security_score"`
```

The JSON serialization sends `security_score`, but the frontend reads `preset.score`.

### Evidence

In `SecurityHeaders.tsx` line 130:

```tsx
<div className="text-2xl font-bold">{preset.score}</div>  // undefined!
```

But backend returns:

```json
{
  "presets": [
    {
      "security_score": 65,  // NOT "score"
      "preset_type": "basic" // NOT "type"
    }
  ]
}
```

### Why It Still "Works" Partially

The frontend API layer (`securityHeaders.ts`) defines:

```typescript
async getPresets(): Promise<SecurityHeaderPreset[]> {
    const response = await client.get<{presets: SecurityHeaderPreset[]}>('/security/headers/presets');
    return response.data.presets;
}
```

TypeScript doesn't validate runtime data - it just trusts the type. The actual JSON has `security_score` but TypeScript thinks it has `score`.

---

## Issue #2: Preset Not Applying (Or Is It?)

### Flow When "Apply Preset" is Clicked

```
1. User clicks "Apply Preset" on "Basic" card
2. handleApplyPreset("basic") is called
3. applyPresetMutation.mutate({ preset_type: "basic", name: "Basic Security Profile" })
4. POST /api/v1/security/headers/presets/apply { preset_type: "basic", name: "..." }
5. Backend ApplyPreset() creates a NEW profile from the preset template
6. Returns the new profile
7. React Query invalidates 'securityHeaderProfiles' query
8. New profile appears in "Custom Profiles" section (NOT "System Presets")
```

### The Preset IS Working

The preset application actually **does work** - it creates a new custom profile. The confusion is:

- User clicks "Apply" on Basic preset
- A new **custom** profile named "Basic Security Profile" appears
- It's NOT a system preset, it's a copy with `is_preset=false`

**Users might not realize** the new profile was created because:

1. No navigation to the new profile
2. No highlighting of the newly created profile
3. The score on the Quick Start card shows `undefined`

---

## Issue #3: Redundancy Analysis - Quick Start vs System Presets

### What They Are

| Section | Data Source | Purpose | Read-Only? |
|---------|-------------|---------|------------|
| **Quick Start Presets** | `/presets` (templates) | Templates to CREATE new profiles | N/A (action buttons) |
| **System Presets** | `/profiles` with `is_preset=true` | Pre-saved profiles in DB | Yes (can only View/Clone) |

### Are They Redundant?

**Answer: Partially YES - They show the SAME presets twice, but serve different UX purposes.**

1. **Quick Start**: Action-oriented - "Click to create a new profile from this template"
2. **System Presets**: Reference-oriented - "These are the built-in profiles you can clone or view"

**The Problem**:

- If `EnsurePresetsExist()` runs on startup, it creates the presets in the database
- So System Presets section shows the same Basic/Strict/Paranoid presets
- Quick Start also shows Basic/Strict/Paranoid templates
- User sees the same 3 presets TWICE

### Why Both Exist

Looking at the code:

- `EnsurePresetsExist()` saves presets to DB so they can be assigned to hosts
- Quick Start exists to let users create custom copies with different names
- But the UX doesn't differentiate them clearly

---

## Recommended Solutions

### Fix #1: Field Name Alignment (Critical)

**Option A - Backend Change** (Recommended):
Transform the backend response to match frontend expectations:

```go
// In GetPresets handler, transform before returning:
type PresetResponse struct {
    Type        string `json:"type"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Score       int    `json:"score"`
}
```

**Option B - Frontend Change**:
Update TypeScript interface and component to use `security_score` and `preset_type`:

```typescript
export interface SecurityHeaderPreset {
  preset_type: string;        // Changed from "type"
  name: string;
  description: string;
  security_score: number;     // Changed from "score"
  // ... other fields match SecurityHeaderProfile
}
```

### Fix #2: UX Clarity for Redundancy

**Option A - Remove Quick Start Section**:

- Users can clone from System Presets instead
- Fewer UI elements, less confusion

**Option B - Differentiate Purpose**:

- Rename "Quick Start Presets" → "Create from Template"
- Add descriptive text: "Create a new custom profile based on these templates"
- Rename "System Presets" → "Built-in Profiles (Read-Only)"

**Option C - Don't Save Presets to DB**:

- Remove `EnsurePresetsExist()` call
- Keep Quick Start only
- Users always create custom profiles from templates
- System Presets section would be empty (could remove)

### Fix #3: Success Feedback on Apply

After applying a preset:

1. Show toast: "Created 'Basic Security Profile' - Scroll down to see it"
2. Or navigate to edit page for the new profile
3. Or highlight/scroll to the new profile card

---

## Code Locations for Fixes

| Fix | File | Lines |
|-----|------|-------|
| Backend type transform | `handlers/security_headers_handler.go` | 220-224 |
| Frontend type update | `api/securityHeaders.ts` | 33-40 |
| Frontend score display | `pages/SecurityHeaders.tsx` | 130 |
| Frontend preset_type | `pages/SecurityHeaders.tsx` | 138 |
| Success feedback | `hooks/useSecurityHeaders.ts` | 75-87 |
| Remove quick start | `pages/SecurityHeaders.tsx` | 118-148 |
| Preset DB creation | `routes/routes.go` | (where EnsurePresetsExist called) |

---

## Severity Assessment

| Issue | Severity | User Impact |
|-------|----------|-------------|
| Score not showing | **High** | UI shows "undefined", looks broken |
| Preset applies but unclear | **Medium** | Works but confusing |
| Redundancy confusion | **Low** | UX issue, not functional bug |

---

## Verification Steps

To confirm these findings:

1. **Check browser Network tab** when loading Security Headers page:
   - GET `/api/v1/security/headers/presets` response should show `security_score` not `score`

2. **Check console for TypeScript errors**:
   - Should NOT show errors (TypeScript doesn't validate runtime JSON)

3. **Click "Apply Preset"**:
   - Watch Network tab for POST to `/presets/apply`
   - Check if new profile appears in Custom Profiles section

4. **Compare Quick Start cards vs System Presets cards**:
   - Should show same 3 presets (Basic, Strict, Paranoid)

---

## Conclusion

The issues stem from:

1. **Type mismatch** between frontend interface and backend JSON serialization
2. **Unclear UX** about what "Apply Preset" does vs. viewing System Presets
3. **Redundant display** of the same preset data in two sections

**Recommended Priority**:

1. First: Fix the type mismatch (score → security_score, type → preset_type)
2. Second: Improve success feedback on preset application
3. Third: Consolidate or differentiate the two preset sections

---

*Investigation completed by Copilot - December 18, 2025*
