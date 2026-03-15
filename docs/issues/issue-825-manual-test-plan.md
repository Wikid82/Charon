---
title: "Manual Test Plan - Issue #825: HTTP Login from Private Network IPs"
status: Open
priority: High
assignee: QA
labels: testing, backend, frontend, security
---

# Test Objective

Confirm that users can log in to Charon over HTTP when accessing from a private network IP (e.g., `192.168.x.x:8080`), and that HTTPS and localhost login paths remain unaffected.

# Scope

- In scope: Auth cookie behavior across HTTP/HTTPS, private IPs, localhost, and port remapping. Session persistence and the Bearer token fallback.
- Out of scope: Setup wizard UI styling, non-auth API endpoints, Caddy proxy configuration changes.

# Prerequisites

- Fresh Charon instance (no existing users in the database).
- Access to a machine on the same LAN (or the host machine itself using its LAN IP).
- A second device or browser for cross-tab and LAN IP tests.
- For HTTPS scenarios: a reverse proxy (Caddy or nginx) with TLS termination in front of Charon.
- Browser DevTools available to inspect cookies and network requests.

# Manual Scenarios

## 1) Fresh install login over HTTP from LAN IP (original bug)

- [ ] Deploy a fresh Charon instance with an empty database.
- [ ] From another machine on the LAN, open `http://<LAN-IP>:8080` (e.g., `http://192.168.1.50:8080`).
- [ ] Complete the setup wizard and create an admin user.
- [ ] Log in with the credentials you just created.
- [ ] **Expected**: Login succeeds. You are redirected to the dashboard. No "unauthorized" flash or redirect back to login.
- [ ] Open DevTools > Network tab. Confirm `/api/auth/me` returns `200`.
- [ ] Open DevTools > Application > Cookies. Confirm the auth cookie has `Secure: false`.

## 2) Login over HTTPS via reverse proxy

- [ ] Configure a reverse proxy (Caddy or nginx) with TLS termination pointing to Charon.
- [ ] Open `https://<your-domain-or-ip>` in the browser.
- [ ] Log in with valid credentials.
- [ ] **Expected**: Login succeeds. Redirected to the dashboard.
- [ ] Open DevTools > Application > Cookies. Confirm the auth cookie has `Secure: true`.

## 3) Login from localhost over HTTP (regression check)

- [ ] On the machine running Charon, open `http://localhost:8080`.
- [ ] Log in with valid credentials.
- [ ] **Expected**: Login succeeds. This is existing behavior and must not regress.

## 4) Session persistence after page refresh

- [ ] Log in successfully via any access method.
- [ ] Press F5 (hard refresh).
- [ ] **Expected**: You remain logged in. The dashboard loads without a redirect to the login page.

## 5) Multiple browser tabs

- [ ] Log in on Tab 1.
- [ ] Open a new Tab 2 and navigate to the same Charon URL.
- [ ] **Expected**: Tab 2 loads the dashboard immediately without prompting for login.

## 6) Logout and re-login

- [ ] Log in, then click Logout.
- [ ] **Expected**: Redirected to the login page.
- [ ] Log in again with the same credentials.
- [ ] **Expected**: Login succeeds with a clean session. No stale session errors.

## 7) Port remapping scenario

- [ ] Run Charon with Docker port mappings: `-p 82:80 -p 445:443 -p 8080:8080`.
- [ ] Access Charon on `http://<LAN-IP>:8080`.
- [ ] Log in with valid credentials.
- [ ] **Expected**: Login succeeds on the remapped port.
- [ ] If a reverse proxy is available, repeat via `https://<LAN-IP>:445`.
- [ ] **Expected**: Login succeeds with `Secure: true` on the cookie.

# Expected Results

- Login over HTTP from a private network IP works on fresh install.
- Cookie `Secure` flag is `false` for HTTP and `true` for HTTPS.
- Localhost login remains functional.
- Sessions survive page refreshes and work across tabs.
- Logout cleanly destroys the session, and re-login creates a new one.
- Port remapping does not break authentication.

# Regression Checks

- [ ] Confirm no changes to login behavior when accessing via `localhost` or `127.0.0.1`.
- [ ] Confirm HTTPS connections still set `Secure: true` on the auth cookie.
- [ ] Confirm the Bearer token header fallback does not interfere when cookies are working normally.
