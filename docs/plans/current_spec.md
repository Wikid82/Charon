# User Management Consolidation & Privilege Tier System

Date: 2026-06-25
Owner: Planning Agent
Status: Proposed
Severity: Medium (UX consolidation + feature enhancement)

---

## 1. Introduction

### 1.1 Overview

Charon currently manages users through **two separate interfaces**:

1. **Admin Account** page at `/settings/account` — self-service profile management (name, email, password, API key, certificate email)
2. **Account Management** page at `/settings/account-management` — admin-only user CRUD (list, invite, delete, update roles/permissions)

This split is confusing and redundant. A logged-in admin must navigate to two different places to manage their own account versus managing other users. The system also lacks a formal privilege tier model — the `Role` field on the `User` model is a raw string defaulting to `"user"`, with only `"admin"` and `"user"` exposed in the frontend selector (the model comment mentions `"viewer"` but it is entirely unused).

### 1.2 Objectives

1. **Consolidate** user management into a single **"Users"** page that lists ALL users, including the admin.
2. **Introduce a formal privilege tier system** with three tiers: **Admin**, **User**, and **Pass-through**.
3. **Self-service profile editing** occurs inline or via a detail drawer/modal on the consolidated page — no separate "Admin Account" page.
4. **Remove** the `/settings/account` route and its sidebar/tab navigation entry entirely.
5. **Update navigation** to a single "Users" entry under Settings (or as a top-level item).
6. **Write E2E Playwright tests** for the consolidated page before any implementation changes.

---

## 2. Research Findings

### 2.1 Current Architecture: Backend

#### 2.1.1 User Model — `backend/internal/models/user.go`

```go
type User struct {
    ID                  uint           `json:"-" gorm:"primaryKey"`
    UUID                string         `json:"uuid" gorm:"uniqueIndex"`
    Email               string         `json:"email" gorm:"uniqueIndex"`
    APIKey              string         `json:"-" gorm:"uniqueIndex"`
    PasswordHash        string         `json:"-"`
    Name                string         `json:"name"`
    Role                string         `json:"role" gorm:"default:'user'"`  // "admin", "user", "viewer"
    Enabled             bool           `json:"enabled" gorm:"default:true"`
    FailedLoginAttempts int            `json:"-" gorm:"default:0"`
    LockedUntil         *time.Time     `json:"-"`
    LastLogin           *time.Time     `json:"last_login,omitempty"`
    SessionVersion      uint           `json:"-" gorm:"default:0"`
    // Invite system fields ...
    PermissionMode      PermissionMode `json:"permission_mode" gorm:"default:'allow_all'"`
    PermittedHosts      []ProxyHost    `json:"permitted_hosts,omitempty" gorm:"many2many:user_permitted_hosts;"`
    CreatedAt           time.Time      `json:"created_at"`
    UpdatedAt           time.Time      `json:"updated_at"`
}
```

**Key observations:**

| Aspect | Current State | Issue |
|--------|--------------|-------|
| `Role` field | Raw string, default `"user"` | No Go-level enum/const; `"viewer"` mentioned in comment but unused anywhere |
| Permission logic | `CanAccessHost()` — admins bypass all checks | Sound; needs extension for pass-through tier |
| JSON serialization | `ID` is `json:"-"`, `APIKey` is `json:"-"` | Correctly hidden from API responses |
| Password hashing | bcrypt via `SetPassword()` / `CheckPassword()` | Solid |

#### 2.1.2 Auth Service — `backend/internal/services/auth_service.go`

- `Register()`: First user becomes admin (`role = "admin"`)
- `Login()`: Account lockout after 5 failed attempts; sets `LastLogin`
- `GenerateToken()`: JWT with claims `{UserID, Role, SessionVersion}`, 24h expiry
- `AuthenticateToken()`: Validates JWT + checks `Enabled` + matches `SessionVersion`
- `InvalidateSessions()`: Increments `SessionVersion` to revoke all existing tokens

**Impact of role changes:** The `Role` string is embedded in the JWT. When a user's role is changed, their existing JWT still carries the old role until it expires or `SessionVersion` is bumped. The `InvalidateSessions()` method exists for this purpose, **but it is NOT currently called in `UpdateUser()`**. This is a security gap — a user demoted from admin to passthrough retains their admin JWT for up to 24 hours. Task 2.6 addresses this.

#### 2.1.3 Auth Middleware — `backend/internal/api/middleware/auth.go`

```go
func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc
func RequireRole(role string) gin.HandlerFunc
```

- `AuthMiddleware`: Extracts token from header/cookie/query, sets `userID` and `role` in Gin context.
- `RequireRole`: Checks `role` against a **single** required role string, always allows `"admin"` as fallback. Currently used only for SMTP settings.

**Gap:** Most admin-only endpoints in `user_handler.go` do inline `role != "admin"` checks via `requireAdmin(c)` in `permission_helpers.go` rather than using middleware. This works but is inconsistent.

#### 2.1.4 User Handler — `backend/internal/api/handlers/user_handler.go`

Two disjoint groups of endpoints:

| Group | Endpoints | Purpose | Auth |
|-------|-----------|---------|------|
| **Self-service** | `GET /user/profile`, `POST /user/profile`, `POST /user/api-key` | Current user edits own profile/API key | Any authenticated user |
| **Admin CRUD** | `GET /users`, `POST /users`, `POST /users/invite`, `GET /users/:id`, `PUT /users/:id`, `DELETE /users/:id`, `PUT /users/:id/permissions`, `POST /users/:id/resend-invite`, `POST /users/preview-invite-url` | Admin manages all users | Admin only (inline check) |

**Key handler behaviors:**

- `ListUsers()`: Returns all users with safe fields (excludes password hash, API key). Admin only.
- `UpdateUser()`: Can change `name`, `email`, `password`, `role`, `enabled`. **Does NOT call `InvalidateSessions()` on role change** — a demoted user retains their old-role JWT for up to 24 hours. This is a security gap that must be fixed in this spec (see Task 2.6).
- `DeleteUser()`: Prevents self-deletion.
- `GetProfile()`: Returns `{id, email, name, role, has_api_key, api_key_masked}` for the calling user.
- `UpdateProfile()`: Requires `current_password` verification when email changes.
- `RegenerateAPIKey()`: Generates new UUID-based API key for the calling user.

#### 2.1.5 Route Registration — `backend/internal/api/routes/routes.go`

```
Public:
  POST /api/v1/auth/login
  POST /api/v1/auth/register
  GET  /api/v1/auth/verify          (forward auth for Caddy)
  GET  /api/v1/auth/status
  GET  /api/v1/setup/status
  POST /api/v1/setup
  GET  /api/v1/invite/validate
  POST /api/v1/invite/accept

Protected (authMiddleware):
  POST /api/v1/auth/logout
  POST /api/v1/auth/refresh
  GET  /api/v1/auth/me
  POST /api/v1/auth/change-password
  GET  /api/v1/user/profile          ← Self-service group
  POST /api/v1/user/profile          ←
  POST /api/v1/user/api-key          ←
  GET    /api/v1/users               ← Admin CRUD group
  POST   /api/v1/users               ←
  POST   /api/v1/users/invite        ←
  POST   /api/v1/users/preview-invite-url ←
  GET    /api/v1/users/:id           ←
  PUT    /api/v1/users/:id           ←
  DELETE /api/v1/users/:id           ←
  PUT    /api/v1/users/:id/permissions ←
  POST   /api/v1/users/:id/resend-invite ←
```

#### 2.1.6 Forward Auth — `backend/internal/api/handlers/auth_handler.go`

The `Verify()` handler at `GET /api/v1/auth/verify` is the critical integration point for the pass-through tier:

1. Extracts token from cookie/header/query
2. Authenticates user via `AuthenticateToken()`
3. Reads `X-Forwarded-Host` from Caddy
4. Checks `user.CanAccessHost(hostID)` against the permission model
5. Returns `200 OK` with `X-Auth-User` headers or `401`/`403`

**Pass-through users will use this exact path** — they log in to Charon only to establish a session, then Caddy's `forward_auth` directive validates their access to proxied services. They never need to see the Charon management UI.

### 2.2 Current Architecture: Frontend

#### 2.2.1 Router — `frontend/src/App.tsx`

```tsx
// Three routes serve user management:
<Route path="users" element={<UsersPage />} />                    // top-level
<Route path="settings" element={<Settings />}>
  <Route path="account" element={<Account />} />                  // self-service
  <Route path="account-management" element={<UsersPage />} />     // admin CRUD
</Route>
```

The `UsersPage` component is mounted at **two** routes: `/users` and `/settings/account-management`. The `Account` component is only at `/settings/account`.

#### 2.2.2 Settings Page — `frontend/src/pages/Settings.tsx`

Tab bar with 4 items: `System`, `Notifications`, `SMTP`, `Account`. Note that **Account Management is NOT in the tab bar** — it is only reachable via the sidebar. The `Account` tab links to `/settings/account` (the self-service profile page).

#### 2.2.3 Sidebar Navigation — `frontend/src/components/Layout.tsx`

Under the Settings section, two entries:

```tsx
{ name: t('navigation.adminAccount'), path: '/settings/account', icon: '🛡️' },
{ name: t('navigation.accountManagement'), path: '/settings/account-management', icon: '👥' },
```

#### 2.2.4 Account Page — `frontend/src/pages/Account.tsx`

Four cards:
1. **Profile** — name, email (with password verification for email changes)
2. **Certificate Email** — toggle between account email and custom email for ACME
3. **Password Change** — old/new/confirm password form
4. **API Key** — masked display + regenerate button

Uses `api/user.ts` (singular) — endpoints: `GET /user/profile`, `POST /user/profile`, `POST /user/api-key`.

#### 2.2.5 Users Page — `frontend/src/pages/UsersPage.tsx`

~900 lines. Contains:
- `InviteModal` — email, role select (`user`/`admin`), permission mode, host selection, URL preview
- `PermissionsModal` — permission mode + host checkboxes per user
- `UsersPage` (default export) — table listing all users with columns: User (name/email), Role (badge), Status (active/pending/expired), Permissions (whitelist/blacklist), Enabled (toggle), Actions (resend/permissions/delete)

**Current limitations:**
- Admin users cannot be disabled (switch is `disabled` when `role === 'admin'`)
- Admin users cannot be deleted (delete button is `disabled` when `role === 'admin'`)
- No permission editing for admins (gear icon hidden when `role !== 'admin'`)
- Role selector in `InviteModal` only has `user` and `admin` options
- No inline profile editing — admins can't edit their own name/email/password from this page
- The admin's own row appears in the list but has limited interactivity

#### 2.2.6 API Client Files

**`frontend/src/api/user.ts`** (singular — self-service):

```tsx
interface UserProfile { id, email, name, role, has_api_key, api_key_masked }
getProfile()         → GET  /user/profile
updateProfile(data)  → POST /user/profile
regenerateApiKey()   → POST /user/api-key
```

**`frontend/src/api/users.ts`** (plural — admin CRUD):

```tsx
interface User { id, uuid, email, name, role, enabled, last_login, invite_status, ... }
type PermissionMode = 'allow_all' | 'deny_all'
listUsers()                  → GET    /users
getUser(id)                  → GET    /users/:id
createUser(data)             → POST   /users
inviteUser(data)             → POST   /users/invite
updateUser(id, data)         → PUT    /users/:id
deleteUser(id)               → DELETE /users/:id
updateUserPermissions(id, d) → PUT    /users/:id/permissions
validateInvite(token)        → GET    /invite/validate
acceptInvite(data)           → POST   /invite/accept
previewInviteURL(email)      → POST   /users/preview-invite-url
resendInvite(id)             → POST   /users/:id/resend-invite
```

#### 2.2.7 Auth Context — `frontend/src/context/AuthContextValue.ts`

```tsx
interface User { user_id: number; role: string; name?: string; email?: string; }
interface AuthContextType { user, login, logout, changePassword, isAuthenticated, isLoading }
```

The `AuthProvider` in `AuthContext.tsx` fetches `/api/v1/auth/me` on mount and stores the user. Auto-logout after 15 minutes of inactivity.

#### 2.2.8 Route Protection — `frontend/src/components/RequireAuth.tsx`

Checks `isAuthenticated`, localStorage token, and `user !== null`. Redirects to `/login` if any check fails. **No role-based route protection exists** — all authenticated users see the same navigation and routes.

#### 2.2.9 Translation Keys — `frontend/src/locales/en/translation.json`

Relevant keys:
```json
"navigation.adminAccount": "Admin Account",
"navigation.accountManagement": "Account Management",
"users.roleUser": "User",
"users.roleAdmin": "Admin"
```

No keys exist for `"roleViewer"` or `"rolePassthrough"`.

### 2.3 Current Architecture: E2E Tests

**No Playwright E2E tests exist** for user management or account pages. The `tests/` directory contains specs for DNS providers, modal dropdowns, and debug utilities only.

### 2.4 Configuration Files Review

| File | Relevance | Action Needed |
|------|-----------|---------------|
| `.gitignore` | May need entries for new test artifacts | Review during implementation |
| `codecov.yml` | Coverage thresholds may need adjustment | Review if new files change coverage ratios |
| `.dockerignore` | No impact expected | No changes |
| `Dockerfile` | No impact expected | No changes |

---

## 3. Technical Specifications

### 3.1 Privilege Tier System

#### 3.1.1 Tier Definitions

| Tier | Value | Charon UI Access | Forward Auth (Proxy) Access | Can Manage Others |
|------|-------|------------------|---------------------------|-------------------|
| **Admin** | `"admin"` | Full access to all pages and settings | All hosts (bypasses permission checks) | Yes — full CRUD on all users |
| **User** | `"user"` | Access to Charon UI except admin-only pages (user management, system settings) | Based on `permission_mode` + `permitted_hosts` | No |
| **Pass-through** | `"passthrough"` | Login page only — redirected away from all management pages after login | Based on `permission_mode` + `permitted_hosts` | No |

#### 3.1.2 Behavioral Details

**Admin:**
- Unchanged from current behavior.
- Can edit any user's role, permissions, name, email, enabled state.
- Can edit their own profile inline on the consolidated Users page.
- Cannot delete themselves. Cannot disable themselves.

**User:**
- Can log in to the Charon management UI.
- Can view proxy hosts, certificates, DNS, uptime, and other read-oriented pages.
- Cannot access: User management, System settings, SMTP settings, Encryption, Plugins.
- Can edit their own profile (name, email, password, API key) via self-service section on the Users page or a dedicated "My Account" modal.
- Forward auth access is governed by `permission_mode` + `permitted_hosts`.

**Pass-through:**
- Can log in to Charon (to obtain a session cookie/JWT).
- Immediately after login, is redirected to `/passthrough` landing page — **no access to the management UI**.
- The session cookie is used by Caddy's `forward_auth` to grant access to proxied services based on `permission_mode` + `permitted_hosts`.
- Can access **only**: `/auth/logout`, `/auth/refresh`, `/auth/me`, `/auth/change-password`.
- **Cannot access**: `/user/profile`, `/user/api-key`, or any management API endpoints.
- Cannot access any Charon management UI pages.

#### 3.1.3 Backend Role Constants

Add typed constants to `backend/internal/models/user.go`:

```go
type UserRole string

const (
    RoleAdmin       UserRole = "admin"
    RoleUser        UserRole = "user"
    RolePassthrough UserRole = "passthrough"
)

func (r UserRole) IsValid() bool {
    switch r {
    case RoleAdmin, RoleUser, RolePassthrough:
        return true
    }
    return false
}
```

Change `User.Role` from `string` to `UserRole` (which is still a `string` alias, so GORM and JSON work identically — zero migration friction).

### 3.2 Database Schema Changes

#### 3.2.1 User Table

**No schema migration needed.** The `Role` column is already a `TEXT` field storing string values. Changing the Go type from `string` to `UserRole` (a `string` alias) requires no ALTER TABLE. New role values (`"passthrough"`) are immediately storable.

#### 3.2.2 Data Migration

A one-time migration function should:
1. Scan for any users with `role = "viewer"` and update them to `role = "passthrough"` (in case any were manually created via the unused viewer path).
2. Validate all existing roles are in the allowed set.

This runs in the `AutoMigrate` block in `routes.go`.

The migration must be in a clearly named function (e.g., `migrateViewerToPassthrough()`) and must log when it runs and how many records it affected:

```go
func migrateViewerToPassthrough(db *gorm.DB) {
    result := db.Model(&User{}).Where("role = ?", "viewer").Update("role", string(RolePassthrough))
    if result.RowsAffected > 0 {
        log.Printf("[migration] Updated %d users from 'viewer' to 'passthrough' role", result.RowsAffected)
    }
}
```

### 3.3 API Changes

#### 3.3.1 Consolidated Self-Service Endpoints

Merge the current `/user/profile` and `/user/api-key` endpoints into the existing `/users/:id` group by allowing users to edit their own record:

| Current Endpoint | Action | Proposed |
|-----------------|--------|----------|
| `GET /user/profile` | Get own profile | **Keep** (convenience alias) |
| `POST /user/profile` | Update own profile | **Keep** (convenience alias) |
| `POST /user/api-key` | Regenerate API key | **Keep** (convenience alias) |
| `PUT /users/:id` | Admin updates any user | **Extend**: allow authenticated user to update their own record (name, email with password verification) |

The self-service endpoints (`/user/*`) remain as convenient aliases that internally resolve to the caller's own user ID and delegate to the same service logic. No breaking API changes.

#### 3.3.2 Role Validation

The `UpdateUser()` and `CreateUser()` handlers must validate the `role` field against `UserRole.IsValid()`. The invite endpoint `InviteUser()` must accept `"passthrough"` as a valid role.

#### 3.3.3 Pass-through Route Protection

Add middleware to block pass-through users from accessing management endpoints:

```go
func RequireManagementAccess() gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("role")
        if role == string(models.RolePassthrough) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "pass-through users cannot access management features",
            })
            return
        }
        c.Next()
    }
}
```

Apply this middleware by restructuring the protected routes into two sub-groups:

**Exempt routes** (authMiddleware only — no management check):
```
POST /auth/logout
POST /auth/refresh
GET  /auth/me
POST /auth/change-password
GET  /auth/accessible-hosts
GET  /auth/check-host/:hostId
GET  /user/profile
POST /user/profile
POST /user/api-key
```

**Management routes** (authMiddleware + `RequireManagementAccess`):
```
GET    /users
POST   /users
POST   /users/invite
POST   /users/preview-invite-url
GET    /users/:id
PUT    /users/:id
DELETE /users/:id
PUT    /users/:id/permissions
POST   /users/:id/resend-invite
All /backup/* routes
All /logs/* routes
All /settings/* routes
All /system/* routes
All /security/* routes
All /features/* routes
All /proxy/* routes
All /certificates/* routes
All /dns/* routes
All /uptime/* routes
All /plugins/* routes
```

The route split in `routes.go` should look like:

```go
// Self-service + auth routes — accessible to all authenticated users (including passthrough)
exempt := protected.Group("")
{
    exempt.POST("/auth/logout", ...)
    exempt.POST("/auth/refresh", ...)
    exempt.GET("/auth/me", ...)
    exempt.POST("/auth/change-password", ...)
    exempt.GET("/auth/accessible-hosts", ...)
    exempt.GET("/auth/check-host/:hostId", ...)
    exempt.GET("/user/profile", ...)
    exempt.POST("/user/profile", ...)
    exempt.POST("/user/api-key", ...)
}

// Management routes — blocked for passthrough users
management := protected.Group("", middleware.RequireManagementAccess())
{
    // All other routes registered here
}
```

#### 3.3.4 New Endpoint: Pass-through Landing

No new backend endpoint needed. The forward auth flow (`GET /auth/verify`) already works for pass-through users. The minimal landing page is purely a frontend concern.

### 3.4 Frontend Changes

#### 3.4.1 Consolidated Users Page

Transform `UsersPage.tsx` into the single source of truth for all user management:

**New capabilities:**
1. **Admin's own row is fully interactive** — edit name/email/password, regenerate API key, view cert email settings.
2. **"My Profile" section** at the top (or accessible via a button) for the currently logged-in user to manage their own account, replacing the `Account.tsx` page entirely.
3. **Role selector** includes three options: `Admin`, `User`, `Pass-through`.
4. **Permission controls** are shown for `User` and `Pass-through` roles (hidden for `Admin`).
5. **Inline editing** or a detail drawer for editing individual users.

**Components to modify:**

| Component | File | Change |
|-----------|------|--------|
| `InviteModal` | `UsersPage.tsx` | Add `"passthrough"` to role `<select>`. Show permission controls for both `user` and `passthrough`. |
| `PermissionsModal` | `UsersPage.tsx` | No functional changes — already works for any non-admin role. |
| `UsersPage` (table) | `UsersPage.tsx` | Add role badge color for `passthrough`. Remove `disabled` from admin row's enable/delete where appropriate. Add "Edit" action for all users. Add "My Profile" card or section. |
| New: `UserDetailModal` | `UsersPage.tsx` or separate file | Modal/drawer for editing a user's full details (including password reset, API key regen for self). Replaces the Account page's four cards. |

#### 3.4.2 Remove Account Page

| Action | File | Detail |
|--------|------|--------|
| Delete | `frontend/src/pages/Account.tsx` | Remove entirely |
| Delete | `frontend/src/api/user.ts` | Remove (merge any unique types into `users.ts`) |
| Remove route | `frontend/src/App.tsx` | Remove `<Route path="account" element={<Account />} />` |
| Remove nav entry | `frontend/src/components/Layout.tsx` | Remove `{ name: t('navigation.adminAccount'), path: '/settings/account', icon: '🛡️' }` |
| Remove tab | `frontend/src/pages/Settings.tsx` | Remove `{ path: '/settings/account', label: t('settings.account'), icon: User }` |
| Remove translation keys | `frontend/src/locales/en/translation.json` | Remove `navigation.adminAccount`, update `navigation.accountManagement` → `navigation.users` |

#### 3.4.3 Navigation Update

**Sidebar** (Layout.tsx): Under Settings, consolidate to a single entry:

```tsx
{ name: t('navigation.users'), path: '/settings/users', icon: '👥' }
```

**Settings tab bar** (Settings.tsx): Add "Users" as a new tab, remove "Account":

```tsx
{ path: '/settings/users', label: t('navigation.users'), icon: Users }
```

**Router** (App.tsx): Update routes:

```tsx
{/* Pass-through landing — accessible to all authenticated users */}
<Route path="passthrough" element={<RequireAuth><PassthroughLanding /></RequireAuth>} />

<Route path="settings" element={<RequireRole allowed={['admin', 'user']}><Settings /></RequireRole>}>
  <Route index element={<SystemSettings />} />
  <Route path="system" element={<SystemSettings />} />
  <Route path="notifications" element={<Notifications />} />
  <Route path="smtp" element={<SMTPSettings />} />
  <Route path="users" element={<RequireRole allowed={['admin']}><UsersPage /></RequireRole>} />
</Route>
```

Add redirects for old paths:
```tsx
<Route path="settings/account" element={<Navigate to="/settings/users" replace />} />
<Route path="settings/account-management" element={<Navigate to="/settings/users" replace />} />
<Route path="users" element={<Navigate to="/settings/users" replace />} />
```

#### 3.4.4 Role-Based Route Protection

Create a new `RequireRole` wrapper component:

```tsx
// frontend/src/components/RequireRole.tsx
const RequireRole: React.FC<{ allowed: string[]; children: React.ReactNode }> = ({ allowed, children }) => {
  const { user } = useAuth();
  if (!user) {
    return <Navigate to="/login" replace />;
  }
  if (!allowed.includes(user.role)) {
    // Role-aware redirect: pass-through users go to their landing page,
    // other unauthorized roles go to dashboard
    const redirectTarget = user.role === 'passthrough' ? '/passthrough' : '/';
    return <Navigate to={redirectTarget} replace />;
  }
  return children;
};
```

Wrap admin-only routes (user management, system settings) with `<RequireRole allowed={['admin']}>`.

For pass-through users, redirect all management routes to a minimal landing page:

```tsx
// frontend/src/pages/PassthroughLanding.tsx
// Minimal page: "You are logged in. Your session grants access to authorized services."
// Shows: user name, logout button, change password link, optionally a list of accessible hosts.
// Route: /passthrough (must be registered in App.tsx router)
//
// Accessibility requirements (WCAG 2.2 AA):
// - Use semantic HTML: <main>, <h1>, <nav>
// - Logout button must be keyboard focusable
// - Announce page purpose via <title> and <h1>
// - Sufficient color contrast on all text (4.5:1 minimum)
```

#### 3.4.5 Auth Context Update

Update the `User` interface in `AuthContextValue.ts`:

```tsx
export interface User {
  user_id: number;
  role: 'admin' | 'user' | 'passthrough';
  name?: string;
  email?: string;
}
```

#### 3.4.6 API Types Consolidation

Merge `frontend/src/api/user.ts` into `frontend/src/api/users.ts`:

- Move `UserProfile` interface into `users.ts` (or inline it).
- Keep `getProfile()`, `updateProfile()`, `regenerateApiKey()` functions in `users.ts`.
- Delete `user.ts`.

Update the `User` role type:

```tsx
role: 'admin' | 'user' | 'passthrough'
```

#### 3.4.7 Translation Keys

Add to `frontend/src/locales/en/translation.json`:

```json
"users.rolePassthrough": "Pass-through",
"users.rolePassthroughDescription": "Can access proxied services only — no Charon management access",
"users.roleUserDescription": "Can view Charon management UI with limited permissions",
"users.roleAdminDescription": "Full access to all Charon features and settings",
"users.myProfile": "My Profile",
"users.editUser": "Edit User",
"users.changePassword": "Change Password",
"users.apiKey": "API Key",
"users.regenerateApiKey": "Regenerate API Key",
"navigation.users": "Users",
"passthrough.title": "Welcome to Charon",
"passthrough.description": "Your session grants access to authorized services through the reverse proxy.",
"passthrough.noAccessToManagement": "You do not have access to the management interface."
```

Remove:
```json
"navigation.adminAccount"  → delete
```

Rename:
```json
"navigation.accountManagement" → "navigation.users"
```

### 3.5 Forward Auth & Pass-through Integration

The existing `Verify()` handler in `auth_handler.go` already handles pass-through users correctly:

1. User logs in → gets JWT with `role: "passthrough"`.
2. User's browser sends cookie with each request to proxied services.
3. Caddy's `forward_auth` calls `GET /api/v1/auth/verify`.
4. `Verify()` authenticates the token, checks `user.CanAccessHost(hostID)`.
5. Pass-through users have `permission_mode` + `permitted_hosts` just like regular users.

**No changes needed to the forward auth flow.** The `CanAccessHost()` method already handles non-admin roles correctly.

**Note on X-Forwarded-Groups header:** The `Verify()` handler currently sets `X-Auth-User` headers for upstream services. It does NOT expose the user's role via `X-Forwarded-Groups` or similar headers. For this iteration, the pass-through role is **not exposed** to upstream services — they only see the authenticated user identity. If upstream services need to differentiate pass-through from regular users, a future enhancement can add `X-Auth-Role` headers.

### 3.6 Error Handling & Edge Cases

| Scenario | Expected Behavior | Status |
|----------|-------------------|--------|
| Admin demotes themselves | Prevent — return 400 "Cannot change your own role" | **Needs implementation** |
| Admin disables themselves | Prevent — return 400 "Cannot disable your own account" | **Needs implementation** |
| Last admin is demoted | Prevent — return 400 "At least one admin must exist". **Must use a DB transaction** around the count-check-and-update to prevent race conditions (see section 3.6.1). | **Needs implementation** |
| Concurrent demotion of last two admins | DB transaction serializes the check — second request fails atomically | **Needs implementation** |
| Pass-through user accesses `/api/v1/users` | Return 403 "pass-through users cannot access management features" | **Needs implementation** |
| Pass-through user accesses Charon UI routes | Frontend redirects to `/passthrough` landing page (NOT `/` which would loop) | **Needs implementation** |
| User with active session has role changed | `InvalidateSessions(user.ID)` **must be called** in `UpdateUser()` when role changes — **not currently implemented** (see Task 2.6) | **Needs implementation** |
| Non-admin edits own record via `PUT /users/:id` | Handler **must reject** changes to `role`, `enabled`, or `permissions` fields — only `name`, `email` (with password verification), and `password` are self-editable (see Task 2.11) | **Needs implementation** |
| Invite with `role: "passthrough"` | Valid — creates pending pass-through user | **Needs validation** |
| Pass-through user changes own password | Allowed via `/auth/change-password` (exempt from management middleware) | Already works |

#### 3.6.1 Last-Admin Race Condition Protection

Two concurrent demotion requests could both pass the admin count check before either commits. SQLite serializes writes, but the spec mandates a transaction for correctness by design:

```go
func (h *UserHandler) updateUserRole(tx *gorm.DB, userID uint, newRole models.UserRole, currentUser uint) error {
    // Must run inside a transaction to prevent race conditions
    return tx.Transaction(func(inner *gorm.DB) error {
        // 1. Prevent self-demotion
        if userID == currentUser {
            return fmt.Errorf("cannot change your own role")
        }

        // 2. If demoting from admin, check admin count atomically
        var user models.User
        if err := inner.First(&user, userID).Error; err != nil {
            return err
        }

        if user.Role == models.RoleAdmin && newRole != models.RoleAdmin {
            var adminCount int64
            if err := inner.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount).Error; err != nil {
                return err
            }
            if adminCount <= 1 {
                return fmt.Errorf("at least one admin must exist")
            }
        }

        // 3. Update role
        if err := inner.Model(&user).Update("role", string(newRole)).Error; err != nil {
            return err
        }

        return nil
    })
}
```

After the transaction succeeds, the handler calls `authService.InvalidateSessions(userID)` to revoke existing JWTs.

### 3.6.2 User Tier Permission Matrix

Complete access matrix for all protected route groups by role:

| Route Group | Admin | User | Pass-through |
|-------------|-------|------|--------------|
| `/auth/logout`, `/auth/refresh`, `/auth/me` | Full | Full | Full |
| `/auth/change-password` | Full | Full | Full |
| `/auth/accessible-hosts`, `/auth/check-host/:hostId` | Full | Full | Full |
| `/user/profile` (GET, POST) | Full | Full | **Blocked** |
| `/user/api-key` (POST) | Full | Full | **Blocked** |
| `/users` (list, create, invite) | Full | **Blocked** | **Blocked** |
| `/users/:id` (get, update, delete) | Full | **Blocked** (except self-edit of own name/email/password) | **Blocked** |
| `/users/:id/permissions` | Full | **Blocked** | **Blocked** |
| `/proxy/*` (all proxy host routes) | Full | **Read-only** | **Blocked** |
| `/certificates/*` | Full | **Read-only** | **Blocked** |
| `/dns/*` | Full | **Read-only** | **Blocked** |
| `/uptime/*` | Full | **Read-only** | **Blocked** |
| `/settings/*` (system, SMTP, encryption) | Full | **Blocked** | **Blocked** |
| `/backup/*` | Full | **Blocked** | **Blocked** |
| `/logs/*` | Full | **Read-only** | **Blocked** |
| `/security/*` | Full | **Blocked** | **Blocked** |
| `/features/*` | Full | **Blocked** | **Blocked** |
| `/plugins/*` | Full | **Blocked** | **Blocked** |
| `/system/*` | Full | **Blocked** | **Blocked** |

**Enforcement strategy:** Pass-through blocking is handled by `RequireManagementAccess` middleware. User-tier read-only vs blocked distinctions are enforced by existing inline `requireAdmin()` checks, which will be consolidated to use `UserRole` constants. A future PR may extract these into dedicated middleware, but for this iteration the inline checks remain.

#### 3.6.3 Pass-through Self-Service Scope

Pass-through users have the **most restricted** self-service access. They can access:
- `/auth/logout` — end their session
- `/auth/refresh` — refresh their JWT
- `/auth/me` — view their own identity
- `/auth/change-password` — change their own password

Pass-through users **cannot** access:
- `/user/profile` — no profile editing (name/email changes are not applicable)
- `/user/api-key` — no API key management

This means the exempt route group must further restrict `/user/*` endpoints for pass-through users. Implementation: the `/user/profile` and `/user/api-key` endpoints remain in the exempt group (accessible to admin and user roles) but add an inline check that rejects pass-through callers:

```go
func (h *UserHandler) GetProfile(c *gin.Context) {
    role, _ := c.Get("role")
    if role == string(models.RolePassthrough) {
        c.JSON(http.StatusForbidden, gin.H{"error": "pass-through users cannot access profile management"})
        return
    }
    // ... existing logic
}
```

### 3.7 Data Flow Diagram

```
┌─────────────┐    Login     ┌─────────┐    JWT Cookie     ┌───────────┐
│  Browser    │──────────────▶│ Charon  │──────────────────▶│  Caddy    │
│  (User)     │              │ Backend │                   │  Proxy    │
└──────┬──────┘              └────┬────┘                   └─────┬─────┘
       │                         │                               │
       │  Admin/User Role        │                               │
       │  ───────────────▶       │                               │
       │  Charon UI pages        │                               │
       │                         │                               │
       │  Pass-through Role      │  forward_auth                 │
       │  ───────────────▶       │◀──────────────────────────────┤
       │  Landing page only      │  GET /auth/verify             │
       │                         │  → 200 OK (access granted)    │
       │                         │  → 403 (access denied)        │
       │                         │                               │
       │◀────────────────────────┤◀──────────────────────────────┤
       │  Proxied service        │  Proxy pass to upstream       │
       │  response               │                               │
```

---

## 4. Implementation Plan

### Phase 1: E2E Tests (Write-First)

Write Playwright tests for the CURRENT behavior so we have a regression baseline. Then update them for the new design.

| Task | Description | Files |
|------|-------------|-------|
| 1.1 | Write E2E tests for current Account page (`/settings/account`) | `tests/account.spec.ts` |
| 1.2 | Write E2E tests for current Users page (`/settings/account-management`) | `tests/users.spec.ts` |
| 1.3 | Write E2E tests for invite flow (invite → accept → login) | `tests/user-invite.spec.ts` |
| 1.4 | Update tests to match the consolidated design (new routes, new role options, pass-through behavior) | All above files |

### Phase 2: Backend Implementation

| Task | Description | Files | Dependencies |
|------|-------------|-------|-------------|
| 2.1 | Add `UserRole` type with constants and `IsValid()` method | `backend/internal/models/user.go` | None |
| 2.2 | Change `User.Role` field type from `string` to `UserRole` | `backend/internal/models/user.go` | 2.1 |
| 2.3 | Add data migration for `"viewer"` → `"passthrough"` | `backend/internal/api/routes/routes.go` (AutoMigrate block) | 2.1 |
| 2.4 | Add `RequireManagementAccess()` middleware | `backend/internal/api/middleware/auth.go` | 2.1 |
| 2.5 | Apply middleware to protected routes, exempt self-service + auth endpoints | `backend/internal/api/routes/routes.go` | 2.4 |
| 2.6 | **[HIGH SEVERITY]** Update `UpdateUser()`: (a) validate role against `UserRole.IsValid()`, (b) **call `authService.InvalidateSessions(user.ID)` when the `role` field changes**, (c) validate role in `CreateUser()` and `InviteUser()` | `backend/internal/api/handlers/user_handler.go` | 2.1 |
| 2.7 | Add self-demotion and last-admin protection to `UpdateUser()` using a **DB transaction** around the admin count check-and-update (see section 3.6.1 for reference implementation) | `backend/internal/api/handlers/user_handler.go` | 2.1 |
| 2.8 | Update `RequireRole()` middleware — note: current signature is `func RequireRole(role string)` (single argument, not variadic). Update to accept `UserRole` type: `func RequireRole(role models.UserRole)` | `backend/internal/api/middleware/auth.go` | 2.1 |
| 2.9 | Update all inline `role != "admin"` checks to use `UserRole` constants. **Including `CanAccessHost()` in `user.go`** which currently hardcodes `"admin"` string — update to use `models.RoleAdmin`. | `backend/internal/api/handlers/permission_helpers.go`, `user_handler.go`, `backend/internal/models/user.go` | 2.1 |
| 2.10 | Update existing Go unit tests for role changes | `backend/internal/models/user_test.go`, handler tests | 2.1–2.9 |
| 2.11 | **[PRIVILEGE ESCALATION GUARD]** Add field-level protection in `UpdateUser()`: when the caller is NOT an admin editing another user's record (i.e., a non-admin editing their own record via `PUT /users/:id`), the handler **must reject** any attempt to modify `role`, `enabled`, or permission fields. Only `name`, `email` (with password verification), and `password` are self-editable. | `backend/internal/api/handlers/user_handler.go` | 2.6 |

### Phase 3: Frontend Implementation

| Task | Description | Files | Dependencies |
|------|-------------|-------|-------------|
| 3.1 | Update `User` role type in `api/users.ts` to include `'passthrough'` | `frontend/src/api/users.ts` | None |
| 3.2 | Merge `api/user.ts` functions into `api/users.ts` | `frontend/src/api/users.ts`, delete `frontend/src/api/user.ts` | 3.1 |
| 3.3 | Update `AuthContextValue.ts` `User` interface role type | `frontend/src/context/AuthContextValue.ts` | None |
| 3.4 | Create `RequireRole` component | `frontend/src/components/RequireRole.tsx` | 3.3 |
| 3.5 | Create `PassthroughLanding` page | `frontend/src/pages/PassthroughLanding.tsx` | 3.3 |
| 3.6 | Add `UserDetailModal` component to `UsersPage.tsx` (inline profile editing, password, API key) | `frontend/src/pages/UsersPage.tsx` | 3.1, 3.2 |
| 3.7 | Add "My Profile" section/card to `UsersPage` | `frontend/src/pages/UsersPage.tsx` | 3.6 |
| 3.8 | Update `InviteModal` — add `'passthrough'` to role select with descriptions | `frontend/src/pages/UsersPage.tsx` | 3.1 |
| 3.9 | Update user table — pass-through badge, edit actions, admin row interactivity | `frontend/src/pages/UsersPage.tsx` | 3.1 |
| 3.10 | Delete `Account.tsx` page | Delete `frontend/src/pages/Account.tsx` | 3.6, 3.7 |
| 3.11 | Update routes in `App.tsx` — remove old paths, add redirects, add pass-through routing | `frontend/src/App.tsx` | 3.4, 3.5, 3.10 |
| 3.12 | Update sidebar navigation in `Layout.tsx` — single "Users" entry | `frontend/src/components/Layout.tsx` | 3.11 |
| 3.13 | Update Settings tab bar in `Settings.tsx` — replace "Account" with "Users" | `frontend/src/pages/Settings.tsx` | 3.11 |
| 3.14 | Add role-based navigation hiding (pass-through sees minimal nav, user sees limited nav) | `frontend/src/components/Layout.tsx` | 3.3, 3.4 |
| 3.15 | Add translation keys for pass-through role, descriptions, landing page | `frontend/src/locales/en/translation.json` (and other locale files) | None |
| 3.16 | Update frontend unit tests (`Account.test.tsx`, `Settings.test.tsx`, `useAuth.test.tsx`) | Various `__tests__/` files | 3.10–3.14 |
| 3.17 | **Audit all locale directories** for stale keys. Remove `navigation.adminAccount` and any other orphaned keys from ALL locales (not just English). Verify all new keys (`users.rolePassthrough`, `passthrough.*`, etc.) are added to every locale file with at least the English fallback. | All files in `frontend/src/locales/*/translation.json` | 3.15 |
| 3.18 | **Accessibility requirements** for new components: (a) `PassthroughLanding` — semantic landmarks (`<main>`, `<h1>`), sufficient contrast, keyboard-operable logout button. (b) `UserDetailModal` — focus trap, Escape to close, `aria-modal="true"`, `role="dialog"`, `aria-labelledby` pointing to modal title heading. (c) `RequireRole` redirect must not cause focus loss — ensure redirected page receives focus on its `<h1>` or `<main>`. | `PassthroughLanding.tsx`, `UsersPage.tsx`, `RequireRole.tsx` | 3.4, 3.5, 3.6 |

### Phase 4: Integration & Validation

| Task | Description |
|------|-------------|
| 4.1 | Run updated E2E tests against consolidated page |
| 4.2 | Test pass-through login flow end-to-end (login → redirect → forward auth working) |
| 4.3 | Test role change flows (admin → user, user → passthrough, passthrough → admin) |
| 4.4 | Test last-admin protection (cannot demote or delete the last admin) |
| 4.5 | Test invite flow with all three roles |
| 4.6 | Verify forward auth still works correctly for all roles |
| 4.7 | Run full backend test suite |
| 4.8 | Run full frontend test suite |
| 4.9 | Generate coverage reports |

### Phase 5: Documentation & Cleanup

| Task | Description |
|------|-------------|
| 5.1 | Update `docs/features.md` with privilege tier documentation |
| 5.2 | Update `CHANGELOG.md` |
| 5.3 | Remove unused translation keys |
| 5.4 | Review `.gitignore` and `codecov.yml` for any needed updates |
| 5.5 | Archive this plan to `docs/plans/` with completion status |

---

## 5. Acceptance Criteria

### 5.1 Functional Requirements

| ID | Requirement (EARS) | Verification |
|----|-------------------|-------------|
| F1 | WHEN an admin navigates to `/settings/users`, THE SYSTEM SHALL display a table listing ALL users including the admin's own account. | E2E test |
| F2 | WHEN an admin clicks "Edit" on any user row, THE SYSTEM SHALL open a detail modal showing name, email, role, permissions, enabled state, and (for self) password change and API key management. | E2E test |
| F3 | WHEN an admin invites a user, THE SYSTEM SHALL offer three role options: Admin, User, and Pass-through. | E2E test |
| F4 | WHEN a pass-through user logs in, THE SYSTEM SHALL redirect them to a minimal landing page and block access to all management routes. | E2E test |
| F5 | WHEN a pass-through user's browser sends a request through Caddy with a valid session, THE SYSTEM SHALL evaluate `CanAccessHost()` and return 200 or 403 via the forward auth endpoint. | Integration test |
| F6 | WHEN an admin attempts to demote themselves, THE SYSTEM SHALL reject the request with a 400 error. | Backend unit test |
| F7 | WHEN an admin attempts to demote the last remaining admin, THE SYSTEM SHALL reject the request with a 400 error. | Backend unit test |
| F8 | WHEN a user navigates to `/settings/account`, THE SYSTEM SHALL redirect to `/settings/users`. | E2E test |
| F9 | WHEN a user (non-admin) is logged in, THE SYSTEM SHALL hide admin-only navigation items (user management, system settings). | E2E test |
| F10 | THE SYSTEM SHALL provide self-service profile management (name, email, password, API key) accessible to all non-passthrough roles from the Users page. | E2E test |

### 5.2 Non-Functional Requirements

| ID | Requirement | Verification |
|----|------------|-------------|
| NF1 | THE SYSTEM SHALL maintain backward compatibility — existing JWTs with `role: "user"` or `role: "admin"` continue working without user action. | Manual test |
| NF2 | THE SYSTEM SHALL require no database migration — the `UserRole` type alias does not alter the SQLite schema. | AutoMigrate verification |
| NF3 | All new UI components SHALL meet WCAG 2.2 Level AA accessibility standards. | Lighthouse/manual audit |

---

## 6. PR Slicing Strategy

### 6.1 Decision: Multiple PRs

**Trigger reasons:**
- Cross-domain changes (backend model + middleware + frontend pages + navigation + tests)
- High review complexity if combined (~30+ files changed)
- Independent validation gates per slice reduce risk
- Backend changes can be deployed and verified before frontend changes land

### 6.2 PR Slices

#### PR-1: Backend Role System & Middleware

**Scope:** Introduce `UserRole` type, role validation, `RequireManagementAccess` middleware, last-admin protection, data migration.

**Files:**
- `backend/internal/models/user.go` — `UserRole` type, constants, `IsValid()`
- `backend/internal/api/middleware/auth.go` — `RequireManagementAccess()` middleware
- `backend/internal/api/routes/routes.go` — apply middleware, data migration in AutoMigrate
- `backend/internal/api/handlers/user_handler.go` — role validation, self-demotion protection, last-admin check
- `backend/internal/api/handlers/permission_helpers.go` — use `UserRole` constants
- `backend/internal/models/user_test.go` — tests for new role logic
- Handler test files — updated for new validation

**Dependencies:** None (standalone backend change).

**Validation gate:** All Go unit tests pass. Manual verification: `POST /users/invite` with `role: "passthrough"` succeeds; pass-through user cannot access `GET /users`.

**Rollback:** Revert PR. No schema changes to undo.

#### PR-2a: Frontend Structural Changes

**Scope:** File merges, deletions, route restructuring, navigation consolidation. No new behavioral components.

**Files:**
- `frontend/src/pages/Account.tsx` — **deleted**
- `frontend/src/api/user.ts` — **deleted** (merged into `users.ts`)
- `frontend/src/api/users.ts` — merged functions + updated types (add `'passthrough'` to role type)
- `frontend/src/App.tsx` — updated routes + redirects for old paths
- `frontend/src/components/Layout.tsx` — single "Users" nav entry, role-based nav hiding
- `frontend/src/pages/Settings.tsx` — replace "Account" tab with "Users"
- `frontend/src/context/AuthContextValue.ts` — updated User role type
- `frontend/src/locales/*/translation.json` — key renames, removals, additions across ALL locales
- Frontend unit test files — updated for removed components

**Dependencies:** PR-1 must be merged first (backend must accept `"passthrough"` role).

**Validation gate:** Frontend unit tests pass. Old routes redirect correctly. Navigation shows single "Users" entry. No regressions in existing functionality.

**Rollback:** Revert PR. Old routes and Account page restored.

#### PR-2b: Frontend Behavioral Changes

**Scope:** New components and behavioral features — detail modal, profile section, pass-through landing, RequireRole guard.

**Files:**
- `frontend/src/components/RequireRole.tsx` — **new** (with role-aware redirect logic)
- `frontend/src/pages/PassthroughLanding.tsx` — **new** (routed at `/passthrough`)
- `frontend/src/pages/UsersPage.tsx` — add `UserDetailModal`, "My Profile" section, pass-through badge, role selector with descriptions
- `frontend/src/App.tsx` — add `/passthrough` route, wrap admin routes with `RequireRole`
- Frontend unit test files — tests for new components

**Dependencies:** PR-2a must be merged first.

**Validation gate:** Frontend unit tests pass. Pass-through user redirected to `/passthrough` (not `/`). Admin-only routes blocked for non-admin roles. Detail modal opens/closes correctly with focus trap.

**Rollback:** Revert PR. Structural changes from PR-2a remain intact.

#### PR-3: E2E Tests

**Scope:** Playwright E2E tests for the consolidated user management flow.

**Files:**
- `tests/users.spec.ts` — **new**: user list, invite flow, role assignment, permissions, inline editing
- `tests/user-invite.spec.ts` — **new**: invite → accept → login flow for all roles
- `tests/passthrough.spec.ts` — **new**: pass-through login → landing → no management access

**Dependencies:** PR-1, PR-2a, and PR-2b must be merged.

**Validation gate:** All E2E tests pass on Firefox, Chromium, and WebKit.

**Rollback:** Revert PR (test-only, no production impact).

### 6.3 Contingency Notes

- If PR-1 causes unexpected JWT issues in production, the `UserRole` type is a string alias — reverting just means removing the type and going back to raw strings. No data loss.
- PR-2 is **committed to a split** into PR-2a and PR-2b (see section 6.2 for details).
- The top-level `/users` route currently duplicates `/settings/account-management`. The redirect in PR-2a handles this, but if external tools link to `/users`, the redirect ensures continuity.

---

## 7. Complexity Estimates

| Component | Estimate | Rationale |
|-----------|----------|-----------|
| PR-1: Backend role system | **Medium** | New type + middleware + validation + transaction-based protection + session invalidation. Well-scoped. ~10 files. |
| PR-2a: Frontend structural | **Medium** | File merges/deletes, route restructuring, nav consolidation. ~10 files. |
| PR-2b: Frontend behavioral | **Medium** | New components (RequireRole, PassthroughLanding, UserDetailModal), role-aware routing. ~8 files. |
| PR-3: E2E tests | **Medium** | 3 new test files, requires running Docker environment. |
| **Total** | **Large** | Touches backend models, middleware, handlers, frontend pages, components, navigation, API client, translations, and tests. Split across 4 PRs for safer review. |
