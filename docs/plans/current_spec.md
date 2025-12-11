# CrowdSec Preset Matching Fix

## Problem
The user reports "preset not found in hub" for all three curated presets:
1. `honeypot-friendly-defaults`
2. `crowdsecurity/base-http-scenarios`
3. `geolocation-aware`

## Root Cause Analysis

### 1. `crowdsecurity/base-http-scenarios`
This preset **exists** in the CrowdSec Hub (verified via `curl`), but the application fails to find it.
- **Cause**: The `fetchIndexHTTPFromURL` function in `backend/internal/crowdsec/hub_sync.go` attempts to unmarshal the index JSON into a `HubIndex` struct.
- The `HubIndex` struct expects a JSON object with an `"items"` field (compiled format).
- The raw hub index (from `raw.githubusercontent.com`) uses a "Map of Maps" structure (source format) with keys like `"collections"`, `"parsers"`, etc., and **no** `"items"` field.
- `json.Unmarshal` succeeds but leaves `idx.Items` empty (nil).
- The code assumes success and returns the empty index, bypassing the fallback to `parseRawIndex`.
- `findIndexEntry` then searches an empty list and returns false.

### 2. `honeypot-friendly-defaults` & `geolocation-aware`
These presets are defined with `Source: "charon-curated"` and `RequiresHub: false`.
- **Cause**: They do not exist in the CrowdSec Hub. The "preset not found" error is correct behavior if `Hub.Pull` is called for them.
- **Implication**: The frontend or handler should not be attempting to `Pull` these presets from the Hub, or the backend should handle them differently (e.g., by generating local configuration).

## Implementation Plan

### 1. Fix Index Parsing in `backend/internal/crowdsec/hub_sync.go`
Modify `fetchIndexHTTPFromURL` to correctly detect the raw index format.
- **Current Logic**:
  ```go
  if err := json.Unmarshal(data, &idx); err != nil {
      // Try parsing as raw index
      if rawIdx, rawErr := parseRawIndex(data, target); rawErr == nil { ... }
  }
  ```
- **New Logic**:
  ```go
  if err := json.Unmarshal(data, &idx); err != nil || len(idx.Items) == 0 {
      // If unmarshal failed OR resulted in empty items (likely raw index format),
      // try parsing as raw index.
      if rawIdx, rawErr := parseRawIndex(data, target); rawErr == nil {
          return rawIdx, nil
      }
      // If both failed, return original error (or new error if unmarshal succeeded but empty)
  }
  ```

### 2. Verify `parseRawIndex`
Ensure `parseRawIndex` correctly handles the `collections` section and extracts the `crowdsecurity/base-http-scenarios` entry.
- The existing implementation iterates over the map and should correctly extract entries.
- `sanitizeSlug` is verified to handle the slug correctly.

### 3. (Future/Separate Task) Handle Charon-Curated Presets
- The handler `PullPreset` currently calls `Hub.Pull` blindly.
- It should check `RequiresHub` from the preset definition.
- If `RequiresHub` is false, it should skip the Hub pull and potentially perform a local "install" (or return success if no action is needed).
- *Note: This plan focuses on fixing the matching issue for the hub-based preset.*

## Verification Steps
1. Run `curl` to fetch the raw index (already done).
2. Apply the fix to `hub_sync.go`.
3. Run `go test ./backend/internal/crowdsec/...` to verify the fix.
4. Attempt to pull `crowdsecurity/base-http-scenarios` again.

# CrowdSec Presets UI Improvements

## Problem
The current CrowdSec Presets UI uses a simple native `<select>` dropdown. As the number of presets grows (especially with the Hub integration), this becomes unwieldy. Users cannot search for presets, sort them, or easily distinguish between curated and Hub presets.

## Goals
1.  **Search**: Allow users to filter presets by title, description, or slug.
2.  **Sort**: Allow users to sort presets by Alphabetical order, Type, or Source.
3.  **UI**: Replace the `<select>` with a more robust, scrollable list view with search and sort controls.

## Implementation Plan

### 1. State Management
Modify `frontend/src/pages/CrowdSecConfig.tsx` to add state for search and sort.

```typescript
const [searchQuery, setSearchQuery] = useState('')
const [sortBy, setSortBy] = useState<'alpha' | 'type' | 'source'>('alpha')
```

### 2. Filtering and Sorting Logic
Update the `presetCatalog` logic or create a derived `filteredPresets` list.

*   **Filter**: Check if `searchQuery` is included in `title`, `description`, or `slug` (case-insensitive).
*   **Sort**:
    *   `alpha`: Sort by `title` (A-Z).
    *   `type`: Sort by `type` (if available, otherwise fallback to title). *Note: The current `CrowdsecPreset` type might need to expose `type` (collection, scenario, etc.) if it's not already clear. If not available, we might infer it or skip this sort option for now.*
    *   `source`: Sort by `source` (e.g., `charon-curated` vs `hub`).

### 3. UI Components
Replace the `<select>` element with a custom UI block.

*   **Search Input**: A standard text input at the top.
*   **Sort Controls**: A small dropdown or set of buttons to toggle sort order.
*   **List View**: A scrollable `div` (max-height constrained) rendering the list of filtered presets.
    *   Each item should show the `title` and maybe a small badge for `source` or `status` (installed/cached).
    *   Clicking an item selects it (updates `selectedPresetSlug`).
    *   The selected item should be visually highlighted.

### 4. Detailed Design
```tsx
<div className="space-y-2">
  <div className="flex gap-2">
    <Input
      placeholder="Search presets..."
      value={searchQuery}
      onChange={(e) => setSearchQuery(e.target.value)}
      className="flex-1"
    />
    <select
      value={sortBy}
      onChange={(e) => setSortBy(e.target.value as any)}
      className="..."
    >
      <option value="alpha">Name (A-Z)</option>
      <option value="source">Source</option>
    </select>
  </div>

  <div className="border border-gray-700 rounded-lg max-h-60 overflow-y-auto bg-gray-900">
    {filteredPresets.map(preset => (
      <div
        key={preset.slug}
        onClick={() => setSelectedPresetSlug(preset.slug)}
        className={`p-2 cursor-pointer hover:bg-gray-800 ${selectedPresetSlug === preset.slug ? 'bg-blue-900/30 border-l-2 border-blue-500' : ''}`}
      >
        <div className="font-medium">{preset.title}</div>
        <div className="text-xs text-gray-400 flex justify-between">
           <span>{preset.slug}</span>
           <span>{preset.source}</span>
        </div>
      </div>
    ))}
  </div>
</div>
```

## Verification Steps
1.  Verify search filters the list correctly.
2.  Verify sorting changes the order of items.
3.  Verify clicking an item selects it and updates the preview/details view below.
4.  Verify the UI handles empty search results gracefully.

# Documentation Updates

## Tasks
- [x] Update `docs/features.md` with new CrowdSec integration details (Hub Presets, Console Enrollment).
- [x] Update `docs/security.md` with instructions for using the new UI and Console Enrollment.
- [x] Create `docs/reports/crowdsec_integration_summary.md` summarizing all changes.
