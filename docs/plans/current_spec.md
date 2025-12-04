## 📋 Plan: Security Hardening, User Gateway & Identity

### 🧐 UX & Context Analysis

This plan expands on the initial security hardening to include a full **Identity Provider (IdP)** feature set. This allows Charon to manage users, invite them via email, and let them log in using external providers (SSO), while providing seamless access to downstream apps.

#### 1. The User Gateway (Forward Auth)
*   **Scenario:** Admin shares `jellyseerr.example.com` with a friend.
*   **Flow:**
    1.  Friend visits `jellyseerr.example.com`.
    2.  Redirected to Charon Login.
    3.  Logs in via **Plex / Google / GitHub** OR Local Account.
    4.  Charon verifies access.
    5.  Charon redirects back to Jellyseerr, injecting `X-Forwarded-User: friend@email.com`.
    6.  **Magic:** Jellyseerr (configured for header auth) sees the header and logs the friend in automatically. **No second login.**

#### 2. User Onboarding (SMTP & Invites)
*   **Problem:** Admin shouldn't set passwords manually.
*   **Solution:** Admin enters email -> Charon sends Invite Link -> User clicks link -> User sets Password & Name.

#### 3. User-Centric Permissions (Allow/Block Lists)
*   **Concept:** Instead of managing groups, Admin manages permissions *per user*.
*   **UX:**
    *   Go to **Users** -> Edit User -> **Permissions** Tab.
    *   **Mode:** Toggle between **"Allow All (Blacklist)"** or **"Deny All (Whitelist)"**.
    *   **Exceptions:** Multi-select list of Proxy Hosts.
    *   *Example:* Set Mode to "Deny All", select "Jellyseerr". User can ONLY access Jellyseerr.
    *   *Example:* Set Mode to "Allow All", select "Home Assistant". User can access everything EXCEPT Home Assistant.

### 🤝 Handoff Contract (The Truth)

#### 1. Auth Verification (Internal API for Caddy)
*   **Endpoint:** `GET /api/auth/verify`
*   **Response Headers:**
    *   `X-Forwarded-User`: The user's email or username.
    *   `X-Forwarded-Groups`: (Future) User roles/groups.

#### 2. SMTP Configuration
```json
// POST /api/settings/smtp
{
  "host": "smtp.gmail.com",
  "port": 587,
  "username": "admin@example.com",
  "password": "app-password",
  "from_address": "Charon <no-reply@example.com>",
  "encryption": "starttls" // none, ssl, starttls
}
```

#### 3. User Permissions
```json
// POST /api/users
{
  "email": "friend@example.com",
  "role": "user",
  "permission_mode": "deny_all", // or "allow_all"
  "permitted_hosts": [1, 4, 5] // List of ProxyHost IDs to treat as exceptions
}
```

### 🏗️ Phase 1: Security Hardening (Quick Wins)
1.  **Secure Headers:** `Content-Security-Policy`, `Strict-Transport-Security`, `X-Frame-Options`.
2.  **Cookie Security:** `HttpOnly`, `Secure`, `SameSite=Strict`.

### 🏗️ Phase 2: Backend Core (User & SMTP)
1.  **Models:**
    *   `User`: Add `InviteToken`, `InviteExpires`, `PermissionMode` (string), `Permissions` (Many-to-Many with ProxyHost).
    *   `ProxyHost`: Add `ForwardAuthEnabled` (bool).
    *   `Setting`: Add keys for `smtp_host`, `smtp_port`, etc.
2.  **Logic:**
    *   `internal/services/mail`: Implement SMTP sender.
    *   `internal/api/handlers/user.go`: Add `InviteUser` handler and Permission logic.

### 🏗️ Phase 3: SSO Implementation
1.  **Library:** Use `github.com/markbates/goth` or `golang.org/x/oauth2`.
2.  **Models:** `SocialAccount` (UserID, Provider, ProviderID, Email).
3.  **Routes:**
    *   `GET /auth/:provider`: Start OAuth flow.
    *   `GET /auth/:provider/callback`: Handle return, create/link user, set session.

### 🏗️ Phase 4: Forward Auth Integration
1.  **Caddy:** Configure `forward_auth` directive to point to Charon API.
2.  **Logic:** `VerifyAccess` handler:
    *   Check if User is logged in.
    *   Fetch User's `PermissionMode` and `Permissions`.
    *   If `allow_all`: Grant access UNLESS host is in `Permissions`.
    *   If `deny_all`: Deny access UNLESS host is in `Permissions`.

### 🎨 Phase 5: Frontend Implementation
1.  **Settings:** New "SMTP" and "SSO" tabs in Settings page.
2.  **User List:** "Invite User" button.
3.  **User Edit:** New "Permissions" tab with "Allow/Block" toggle and Host selector.
4.  **Login Page:** Add "Sign in with Google/Plex/GitHub" buttons.

### 📚 Phase 6: Documentation
1.  **SSO Guides:** How to get Client IDs from Google/GitHub.
2.  **Header Auth:** Guide on configuring Jellyseerr/Grafana to trust Charon.
