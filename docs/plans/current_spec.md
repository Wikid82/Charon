# 📋 Plan: Thematic Loading Overlays (Charon, Coin, & Cerberus)

## 🧐 UX & Context Analysis

**Problem**: When users make configuration changes (create/update/delete proxy hosts, security configs, certificates), Charon applies the new config to Caddy via its admin API. During this reload process (which can take 1-3 seconds, and up to 5-10 seconds with WAF/security features), the Caddy admin API temporarily stops responding on port 2019. Currently, users receive no visual feedback that a reload is happening, and they can attempt to make additional changes before the previous reload completes.

**Desired User Flow**:
1. User submits a configuration change (create/update/delete proxy host, security config, etc.)
2. **NEW**: Thematic loading overlay appears:
   - **Coin Theme** (Gold/Spinning Obol): Authentication/Login - "Paying the ferryman"
   - **Charon Theme** (Blue/Boat): Proxy hosts, certificates, general config - "Ferrying across the Styx"
   - **Cerberus Theme** (Red/Guardian): WAF, CrowdSec, ACL, Rate Limiting - "Guardian stands watch"
3. Backend applies config to Caddy (admin API may restart during this process)
4. Backend returns success/failure response
5. **NEW**: Loading overlay disappears
6. User sees success toast and updated data
7. User can safely make additional changes

**Why This Matters**:
- Prevents race conditions from rapid sequential changes
- Provides clear feedback during potentially slow operations (WAF config reloads can take 5-10s)
- Prevents user confusion when admin API is temporarily unavailable
- **Reinforces Branding**: Complete Greek mythology theme (Charon the ferryman, Cerberus the guardian, obol coin)
- **Visual Distinction**: Three clear themes - Auth (gold), Proxy (blue), Security (red)
- **Perfect Metaphor**: Login = paying Charon for passage into the Underworld (app)
- Matches enterprise-grade UX expectations with personality

## 🤝 Handoff Contract (The Truth)

### Backend Changes: NONE REQUIRED
Backend already handles config reloads correctly and returns appropriate HTTP status codes. The backend sequence is:
1. Save changes to database
2. Call `caddyManager.ApplyConfig(ctx)`
3. Return success (200/201) or error (400/500)
4. If error, rollback database changes

No backend modifications needed - this is a **frontend-only UX enhancement**.

### Frontend API Response Structure (Existing)
```json
// POST /api/v1/proxy-hosts (success)
{
  "uuid": "abc-123",
  "name": "My Service",
  "domain_names": "example.com",
  "enabled": true,
  "created_at": "2025-12-04T10:00:00Z",
  "updated_at": "2025-12-04T10:00:00Z"
}

// Error response (if Caddy reload fails)
{
  "error": "Failed to apply configuration: connection refused"
}
```

## 🎨 Phase 1: Frontend Implementation (React)

### 1.1 Create Thematic Loading Animations

**File**: `frontend/src/components/LoadingStates.tsx`

#### A. Charon-Themed Loader (Proxy/General Operations)

**New Component**: `CharonLoader` - Boat on Waves animation (Charon ferrying across the Styx)

```tsx
export function CharonLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Loading">
      {/* Animated waves */}
      <svg className="w-full h-full absolute inset-0" viewBox="0 0 100 100">
        {/* Top wave */}
        <path
          d="M0,50 Q25,45 50,50 T100,50"
          stroke="currentColor"
          className="text-blue-400/40"
          fill="none"
          strokeWidth="2"
          strokeLinecap="round"
        >
          <animate
            attributeName="d"
            values="M0,50 Q25,45 50,50 T100,50;
                    M0,50 Q25,55 50,50 T100,50;
                    M0,50 Q25,45 50,50 T100,50"
            dur="2s"
            repeatCount="indefinite"
          />
        </path>

        {/* Bottom wave (delayed) */}
        <path
          d="M0,60 Q25,55 50,60 T100,60"
          stroke="currentColor"
          className="text-blue-500/30"
          fill="none"
          strokeWidth="2"
          strokeLinecap="round"
        >
          <animate
            attributeName="d"
            values="M0,60 Q25,55 50,60 T100,60;
                    M0,60 Q25,65 50,60 T100,60;
                    M0,60 Q25,55 50,60 T100,60"
            dur="2s"
            begin="0.3s"
            repeatCount="indefinite"
          />
        </path>
      </svg>

      {/* Boat silhouette (bobbing) */}
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="animate-bob-boat">
          {/* Simple boat shape */}
          <svg width="32" height="24" viewBox="0 0 32 24" fill="none">
            <path
              d="M4,16 L8,8 L24,8 L28,16 L26,20 L6,20 Z"
              fill="currentColor"
              className="text-slate-600"
            />
            <path
              d="M8,8 L16,4 L24,8"
              stroke="currentColor"
              className="text-slate-700"
              strokeWidth="2"
              strokeLinecap="round"
            />
          </svg>
        </div>
      </div>
    </div>
  )
}
```

**Tailwind Config Addition** (or add to global CSS):

```css
@keyframes bob-boat {
  0%, 100% { transform: translateY(-3px); }
  50% { transform: translateY(3px); }
}
.animate-bob-boat {
  animation: bob-boat 2s ease-in-out infinite;
}

@keyframes pulse-glow {
  0%, 100% { opacity: 0.6; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.05); }
}
.animate-pulse-glow {
  animation: pulse-glow 2s ease-in-out infinite;
}

@keyframes rotate-head {
  0%, 100% { transform: rotate(-10deg); }
  50% { transform: rotate(10deg); }
}
.animate-rotate-head {
  animation: rotate-head 3s ease-in-out infinite;
}
```

#### B. Charon Coin Loader (Authentication/Login)

**New Component**: `CharonCoinLoader` - Spinning Obol Coin animation (Payment to the Ferryman)

```tsx
export function CharonCoinLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Authenticating">
      {/* Coin spinning on Y-axis */}
      <svg className="w-full h-full absolute inset-0" viewBox="0 0 100 100">
        {/* Coin face (animated perspective) */}
        <ellipse
          cx="50"
          cy="50"
          rx="30"
          ry="30"
          fill="currentColor"
          className="text-amber-600"
        >
          <animate
            attributeName="rx"
            values="30;5;30"
            dur="2s"
            repeatCount="indefinite"
          />
        </ellipse>

        {/* Coin edge (visible during flip) */}
        <rect
          x="45"
          y="20"
          width="10"
          height="60"
          fill="currentColor"
          className="text-amber-800"
          rx="2"
        >
          <animate
            attributeName="width"
            values="10;0;10"
            dur="2s"
            repeatCount="indefinite"
          />
          <animate
            attributeName="x"
            values="45;50;45"
            dur="2s"
            repeatCount="indefinite"
          />
        </rect>

        {/* Coin detail lines (Charon's mark) */}
        <g opacity="0.7">
          <line x1="40" y1="45" x2="60" y2="45" stroke="currentColor" className="text-amber-900" strokeWidth="2">
            <animate
              attributeName="opacity"
              values="0.7;0;0.7"
              dur="2s"
              repeatCount="indefinite"
            />
          </line>
          <line x1="40" y1="50" x2="60" y2="50" stroke="currentColor" className="text-amber-900" strokeWidth="2">
            <animate
              attributeName="opacity"
              values="0.7;0;0.7"
              dur="2s"
              repeatCount="indefinite"
            />
          </line>
          <line x1="40" y1="55" x2="60" y2="55" stroke="currentColor" className="text-amber-900" strokeWidth="2">
            <animate
              attributeName="opacity"
              values="0.7;0;0.7"
              dur="2s"
              repeatCount="indefinite"
            />
          </line>
        </g>

        {/* Subtle shine effect */}
        <ellipse
          cx="55"
          cy="40"
          rx="8"
          ry="12"
          fill="currentColor"
          className="text-yellow-400/40"
        >
          <animate
            attributeName="opacity"
            values="0.4;0.7;0.4"
            dur="2s"
            repeatCount="indefinite"
          />
        </ellipse>
      </svg>
    </div>
  )
}
```

**Why Coin for Authentication**:
- **Mythology Perfect**: In Greek mythology, the dead paid Charon with an obol (coin) to cross the River Styx
- **Metaphor**: User is "paying for passage" into the application
- **Visual Interest**: Spinning coin on Y-axis creates engaging 3D effect
- **Distinct From Other Operations**: Gold/amber vs blue (proxy) or red (security)

#### C. Cerberus-Themed Loader (Security Operations)

**New Component**: `CerberusLoader` - Three-Headed Guardian animation

```tsx
export function CerberusLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Security Loading">
      {/* Central body with pulsing shield */}
      <svg className="w-full h-full absolute inset-0" viewBox="0 0 100 100">
        {/* Shield background (pulsing) */}
        <path
          d="M50,10 L70,20 L70,45 Q70,65 50,75 Q30,65 30,45 L30,20 Z"
          fill="currentColor"
          className="text-red-900/30"
        >
          <animate
            attributeName="opacity"
            values="0.3;0.6;0.3"
            dur="2s"
            repeatCount="indefinite"
          />
        </path>

        {/* Shield outline */}
        <path
          d="M50,10 L70,20 L70,45 Q70,65 50,75 Q30,65 30,45 L30,20 Z"
          stroke="currentColor"
          className="text-red-500"
          fill="none"
          strokeWidth="2"
        />

        {/* Left head (animated rotation) */}
        <circle cx="35" cy="30" r="6" fill="currentColor" className="text-red-600">
          <animate
            attributeName="cy"
            values="30;28;30"
            dur="2s"
            repeatCount="indefinite"
          />
        </circle>

        {/* Center head (larger, animated) */}
        <circle cx="50" cy="35" r="7" fill="currentColor" className="text-red-500">
          <animate
            attributeName="r"
            values="7;8;7"
            dur="2s"
            repeatCount="indefinite"
          />
        </circle>

        {/* Right head (animated rotation) */}
        <circle cx="65" cy="30" r="6" fill="currentColor" className="text-red-600">
          <animate
            attributeName="cy"
            values="30;28;30"
            dur="2s"
            begin="1s"
            repeatCount="indefinite"
          />
        </circle>

        {/* Eyes (glowing effect) */}
        <circle cx="33" cy="29" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            repeatCount="indefinite"
          />
        </circle>
        <circle cx="37" cy="29" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            repeatCount="indefinite"
          />
        </circle>

        <circle cx="48" cy="34" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            begin="0.5s"
            repeatCount="indefinite"
          />
        </circle>
        <circle cx="52" cy="34" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            begin="0.5s"
            repeatCount="indefinite"
          />
        </circle>

        <circle cx="63" cy="29" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            begin="1s"
            repeatCount="indefinite"
          />
        </circle>
        <circle cx="67" cy="29" r="1.5" fill="currentColor" className="text-yellow-300">
          <animate
            attributeName="opacity"
            values="1;0.3;1"
            dur="3s"
            begin="1s"
            repeatCount="indefinite"
          />
        </circle>
      </svg>
    </div>
  )
}
```

**Enhancement**: Add overlay components with appropriate theming:

```tsx
export function ConfigReloadOverlay({
  message = 'Ferrying configuration...',
  submessage = 'Charon is crossing the Styx',
  type = 'charon'
}: {
  message?: string
  submessage?: string
  type?: 'charon' | 'coin' | 'cerberus'
}) {
  const Loader =
    type === 'cerberus' ? CerberusLoader :
    type === 'coin' ? CharonCoinLoader :
    CharonLoader

  const bgColor =
    type === 'cerberus' ? 'bg-red-950/90' :
    type === 'coin' ? 'bg-amber-950/90' :
    'bg-slate-800'

  const borderColor =
    type === 'cerberus' ? 'border-red-900/50' :
    type === 'coin' ? 'border-amber-900/50' :
    'border-slate-700'

  return (
    <div className="fixed inset-0 bg-slate-900/70 backdrop-blur-sm flex items-center justify-center z-50">
      <div className={`${bgColor} ${borderColor} border rounded-lg p-8 flex flex-col items-center gap-6 shadow-xl max-w-md`}>
        <Loader size="lg" />
        <div className="text-center">
          <p className="text-slate-200 font-medium text-lg">{message}</p>
          <p className="text-slate-400 text-sm mt-2">{submessage}</p>
        </div>
      </div>
    </div>
  )
}
```

**Why Cerberus Theme**:
- **Mythology Match**: Cerberus is the three-headed guard dog of the Underworld gates - perfect for security operations
- **Charon Connection**: Both from Greek mythology, thematically consistent with app branding
- **Visual Distinction**: Red/shield theme vs blue/boat clearly differentiates security vs general operations
- **Three Heads = Three Layers**: WAF, CrowdSec, Rate Limiting (the three security components)
- **Guardian Symbolism**: Emphasizes protective nature of security features

**Why Coin Theme for Login**:
- **Perfect Mythology**: In Greek myth, souls paid Charon an obol (coin) to cross into the Underworld
- **Natural Metaphor**: User "pays for passage" to access the application
- **Thematic Consistency**: Login = entering the realm, coin = the required payment
- **Visual Appeal**: 3D spinning coin effect is engaging and distinct
- **Color Distinction**: Gold/amber distinguishes auth from proxy (blue) and security (red)

**Future Enhancement** (separate issue):
Implement hybrid approach with rotating animations for all three themes:
- **Charon**: Boat (current), Rowing Oar, River Flow
- **Coin/Auth**: Coin Flip (current), Coin Drop, Token Glow, Gate Opening
- **Cerberus**: Three Heads (current), Shield Pulse, Guardian Stance, Chain Links

### 1.2 Update Hook to Expose Mutation States

**File**: `frontend/src/hooks/useProxyHosts.ts`

**Change**: Already exposes `isCreating`, `isUpdating`, `isDeleting`, `isBulkUpdating` - **NO CHANGES NEEDED**.

### 1.3 Add Loading Overlay to UI Pages

**Files to Modify**:

**Charon Theme** (Blue/Boat):
- `frontend/src/pages/ProxyHosts.tsx` - Proxy host CRUD
- `frontend/src/components/ProxyHostForm.tsx` - Form mutations
- `frontend/src/components/CertificateList.tsx` - Certificate operations

**Coin Theme** (Gold/Amber):
- `frontend/src/pages/Login.tsx` - Login authentication
- `frontend/src/context/AuthContext.tsx` - Initial auth check (optional)

**Cerberus Theme** (Red/Guardian):
- `frontend/src/pages/WafConfig.tsx` - WAF ruleset operations
- `frontend/src/pages/Security.tsx` - Security toggle operations
- `frontend/src/pages/CrowdSecConfig.tsx` - CrowdSec configuration
- `frontend/src/pages/AccessLists.tsx` - ACL operations (when implementing rate limiting page)

**Implementation Pattern** (ProxyHosts.tsx example - Charon Theme):

```tsx
import { ConfigReloadOverlay } from '../components/LoadingStates'

export default function ProxyHosts() {
  const {
    hosts,
    loading,
    isCreating,
    isUpdating,
    isDeleting,
    isBulkUpdating
  } = useProxyHosts()

  // Show overlay when ANY mutation is in progress
  const isApplyingConfig = isCreating || isUpdating || isDeleting || isBulkUpdating

  // Determine contextual message based on operation
  const getMessage = () => {
    if (isCreating) return {
      message: "Ferrying new host...",
      submessage: "Charon is crossing the Styx"
    }
    if (isDeleting) return {
      message: "Returning to shore...",
      submessage: "Host departure in progress"
    }
    if (isBulkUpdating) return {
      message: "Ferrying souls...",
      submessage: "Bulk operation crossing the river"
    }
    return {
      message: "Guiding changes across...",
      submessage: "Configuration in transit"
    }
  }

  const { message, submessage } = getMessage()

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          type="charon"
          message={message}
          submessage={submessage}
        />
      )}

      {/* Existing page content */}
      <div className="space-y-6">
        {/* ... existing code ... */}
      </div>
    </>
  )
}
```

**Implementation Pattern** (Login.tsx example - Coin Theme):

```tsx
import { ConfigReloadOverlay } from '../components/LoadingStates'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const { login } = useAuth()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      await client.post('/auth/login', { email, password })
      await login()
      toast.success('Welcome aboard')
      navigate('/')
    } catch (err) {
      toast.error('Invalid credentials')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      {loading && (
        <ConfigReloadOverlay
          type="coin"
          message="Paying the ferryman..."
          submessage="Your obol grants passage"
        />
      )}

      <div className="min-h-screen bg-dark-bg flex items-center justify-center">
        <Card>
          <form onSubmit={handleSubmit}>
            {/* form fields */}
            <Button type="submit" disabled={loading}>
              Sign In
            </Button>
          </form>
        </Card>
      </div>
    </>
  )
}
```

**Implementation Pattern** (WafConfig.tsx example - Cerberus Theme):

```tsx
import { ConfigReloadOverlay } from '../components/LoadingStates'

export default function WafConfig() {
  const { data: ruleSets, isLoading, error } = useRuleSets()
  const upsertMutation = useUpsertRuleSet()
  const deleteMutation = useDeleteRuleSet()

  // Determine if any security operation is in progress
  const isApplyingConfig = upsertMutation.isPending || deleteMutation.isPending

  // Determine contextual message based on operation
  const getMessage = () => {
    if (upsertMutation.isPending) return {
      message: "Forging new defenses...",
      submessage: "Cerberus strengthens the ward"
    }
    if (deleteMutation.isPending) return {
      message: "Lowering a barrier...",
      submessage: "Defense layer removed"
    }
    return {
      message: "Cerberus awakens...",
      submessage: "Guardian stands watch"
    }
  }

  const { message, submessage } = getMessage()

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          type="cerberus"
          message={message}
          submessage={submessage}
        />
      )}

      {/* Existing page content */}
      <div className="space-y-6">
        {/* ... existing code ... */}
      </div>
    </>
  )
}
```

**Custom Messages per Operation**:

**Charon Theme** (Proxy/General Operations):
- Create: `"Ferrying new host..."` / `"Charon is crossing the Styx"`
- Update: `"Guiding changes across..."` / `"Configuration in transit"`
- Delete: `"Returning to shore..."` / `"Host departure in progress"`
- Bulk Update: `"Ferrying {count} souls..."` / `"Bulk operation crossing the river"`

**Coin Theme** (Authentication):
- Login: `"Paying the ferryman..."` / `"Your obol grants passage"`
- Initial Load: `"The coin spins..."` / `"Seeking Charon's favor"`
- Session Check: `"Verifying payment..."` / `"Charon examines the coin"`

**Cerberus Theme** (Security Operations):
- WAF Config: `"Cerberus awakens..."` / `"Guardian of the gates stands watch"`
- WAF Enable/Disable: `"Three heads turn..."` / `"Web Application Firewall ${enabled ? 'rising' : 'resting'}"`
- Security Config: `"Strengthening the guard..."` / `"Protective wards activating"`
- CrowdSec Enable: `"Summoning the guardian..."` / `"Intrusion prevention rising"`
- Rate Limit Enable: `"Chains rattle..."` / `"Traffic gates engaging"`
- ACL Update: `"Guarding the threshold..."` / `"Access barriers shifting"`
- Ruleset Create/Update: `"Forging new defenses..."` / `"Security rules inscribing"`
- Ruleset Delete: `"Lowering a barrier..."` / `"Defense layer removed"`

### 1.4 Disable Form Inputs During Mutations

**File**: `frontend/src/components/ProxyHostForm.tsx`

**Enhancement**: Disable all form inputs when parent is applying config:

```tsx
interface ProxyHostFormProps {
  // ... existing props
  isApplyingConfig?: boolean  // NEW
}

export default function ProxyHostForm({
  host,
  onSave,
  onCancel,
  isApplyingConfig = false  // NEW
}: ProxyHostFormProps) {

  // Disable entire form during config reload
  const isFormDisabled = isApplyingConfig

  return (
    <form onSubmit={handleSubmit}>
      <input
        disabled={isFormDisabled}
        // ... other props
      />
      <button
        disabled={isFormDisabled}
        type="submit"
      >
        {isApplyingConfig ? 'Applying...' : 'Save'}
      </button>
    </form>
  )
}
```

### 1.5 Handle Bulk Operations

**File**: `frontend/src/pages/ProxyHosts.tsx`

**Bulk ACL Update**: Already uses `isBulkUpdating` state - just add overlay:

```tsx
const handleBulkUpdateACL = async () => {
  try {
    // Loading overlay automatically shows via isBulkUpdating
    // Message: "Ferrying {count} souls..." displays automatically
    const result = await bulkUpdateACL(selectedUUIDs, selectedACLID)

    toast.success(`Ferried ${result.updated} souls safely across`)

    if (result.errors.length > 0) {
      toast.error(`${result.errors.length} souls could not cross`)
    }
  } catch (err) {
    toast.error('Ferry crossing failed')
  }
}
```

**Bulk Delete**: Same pattern with `isDeleting` state.

## 🕵️ Phase 2: QA & Edge Cases

### Edge Case Testing

| Scenario | Expected Behavior |
|----------|------------------|
| **Rapid Sequential Changes** | Second change waits for first to complete (overlay remains visible) |
| **Config Apply Fails** | Overlay disappears, error toast shows, form re-enabled |
| **Long WAF Reload (10s)** | Overlay remains visible throughout, no timeout |
| **Concurrent User Changes** | Each user sees their own overlay, React Query handles cache |
| **Browser Tab Switch** | Overlay persists across tab switches (React state maintained) |
| **Form Validation Error** | Overlay never appears (validation happens before mutation) |
| **Network Timeout** | React Query timeout (30s default) triggers error, overlay clears |
| **Theme Switching** | Coin (gold) for auth, Charon (blue) for proxy, Cerberus (red) for security |
| **Login Flow** | Coin overlay shows "Paying the ferryman..." during authentication |
| **Security Toggle** | Cerberus overlay shows when enabling/disabling WAF, CrowdSec, Rate Limit, ACL |
| **Ruleset Operations** | Cerberus overlay for create/update/delete WAF rulesets |

### Testing Checklist

**Manual Testing**:

**Coin Theme (Authentication)**:
1. ✅ Login with valid credentials → Coin (gold) overlay appears → "Paying the ferryman..." → success → dashboard
2. ✅ Login with invalid credentials → Coin overlay → error toast → overlay clears
3. ✅ App initial load (auth check) → Optional: subtle coin animation during /auth/me call

**Charon Theme (Proxy Operations)**:
4. ✅ Create new proxy host → Charon (blue) overlay appears → success → overlay disappears
5. ✅ Update existing host → Charon overlay during update → success
6. ✅ Delete host → Charon overlay with "Returning to shore..." → success
7. ✅ Bulk update ACL on 5 hosts → Charon overlay with "Ferrying souls..." → success
8. ✅ Certificate upload → Charon overlay → success

**Cerberus Theme (Security Operations)**:
9. ✅ Enable WAF → Cerberus (red) overlay with "Three heads turn..." → success
10. ✅ Create WAF ruleset → Cerberus overlay "Forging new defenses..." → success (5-10s)
11. ✅ Delete WAF ruleset → Cerberus overlay "Lowering a barrier..." → success
12. ✅ Enable CrowdSec → Cerberus overlay "Summoning the guardian..." → success
13. ✅ Update security config → Cerberus overlay "Strengthening the guard..." → success
14. ✅ Enable Rate Limiting → Cerberus overlay "Chains rattle..." → success

**General**:
15. ✅ Submit invalid data → validation error, NO overlay shown
16. ✅ Trigger Caddy error (stop Caddy) → overlay → error toast → overlay clears
17. ✅ Rapid clicks on save button → first click triggers overlay, subsequent ignored
18. ✅ Navigate away during reload → confirm user intent, abort mutation
19. ✅ Test in Firefox, Chrome, Safari → consistent behavior
20. ✅ Verify theme colors: Coin (gold/amber), Charon (blue boat), Cerberus (red guardian)

**Automated Testing**:
```tsx
// frontend/src/pages/__tests__/ProxyHosts-reload-overlay.test.tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProxyHosts from '../ProxyHosts'

it('shows Charon-themed overlay during proxy host create', async () => {
  // Mock API to delay response
  vi.mocked(proxyHostsApi.createProxyHost).mockImplementation(
    () => new Promise(resolve => setTimeout(() => resolve(mockHost), 2000))
  )

  render(<ProxyHosts />)

  // Click create
  await userEvent.click(screen.getByText('Add Proxy Host'))
  await userEvent.click(screen.getByText('Save'))

  // Charon-themed overlay should appear
  expect(screen.getByText('Ferrying new host...')).toBeInTheDocument()
  expect(screen.getByText('Charon is crossing the Styx')).toBeInTheDocument()

  // Overlay should disappear after completion
  await waitFor(() => {
    expect(screen.queryByText('Ferrying new host...')).not.toBeInTheDocument()
  }, { timeout: 3000 })
})

it('disables form inputs during config reload', async () => {
  render(<ProxyHosts />)

  const saveButton = screen.getByText('Save')
  await userEvent.click(saveButton)

  // Button should be disabled during mutation
  expect(saveButton).toBeDisabled()
  expect(saveButton).toHaveTextContent('Applying...')
})
```

```tsx
// frontend/src/pages/__tests__/Login-coin-overlay.test.tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Login from '../Login'

it('shows coin-themed overlay during login', async () => {
  // Mock API to delay response
  vi.mocked(client.post).mockImplementation(
    () => new Promise(resolve => setTimeout(() => resolve({ data: {} }), 2000))
  )

  render(<Login />)

  // Fill form and submit
  await userEvent.type(screen.getByLabelText('Email'), 'admin@example.com')
  await userEvent.type(screen.getByLabelText('Password'), 'password123')
  await userEvent.click(screen.getByText('Sign In'))

  // Coin-themed overlay should appear
  expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()
  expect(screen.getByText('Your obol grants passage')).toBeInTheDocument()

  // Verify gold/amber theme styling
  const overlay = screen.getByText('Paying the ferryman...').closest('div')
  expect(overlay).toHaveClass('bg-amber-950/90')

  // Overlay should disappear after successful login
  await waitFor(() => {
    expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
  }, { timeout: 3000 })
})

it('clears overlay on login error', async () => {
  vi.mocked(client.post).mockRejectedValue({
    response: { data: { error: 'Invalid credentials' } }
  })

  render(<Login />)

  await userEvent.type(screen.getByLabelText('Email'), 'wrong@example.com')
  await userEvent.type(screen.getByLabelText('Password'), 'wrong')
  await userEvent.click(screen.getByText('Sign In'))

  // Overlay appears
  expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()

  // Overlay clears after error
  await waitFor(() => {
    expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
  })

  // Error toast shown (tested elsewhere)
})
```

```tsx
// frontend/src/pages/__tests__/WafConfig-reload-overlay.test.tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import WafConfig from '../WafConfig'

it('shows Cerberus-themed overlay during ruleset create', async () => {
  // Mock API to delay response (WAF operations can be slow)
  vi.mocked(securityApi.upsertRuleSet).mockImplementation(
    () => new Promise(resolve => setTimeout(() => resolve(mockRuleSet), 5000))
  )

  render(<WafConfig />)

  // Open create form and submit
  await userEvent.click(screen.getByText('Add Rule Set'))
  await userEvent.type(screen.getByLabelText('Rule Set Name'), 'Test Rules')
  await userEvent.type(screen.getByLabelText('Rule Content'), 'SecRule REQUEST_URI "@contains test"')
  await userEvent.click(screen.getByText('Create Rule Set'))

  // Cerberus-themed overlay should appear
  expect(screen.getByText('Forging new defenses...')).toBeInTheDocument()
  expect(screen.getByText('Cerberus strengthens the ward')).toBeInTheDocument()

  // Verify red theme styling
  const overlay = screen.getByText('Forging new defenses...').closest('div')
  expect(overlay).toHaveClass('bg-red-950/90')

  // Overlay should disappear after completion
  await waitFor(() => {
    expect(screen.queryByText('Forging new defenses...')).not.toBeInTheDocument()
  }, { timeout: 6000 })
})

it('shows Cerberus overlay for delete operation', async () => {
  vi.mocked(securityApi.deleteRuleSet).mockImplementation(
    () => new Promise(resolve => setTimeout(() => resolve(), 2000))
  )

  render(<WafConfig />)

  await userEvent.click(screen.getByTestId('delete-ruleset-1'))
  await userEvent.click(screen.getByTestId('confirm-delete-btn'))

  // Cerberus delete message
  expect(screen.getByText('Lowering a barrier...')).toBeInTheDocument()
  expect(screen.getByText('Defense layer removed')).toBeInTheDocument()
})
```

## 📚 Phase 3: Documentation

### User Documentation

**File**: `docs/features.md`

Add new section:

```markdown
## Configuration Feedback

When you make changes to proxy hosts, security settings, or certificates, Charon applies the configuration to Caddy's reverse proxy. During this process:

- 🔄 **Loading Overlay**: A blocking overlay appears with "Applying configuration..."
- ⏱️ **Duration**: Typically 1-3 seconds, up to 10 seconds for complex WAF configurations
- 🚫 **Input Disabled**: Form inputs are disabled during reload to prevent conflicts
- ✅ **Success Feedback**: Toast notification confirms successful application
- ❌ **Error Handling**: If reload fails, the overlay clears and an error message appears

**Note**: Caddy's admin API temporarily restarts during config reloads. This is normal behavior and the UI will wait for completion before allowing new changes.
```

### Developer Documentation

**File**: `frontend/src/components/LoadingStates.tsx` (JSDoc comments)

```tsx
/**
 * ConfigReloadOverlay - Full-screen blocking overlay for Caddy configuration reloads
 *
 * Display when:
 * - Creating/updating/deleting proxy hosts
 * - Applying WAF or security configurations
 * - Bulk operations that trigger Caddy reloads
 *
 * Technical Notes:
 * - Caddy admin API (port 2019) stops during config reloads (1-10s)
 * - Overlay uses z-50 to block all interactions
 * - Automatically clears when mutation completes/fails
 *
 * @param message - Primary message (e.g., "Applying configuration...")
 * @param submessage - Secondary context (e.g., "Please wait while Caddy reloads")
 */
```

## 🛠️ Implementation Checklist

### Step 1: Create Components (45 min)
- [ ] Add `CharonLoader` (boat) to `LoadingStates.tsx`
- [ ] Add `CharonCoinLoader` (spinning obol) to `LoadingStates.tsx`
- [ ] Add `CerberusLoader` (three heads) to `LoadingStates.tsx`
- [ ] Add `ConfigReloadOverlay` with theme support
- [ ] Add Tailwind keyframes for all animations
- [ ] Add unit tests for new components
- [ ] Verify styling in dev environment

### Step 2: Update Login Page (20 min)
- [ ] Import `ConfigReloadOverlay` with coin theme
- [ ] Replace button `isLoading` state with full overlay
- [ ] Add "Paying the ferryman..." message
- [ ] Test login flow with overlay
- [ ] Verify coin animation performance

### Step 3: Update ProxyHosts Page (45 min)
- [ ] Import `ConfigReloadOverlay` with Charon theme
- [ ] Add `isApplyingConfig` computed state
- [ ] Render overlay conditionally
- [ ] Test create/update/delete operations
- [ ] Test bulk operations

### Step 4: Update Security Pages (30 min each)
- [ ] Update `CrowdSecConfig.tsx` (Cerberus theme)
- [ ] Update `WAFConfig.tsx` (Cerberus theme)
- [ ] Update `Security.tsx` for toggle operations (Cerberus theme)
- [ ] Test with actual WAF ruleset uploads (slow path)
- [ ] Test security toggle operations (enable/disable services)

### Step 5: Update Certificate Management (20 min)
- [ ] Update `CertificateList.tsx` (Charon theme)
- [ ] Test certificate upload/delete

### Step 6: Update Form Component (30 min)
- [ ] Add `isApplyingConfig` prop to `ProxyHostForm`
- [ ] Disable all inputs when true
- [ ] Update button text during mutation
- [ ] Test in modal and standalone contexts

### Step 7: Write Tests (75 min)
- [ ] Component tests for all three loaders (Charon, Coin, Cerberus)
- [ ] Component tests for `ConfigReloadOverlay` theme switching
- [ ] Integration tests for Login page (coin theme)
- [ ] Integration tests for ProxyHosts page (Charon theme)
- [ ] Integration tests for WafConfig page (Cerberus theme)
- [ ] Test rapid sequential operations
- [ ] Test error cases

### Step 8: Manual QA (40 min)
- [ ] Test login flow with coin animation
- [ ] Test all CRUD operations on proxy hosts (Charon)
- [ ] Test security operations (Cerberus)
- [ ] Test bulk operations
- [ ] Test with slow Caddy reloads (add artificial delay)
- [ ] Test cross-browser (Chrome, Firefox)
- [ ] Verify theme colors: Coin (gold), Charon (blue), Cerberus (red)

### Step 9: Documentation (15 min)
- [ ] Update `docs/features.md`
- [ ] Add JSDoc comments
- [ ] Update CHANGELOG

**Total Estimated Time**: 6-7 hours (includes Charon, Coin, and Cerberus themes)

## ✅ Acceptance Criteria

- [ ] Loading overlay appears immediately when config mutation starts
- [ ] Overlay blocks all UI interactions during reload
- [ ] Overlay shows contextual messages per operation type
- [ ] Form inputs are disabled during mutations
- [ ] Overlay automatically clears on success or error
- [ ] No race conditions from rapid sequential changes
- [ ] Works consistently in Firefox, Chrome, Safari
- [ ] Existing functionality unchanged (no regressions)
- [ ] All tests pass (existing + new)
- [ ] Pre-commit checks pass
- [ ] Correct theme used: Coin (gold) for auth, Charon (blue) for proxy, Cerberus (red) for security
- [ ] Login page uses coin theme with "Paying the ferryman..." message
- [ ] All security operations (WAF, CrowdSec, ACL, Rate Limit) use Cerberus theme
- [ ] Animation performance acceptable (no janky SVG rendering, smooth 60fps)

## 🔍 Technical Notes

### Why Frontend-Only?

The backend already handles config reloads correctly:
1. Backend receives request
2. Saves to database
3. Calls `caddyManager.ApplyConfig()`
4. Returns success/error
5. Rolls back DB changes on error

The issue is purely UX - users don't see that a reload is happening and the admin API is temporarily unavailable.

### React Query Benefits

We use React Query for state management, which provides:
- Automatic loading states (`isPending`)
- Error handling
- Cache invalidation
- Retry logic
- Request deduplication

No additional state management needed - we just surface the existing mutation states to the UI.

### Z-Index Layering

```
z-10: Navigation
z-20: Modals
z-30: Tooltips
z-40: Toast notifications
z-50: Config reload overlay (NEW - must block everything)
```

### Performance Impact

**Negligible**:
- Overlay is conditionally rendered (not always in DOM)
- No polling or long-running timers
- React Query handles all async logic
- Single boolean state check per render

## 🚫 Out of Scope

The following are explicitly NOT included in this plan:

1. **Progress Bar**: We don't know total reload time in advance
2. **Cancel Operation**: Once submitted to backend, rollback is complex
3. **Optimistic Updates**: Config changes must succeed before showing
4. **Background Reloads**: Config changes MUST complete before new ones start
5. **Admin API Monitoring**: We rely on backend response, not admin API polling
6. **Retry Logic**: React Query provides this, no custom implementation
7. **Queue System**: Not needed - mutations are already sequential per user

## 📊 Success Metrics

**Before** (Current):
- Users confused why subsequent changes fail
- Support tickets: "Config changes not working"
- Rapid-fire changes cause race conditions

**After** (Target):
- Clear visual feedback during reloads
- Zero race conditions from rapid changes
- Reduced support tickets
- Professional UX matching enterprise tools

## 🔗 Related Issues

- WAF Integration Test Reliability (Issue with Caddy admin API stopping during reload)
- User reported: "Changes don't seem to save" (actually timing issue)
- Enhancement request: Loading indicators for long operations
- **Future Enhancement**: Hybrid rotating loading animations - see GitHub Issue (to be created)
  - **Charon Variants**: Boat on Waves, Coin Flip, Rowing Oar, River Flow
  - **Cerberus Variants**: Three Heads Alert, Shield Pulse, Guardian Stance, Chain Links
  - Randomized selection on each load for visual variety
  - Matching thematic messages for each animation variant

## 📅 Timeline

**Day 1** (6 hours):
- Morning: Create all three loader components: Charon, Coin, Cerberus (2.5 hours)
- Afternoon: Update Login page with Coin theme (30 min)
- Afternoon: Update ProxyHosts page with Charon theme (1.5 hours)
- Afternoon: Update WAF/Security pages with Cerberus theme (1.5 hours)

**Day 2** (3 hours):
- Morning: Certificate management, CrowdSec config (1 hour)
- Morning: Write unit tests for all three themes (1 hour)
- Afternoon: Manual QA, documentation, code review (1 hour)

**Total**: 2 days for full tri-theme implementation and testing
