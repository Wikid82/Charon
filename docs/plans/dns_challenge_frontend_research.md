# DNS Challenge Frontend Research - Issue #21

## Overview

This document outlines the frontend architecture analysis and implementation plan for DNS challenge support in the Charon proxy manager. DNS challenges are required for wildcard certificate issuance via Let's Encrypt/ACME.

---

## 1. Existing Frontend Architecture Analysis

### Technology Stack

| Layer | Technology |
|-------|------------|
| Framework | React 18+ with TypeScript |
| State Management | TanStack Query (React Query) |
| Routing | React Router |
| UI Components | Custom component library (Radix UI primitives) |
| Styling | Tailwind CSS with custom design tokens |
| Internationalization | react-i18next |
| HTTP Client | Axios |
| Icons | Lucide React |

### Directory Structure

```
frontend/src/
├── api/           # API client functions (typed, async)
├── components/    # Reusable UI components
│   ├── ui/        # Design system primitives
│   ├── layout/    # Layout components (PageShell, etc.)
│   └── dialogs/   # Modal dialogs
├── hooks/         # Custom React hooks (data fetching)
├── pages/         # Page-level components
└── utils/         # Utility functions
```

### Key Architectural Patterns

1. **API Layer** (`frontend/src/api/`)
   - Each domain has its own API file (e.g., `certificates.ts`, `smtp.ts`)
   - Uses typed interfaces for request/response
   - All functions are async, returning Promise types
   - Axios client with base URL `/api/v1`

2. **Custom Hooks** (`frontend/src/hooks/`)
   - Wrap TanStack Query for data fetching
   - Naming convention: `use{Resource}` (e.g., `useCertificates`, `useDomains`)
   - Return `{ data, isLoading, error, refetch }` pattern

3. **Page Components** (`frontend/src/pages/`)
   - Use `PageShell` wrapper for consistent layout
   - Header with title, description, and action buttons
   - Card-based content organization

4. **Form Patterns**
   - Use controlled components with `useState`
   - `useMutation` for form submissions
   - Toast notifications for success/error feedback
   - Inline validation with error state

---

## 2. UI Component Patterns Identified

### Design System Components (`frontend/src/components/ui/`)

| Component | Purpose | Usage |
|-----------|---------|-------|
| `Button` | Primary actions, variants: primary, secondary, danger, ghost | All forms |
| `Input` | Text/password inputs with label, error, helper text support | Forms |
| `Select` | Radix-based dropdown with proper styling | Provider selection |
| `Card` | Content containers with header/content/footer | Page sections |
| `Dialog` | Modal dialogs for forms/confirmations | Add/edit modals |
| `Alert` | Info/warning/error banners | Notifications |
| `Badge` | Status indicators | Provider status |
| `Label` | Form field labels | All form fields |
| `Textarea` | Multi-line text input | Advanced config |
| `Switch` | Toggle switches | Enable/disable |

### Form Patterns (from `SMTPSettings.tsx`, `ProxyHostForm.tsx`)

```tsx
// Standard form structure
const [formData, setFormData] = useState({ /* initial state */ })

const mutation = useMutation({
  mutationFn: async () => { /* API call */ },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['resource'] })
    toast.success(t('resource.success'))
  },
  onError: (error) => {
    toast.error(error.message)
  },
})

// Form submission
const handleSubmit = (e: React.FormEvent) => {
  e.preventDefault()
  mutation.mutate()
}
```

### Password/Credential Input Pattern

From `Input.tsx`:

- Built-in password visibility toggle
- Uses `type="password"` with eye icon
- Supports `helperText` for guidance
- `autoComplete` attributes for security

### Test Connection Pattern (from `RemoteServerForm.tsx`, `SMTPSettings.tsx`)

```tsx
const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')

const handleTestConnection = async () => {
  setTestStatus('testing')
  try {
    await testConnection(/* params */)
    setTestStatus('success')
    setTimeout(() => setTestStatus('idle'), 3000)
  } catch {
    setTestStatus('error')
    setTimeout(() => setTestStatus('idle'), 3000)
  }
}
```

---

## 3. Proposed New Components

### 3.1 DNS Providers Page (`DNSProviders.tsx`)

**Location**: `frontend/src/pages/DNSProviders.tsx`

**Purpose**: Manage DNS provider configurations for DNS-01 challenges

**Layout**:

```
┌─────────────────────────────────────────────────────────┐
│ DNS Providers                        [+ Add Provider]   │
│ Configure DNS providers for wildcard certificates       │
├─────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Alert: DNS providers enable wildcard certificates   │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌─────────────────────┐ ┌─────────────────────┐        │
│ │ Cloudflare          │ │ Route53             │        │
│ │ ✓ Configured        │ │ Not configured      │        │
│ │ [Edit] [Test] [Del] │ │ [Configure]         │        │
│ └─────────────────────┘ └─────────────────────┘        │
└─────────────────────────────────────────────────────────┘
```

**Features**:

- List configured DNS providers
- Add/Edit/Delete provider configurations
- Test DNS propagation
- Status badges (configured, error, pending)

### 3.2 DNS Provider Form (`DNSProviderForm.tsx`)

**Location**: `frontend/src/components/DNSProviderForm.tsx`

**Purpose**: Add/edit DNS provider configuration

**Key Features**:

- Dynamic form fields based on provider type
- Credential inputs with password masking
- Test connection button
- Validation feedback

**Provider-Specific Fields**:

| Provider | Required Fields |
|----------|-----------------|
| Cloudflare | API Token OR (API Key + Email) |
| Route53 | Access Key ID, Secret Access Key, Region |
| DigitalOcean | API Token |
| Google Cloud DNS | Service Account JSON |
| Namecheap | API User, API Key |
| GoDaddy | API Key, API Secret |
| Azure DNS | Client ID, Client Secret, Subscription ID, Resource Group |

### 3.3 Provider Selector Dropdown (`DNSProviderSelector.tsx`)

**Location**: `frontend/src/components/DNSProviderSelector.tsx`

**Purpose**: Dropdown to select DNS provider when requesting wildcard certificates

**Usage**: Integrated into `ProxyHostForm.tsx` or certificate request flow

```tsx
<DNSProviderSelector
  value={selectedProviderId}
  onChange={setSelectedProviderId}
  required={isWildcard}
/>
```

### 3.4 DNS Propagation Test Component (`DNSPropagationTest.tsx`)

**Location**: `frontend/src/components/DNSPropagationTest.tsx`

**Purpose**: Visual feedback for DNS TXT record propagation

**Features**:

- Shows test domain and expected TXT record
- Real-time propagation status
- Retry button
- Success/failure indicators

---

## 4. API Hooks Needed

### 4.1 `useDNSProviders` Hook

**Location**: `frontend/src/hooks/useDNSProviders.ts`

```typescript
export function useDNSProviders() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['dns-providers'],
    queryFn: getDNSProviders,
  })

  return {
    providers: data || [],
    isLoading,
    error,
    refetch,
  }
}
```

### 4.2 `useDNSProviderMutations` Hook

**Location**: `frontend/src/hooks/useDNSProviders.ts`

```typescript
export function useDNSProviderMutations() {
  const queryClient = useQueryClient()

  const createMutation = useMutation({
    mutationFn: createDNSProvider,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['dns-providers'] }),
  })

  const updateMutation = useMutation({
    mutationFn: updateDNSProvider,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['dns-providers'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDNSProvider,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['dns-providers'] }),
  })

  const testMutation = useMutation({
    mutationFn: testDNSProvider,
  })

  return { createMutation, updateMutation, deleteMutation, testMutation }
}
```

---

## 5. API Client Functions Needed

### 5.1 `frontend/src/api/dnsProviders.ts`

```typescript
import client from './client'

/** Supported DNS provider types */
export type DNSProviderType =
  | 'cloudflare'
  | 'route53'
  | 'digitalocean'
  | 'gcloud'
  | 'namecheap'
  | 'godaddy'
  | 'azure'

/** DNS Provider configuration */
export interface DNSProvider {
  id: number
  name: string
  provider_type: DNSProviderType
  is_default: boolean
  created_at: string
  updated_at: string
  last_used_at?: string
  status: 'active' | 'error' | 'unconfigured'
}

/** Provider-specific credentials (varies by type) */
export interface DNSProviderCredentials {
  // Cloudflare
  api_token?: string
  api_key?: string
  email?: string
  // Route53
  access_key_id?: string
  secret_access_key?: string
  region?: string
  // DigitalOcean
  token?: string
  // GCloud
  service_account_json?: string
  project_id?: string
  // Generic
  [key: string]: string | undefined
}

/** Request payload for creating/updating provider */
export interface DNSProviderRequest {
  name: string
  provider_type: DNSProviderType
  credentials: DNSProviderCredentials
  is_default?: boolean
}

/** Test result response */
export interface DNSTestResult {
  success: boolean
  message?: string
  error?: string
  propagation_time_ms?: number
}

// API functions
export const getDNSProviders = async (): Promise<DNSProvider[]> => {
  const response = await client.get<DNSProvider[]>('/dns-providers')
  return response.data
}

export const getDNSProvider = async (id: number): Promise<DNSProvider> => {
  const response = await client.get<DNSProvider>(`/dns-providers/${id}`)
  return response.data
}

export const createDNSProvider = async (data: DNSProviderRequest): Promise<DNSProvider> => {
  const response = await client.post<DNSProvider>('/dns-providers', data)
  return response.data
}

export const updateDNSProvider = async (
  id: number,
  data: Partial<DNSProviderRequest>
): Promise<DNSProvider> => {
  const response = await client.put<DNSProvider>(`/dns-providers/${id}`, data)
  return response.data
}

export const deleteDNSProvider = async (id: number): Promise<void> => {
  await client.delete(`/dns-providers/${id}`)
}

export const testDNSProvider = async (id: number): Promise<DNSTestResult> => {
  const response = await client.post<DNSTestResult>(`/dns-providers/${id}/test`)
  return response.data
}

export const testDNSProviderCredentials = async (
  data: DNSProviderRequest
): Promise<DNSTestResult> => {
  const response = await client.post<DNSTestResult>('/dns-providers/test', data)
  return response.data
}
```

---

## 6. Files to Create/Modify

### New Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/pages/DNSProviders.tsx` | DNS providers management page |
| `frontend/src/components/DNSProviderForm.tsx` | Add/edit provider form |
| `frontend/src/components/DNSProviderSelector.tsx` | Provider dropdown selector |
| `frontend/src/components/DNSPropagationTest.tsx` | Propagation test UI |
| `frontend/src/api/dnsProviders.ts` | API client functions |
| `frontend/src/hooks/useDNSProviders.ts` | Data fetching hooks |
| `frontend/src/data/dnsProviderSchemas.ts` | Provider field definitions |

### Existing Files to Modify

| File | Modification |
|------|--------------|
| `frontend/src/App.tsx` | Add route for `/dns-providers` |
| `frontend/src/components/layout/Layout.tsx` | Add navigation link |
| `frontend/src/pages/Certificates.tsx` | Add DNS provider selector for wildcard requests |
| `frontend/src/components/ProxyHostForm.tsx` | Add DNS provider option when wildcard domain detected |
| `frontend/src/api/certificates.ts` | Add `dns_provider_id` to certificate request types |

---

## 7. Translation Keys Needed

Add to `frontend/src/locales/en/translation.json`:

```json
{
  "dnsProviders": {
    "title": "DNS Providers",
    "description": "Configure DNS providers for wildcard certificate issuance",
    "addProvider": "Add Provider",
    "editProvider": "Edit Provider",
    "deleteProvider": "Delete Provider",
    "deleteConfirm": "Are you sure you want to delete this DNS provider?",
    "testConnection": "Test Connection",
    "testSuccess": "DNS provider connection successful",
    "testFailed": "DNS provider test failed",
    "providerType": "Provider Type",
    "providerName": "Provider Name",
    "credentials": "Credentials",
    "setAsDefault": "Set as Default",
    "default": "Default",
    "status": {
      "active": "Active",
      "error": "Error",
      "unconfigured": "Not Configured"
    },
    "providers": {
      "cloudflare": "Cloudflare",
      "route53": "Amazon Route 53",
      "digitalocean": "DigitalOcean",
      "gcloud": "Google Cloud DNS",
      "namecheap": "Namecheap",
      "godaddy": "GoDaddy",
      "azure": "Azure DNS"
    },
    "fields": {
      "apiToken": "API Token",
      "apiKey": "API Key",
      "email": "Email",
      "accessKeyId": "Access Key ID",
      "secretAccessKey": "Secret Access Key",
      "region": "Region",
      "serviceAccountJson": "Service Account JSON",
      "projectId": "Project ID"
    },
    "hints": {
      "cloudflare": "Use an API token with Zone:DNS:Edit permissions",
      "route53": "IAM user with route53:ChangeResourceRecordSets permission",
      "digitalocean": "Personal access token with write scope"
    },
    "propagation": {
      "title": "DNS Propagation",
      "checking": "Checking DNS propagation...",
      "success": "DNS record propagated successfully",
      "failed": "DNS propagation check failed",
      "retry": "Retry Check"
    }
  }
}
```

---

## 8. UI/UX Considerations

### Provider Selection Flow

1. User creates proxy host with wildcard domain (e.g., `*.example.com`)
2. System detects wildcard and shows DNS provider requirement
3. User selects existing provider OR creates new one inline
4. On save, backend uses DNS challenge for certificate

### Security Considerations

- Credentials masked by default (password inputs)
- API tokens never returned in full from backend (masked)
- Clear warning about credential storage
- Option to test without saving

### Error Handling

- Clear error messages for invalid credentials
- Specific guidance for each provider's setup
- Link to provider documentation
- Retry mechanisms for transient failures

---

## 9. Implementation Priority

1. **Phase 1**: API layer and hooks
   - `dnsProviders.ts` API client
   - `useDNSProviders.ts` hooks

2. **Phase 2**: Core UI components
   - `DNSProviders.tsx` page
   - `DNSProviderForm.tsx` form

3. **Phase 3**: Integration
   - Navigation and routing
   - Certificate/ProxyHost form integration
   - Provider selector component

4. **Phase 4**: Polish
   - Translations
   - Error handling refinement
   - Propagation test UI

---

## 10. Related Backend Requirements

The frontend implementation depends on these backend API endpoints:

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/dns-providers` | List all providers |
| POST | `/api/v1/dns-providers` | Create provider |
| GET | `/api/v1/dns-providers/:id` | Get provider details |
| PUT | `/api/v1/dns-providers/:id` | Update provider |
| DELETE | `/api/v1/dns-providers/:id` | Delete provider |
| POST | `/api/v1/dns-providers/:id/test` | Test saved provider |
| POST | `/api/v1/dns-providers/test` | Test credentials before saving |

---

*Research completed: 2026-01-01*
