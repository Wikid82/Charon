# Security Headers "Apply Preset" Workflow Analysis

**Date**: December 18, 2025
**Issue**: User confusion after applying security header preset - no feedback, unclear activation status

---

## Executive Summary

The user applied a security header preset (e.g., "Basic Security") and experienced confusion because:

1. **No toast appeared** (actually it does, but message is ambiguous)
2. **No loading indicator** (button state doesn't show progress)
3. **Profile appeared in "Custom Profiles"** (unclear naming)
4. **Uncertainty about activation** (doesn't know if headers are live)
5. **Suggested renaming** section if headers are already active

**KEY FINDING**: Headers are **NOT ACTIVE** after applying preset. The preset creates a **new custom profile** that must be **manually assigned to each proxy host** to take effect.

**Root Cause**: UX does not communicate the multi-step workflow clearly.

---

## 🔍 Complete Workflow Trace

### Step 1: User Action

**Location**: [SecurityHeaders.tsx](../../frontend/src/pages/SecurityHeaders.tsx)
**Trigger**: User clicks "Apply" button on a preset card (Basic/Strict/Paranoid)

```tsx
<Button onClick={() => handleApplyPreset(profile.preset_type)}>
  <Play className="h-4 w-4 mr-1" /> Apply
</Button>
```

### Step 2: Frontend Handler

**Function**: `handleApplyPreset(presetType: string)`

```tsx
const handleApplyPreset = (presetType: string) => {
  const name = `${presetType.charAt(0).toUpperCase() + presetType.slice(1)} Security Profile`;
  applyPresetMutation.mutate({ preset_type: presetType, name });
};
```

**What happens**:

- Constructs name: "Basic Security Profile", "Strict Security Profile", etc.
- Calls mutation from React Query hook

### Step 3: React Query Hook

**Location**: [useSecurityHeaders.ts](../../frontend/src/hooks/useSecurityHeaders.ts#L63-L74)

```typescript
export function useApplySecurityHeaderPreset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: ApplyPresetRequest) => securityHeadersApi.applyPreset(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
      toast.success('Preset applied successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to apply preset: ${error.message}`);
    },
  });
}
```

**What happens**:

- ✅ **DOES** show toast: `'Preset applied successfully'`
- ✅ **DOES** invalidate queries (triggers refetch of profile list)
- ❌ **DOES NOT** show loading indicator during mutation

### Step 4: Backend Handler

**Location**: [security_headers_handler.go](../../backend/internal/api/handlers/security_headers_handler.go#L223-L240)

```go
func (h *SecurityHeadersHandler) ApplyPreset(c *gin.Context) {
 var req struct {
  PresetType string `json:"preset_type" binding:"required"`
  Name       string `json:"name" binding:"required"`
 }

 if err := c.ShouldBindJSON(&req); err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  return
 }

 profile, err := h.service.ApplyPreset(req.PresetType, req.Name)
 if err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  return
 }

 c.JSON(http.StatusCreated, gin.H{"profile": profile})
}
```

**What happens**:

- Receives preset type and name
- Delegates to service layer
- Returns created profile
- ❌ **DOES NOT** trigger `ApplyConfig()` (no Caddy reload)

### Step 5: Service Layer

**Location**: [security_headers_service.go](../../backend/internal/services/security_headers_service.go#L95-L120)

```go
func (s *SecurityHeadersService) ApplyPreset(presetType, name string) (*models.SecurityHeaderProfile, error) {
 presets := s.GetPresets()

 var selectedPreset *models.SecurityHeaderProfile
 for i := range presets {
  if presets[i].PresetType == presetType {
   selectedPreset = &presets[i]
   break
  }
 }

 if selectedPreset == nil {
  return nil, fmt.Errorf("preset type %s not found", presetType)
 }

 // Create a copy with custom name and UUID
 newProfile := *selectedPreset
 newProfile.ID = 0 // Clear ID so GORM creates a new record
 newProfile.UUID = uuid.New().String()
 newProfile.Name = name
 newProfile.IsPreset = false // User-created profiles are not presets
 newProfile.PresetType = ""  // Clear preset type for custom profiles

 if err := s.db.Create(&newProfile).Error; err != nil {
  return nil, fmt.Errorf("failed to create profile from preset: %w", err)
 }

 return &newProfile, nil
}
```

**What happens**:

- Finds the requested preset (basic/strict/paranoid)
- Creates a **COPY** of the preset as a new custom profile
- Saves to database
- Returns the new profile
- ❌ **Profile is NOT assigned to any hosts**
- ❌ **Headers are NOT active yet**

### Step 6: Profile Appears in UI

**Location**: "Custom Profiles" section

**What user sees**:

- New card appears in "Custom Profiles" grid
- Shows profile name, security score, timestamp
- User can Edit/Clone/Delete
- ⚠️ **No indication that profile needs to be assigned to hosts**

---

## 🔑 Critical Understanding: Per-Host Assignment

### How Security Headers Work

Security headers in Charon are **PER-HOST**, not global:

```go
// ProxyHost model
type ProxyHost struct {
 // ...
 SecurityHeaderProfileID *uint                  `json:"security_header_profile_id"`
 SecurityHeaderProfile   *SecurityHeaderProfile `json:"security_header_profile" gorm:"foreignKey:SecurityHeaderProfileID"`

 SecurityHeadersEnabled bool   `json:"security_headers_enabled" gorm:"default:true"`
 SecurityHeadersCustom  string `json:"security_headers_custom" gorm:"type:text"`
 // ...
}
```

**Key facts**:

1. Each proxy host can reference ONE profile via `SecurityHeaderProfileID`
2. If no profile is assigned, host uses inline settings or defaults
3. Creating a profile **DOES NOT** automatically assign it to any hosts
4. Headers are applied when Caddy config is generated from ProxyHost data

### When Headers Become Active

**Location**: [config.go](../../backend/internal/caddy/config.go#L1143-L1160)

```go
func buildSecurityHeadersHandler(host *models.ProxyHost) (Handler, error) {
 if host == nil {
  return nil, nil
 }

 // Use profile if configured
 var cfg *models.SecurityHeaderProfile
 if host.SecurityHeaderProfile != nil {
  cfg = host.SecurityHeaderProfile  // ✅ Profile assigned to host
 } else if !host.SecurityHeadersEnabled {
  // No profile and headers disabled - skip
  return nil, nil
 } else {
  // Use default secure headers
  cfg = getDefaultSecurityHeaderProfile()  // ⚠️ Fallback defaults
 }
 // ... builds headers from cfg ...
}
```

**Activation requires**:

1. User creates/edits a proxy host
2. User selects the security header profile in the host form
3. User saves the host
4. `ProxyHostHandler.UpdateProxyHost()` calls `caddyManager.ApplyConfig()`
5. Caddy reloads with new headers applied

---

## ❌ Current Behavior vs ✅ Expected Behavior

| Aspect | Current | Expected | Severity |
|--------|---------|----------|----------|
| **Toast notification** | ✅ Shows "Preset applied successfully" | ✅ Same (but could be clearer) | Low |
| **Loading indicator** | ❌ None during mutation | ✅ Should show loading state on button | Medium |
| **Profile location** | ✅ Appears in "Custom Profiles" | ⚠️ Should clarify activation needed | High |
| **User confusion** | ❌ "Is it active?" "What's next?" | ✅ Clear next steps | **Critical** |
| **Caddy reload** | ❌ Not triggered | ✅ Correct - only reload when assigned to host | Low |
| **Section naming** | "Custom Profiles" | ⚠️ Misleading - implies active | Medium |

---

## 🐛 Root Cause of User Confusion

### Problem 1: Ambiguous Toast Message

**Current**: `"Preset applied successfully"`
**User thinks**: "Applied to what? Is it protecting my sites now?"

### Problem 2: No Loading Indicator

Button shows no feedback during the async operation. User doesn't know:

- When request starts
- When request completes
- If anything happened at all

### Problem 3: "Custom Profiles" Name is Misleading

This section name implies:

- These are "your active profiles"
- Headers are protecting something

**Reality**: These are **AVAILABLE** profiles, not **ACTIVE** profiles

### Problem 4: No Next Steps Guidance

After applying preset, UI doesn't tell user:

- ✅ Profile created
- ⚠️ **Next**: Assign this profile to proxy hosts
- 📍 **Where**: Edit any Proxy Host → Security Headers dropdown

---

## 🎯 Recommended Fixes

### Fix 1: Improve Toast Messages ⭐ HIGH PRIORITY

**Change in**: [useSecurityHeaders.ts](../../frontend/src/hooks/useSecurityHeaders.ts)

```typescript
// CURRENT
toast.success('Preset applied successfully');

// RECOMMENDED
toast.success('Profile created! Assign it to proxy hosts to activate headers.');
```

**Better yet**, use a rich toast with action:

```typescript
onSuccess: (data) => {
  queryClient.invalidateQueries({ queryKey: ['securityHeaderProfiles'] });
  toast.success(
    <div>
      <strong>Profile created!</strong>
      <p className="text-sm">Assign it to proxy hosts to activate security headers.</p>
    </div>,
    { duration: 5000 }
  );
},
```

### Fix 2: Add Loading State to Apply Button ⭐ HIGH PRIORITY

**Change in**: [SecurityHeaders.tsx](../../frontend/src/pages/SecurityHeaders.tsx)

```tsx
<Button
  variant="outline"
  size="sm"
  onClick={() => handleApplyPreset(profile.preset_type)}
  disabled={applyPresetMutation.isPending}  // ✅ Already exists!
>
  {applyPresetMutation.isPending ? (
    <>
      <Loader2 className="h-4 w-4 mr-1 animate-spin" /> Applying...
    </>
  ) : (
    <>
      <Play className="h-4 w-4 mr-1" /> Apply
    </>
  )}
</Button>
```

**Issue**: Loading state needs to track WHICH preset is being applied (currently all buttons disable)

### Fix 3: Rename "Custom Profiles" Section ⭐ MEDIUM PRIORITY

**Options**:

| Name | Pros | Cons | Verdict |
|------|------|------|---------|
| "Available Profiles" | ✅ Accurate | ❌ Generic | ⭐⭐⭐ Good |
| "Your Profiles" | ✅ User-centric | ❌ Still ambiguous | ⭐⭐ Okay |
| "Saved Profiles" | ✅ Clear state | ❌ Wordy | ⭐⭐⭐ Good |
| "Custom Profiles (Not Assigned)" | ✅ Very clear | ❌ Too long | ⭐⭐ Okay |

**Recommended**: **"Your Saved Profiles"**

- Clear that these are stored but not necessarily active
- Differentiates from system presets
- User-friendly tone

### Fix 4: Add Empty State Guidance ⭐ MEDIUM PRIORITY

After applying first preset, show a helpful alert:

```tsx
{customProfiles.length === 1 && (
  <Alert variant="info" className="mb-4">
    <Info className="w-4 h-4" />
    <div>
      <p className="font-semibold">Next Step: Assign to Proxy Hosts</p>
      <p className="text-sm mt-1">
        Go to <Link to="/proxy-hosts">Proxy Hosts</Link>, edit a host,
        and select this profile under "Security Headers" to activate protection.
      </p>
    </div>
  </Alert>
)}
```

### Fix 5: Track Apply State Per-Preset ⭐ HIGH PRIORITY

**Problem**: `applyPresetMutation.isPending` is global - disables all buttons

**Solution**: Track which preset is being applied

```tsx
const [applyingPreset, setApplyingPreset] = useState<string | null>(null);

const handleApplyPreset = (presetType: string) => {
  setApplyingPreset(presetType);
  const name = `${presetType.charAt(0).toUpperCase() + presetType.slice(1)} Security Profile`;
  applyPresetMutation.mutate(
    { preset_type: presetType, name },
    {
      onSettled: () => setApplyingPreset(null),
    }
  );
};

// In button:
disabled={applyingPreset !== null}
// Show loading only for the specific button:
{applyingPreset === profile.preset_type ? <Loader2 .../> : <Play .../>}
```

---

## 📋 Implementation Checklist

### Phase 1: Immediate Fixes (High Priority)

- [ ] Fix toast message to clarify next steps
- [ ] Add per-preset loading state tracking
- [ ] Show loading spinner on Apply button for active preset
- [ ] Disable all Apply buttons while any is loading

### Phase 2: UX Improvements (Medium Priority)

- [ ] Rename "Custom Profiles" to "Your Saved Profiles"
- [ ] Add info alert after first profile creation
- [ ] Link alert to Proxy Hosts page with guidance

### Phase 3: Advanced (Low Priority)

- [ ] Add tooltip to Apply button explaining what happens
- [ ] Show usage count on profile cards ("Used by X hosts")
- [ ] Add "Assign to Hosts" quick action after creation

---

## 🧪 Testing Checklist

Before marking as complete:

### 1. Apply Preset Flow

- [ ] Click "Apply" on Basic preset
- [ ] Verify button shows loading spinner
- [ ] Verify other Apply buttons are disabled
- [ ] Verify toast appears with clear message
- [ ] Verify new profile appears in "Your Saved Profiles"
- [ ] Verify profile shows correct security score

### 2. Assignment Verification

- [ ] Navigate to Proxy Hosts
- [ ] Edit a host
- [ ] Verify new profile appears in Security Headers dropdown
- [ ] Select profile and save
- [ ] Verify Caddy reloads
- [ ] Verify headers appear in HTTP response (curl -I)

### 3. Edge Cases

- [ ] Apply same preset twice (should create second copy)
- [ ] Apply preset while offline (should show error toast)
- [ ] Apply preset with very long name
- [ ] Rapid-click Apply button (should debounce)

---

## 🔗 Related Files

### Backend

- [security_headers_handler.go](../../backend/internal/api/handlers/security_headers_handler.go) - API endpoint
- [security_headers_service.go](../../backend/internal/services/security_headers_service.go) - Business logic
- [proxy_host.go](../../backend/internal/models/proxy_host.go) - Host-profile relationship
- [config.go](../../backend/internal/caddy/config.go#L1143) - Header application logic

### Frontend

- [SecurityHeaders.tsx](../../frontend/src/pages/SecurityHeaders.tsx) - Main UI
- [useSecurityHeaders.ts](../../frontend/src/hooks/useSecurityHeaders.ts) - React Query hooks
- [securityHeaders.ts](../../frontend/src/api/securityHeaders.ts) - API client

---

## 📊 Summary

### What Actually Happens

1. User clicks "Apply" on preset
2. Frontend creates a **new custom profile** by copying preset settings
3. Profile is saved to database
4. Profile appears in "Custom Profiles" list
5. **Headers are NOT ACTIVE** until profile is assigned to a proxy host
6. User must edit each proxy host and select the profile
7. Only then does Caddy reload with new headers

### Why User is Confused

- ✅ Toast says "applied" but headers aren't active
- ❌ No loading indicator during save
- ❌ Section name "Custom Profiles" doesn't indicate activation needed
- ❌ No guidance on next steps
- ❌ User expects preset to "just work" globally

### Solution

Improve feedback and guidance to make the workflow explicit:

1. **Clear toast**: "Profile created! Assign to hosts to activate."
2. **Loading state**: Show spinner on Apply button
3. **Better naming**: "Your Saved Profiles" instead of "Custom Profiles"
4. **Next steps**: Show alert linking to Proxy Hosts page

---

**Status**: Analysis Complete ✅
**Next Action**: Implement Phase 1 fixes
