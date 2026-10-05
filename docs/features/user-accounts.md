---
title: User Accounts & Roles
description: How people sign in to Charon, who is allowed to change what, and how new accounts get added
category: features
---

# User Accounts & Roles

Charon keeps a short, simple list of who is allowed to sign in. There is no public
"create an account" page — every account is set up on purpose by someone who
already has access. That means a stranger who finds your Charon login screen has
nothing to do there but look at it.

---

## The Two Kinds of Account

**Administrator**

The full-control account. An admin can change anything: security settings
(the firewall, CrowdSec, access lists, security headers), certificates and the
credentials used to get them, DNS provider logins, SSH connection details for
remote servers, tunnels and agents, app settings, plugins, and the user list
itself. The very first account you create when you set Charon up is an
administrator.

**Standard user**

A day-to-day account for adding and managing proxy hosts. A standard user can
still *open* most settings pages and see how things are configured — the
certificates list, the access lists, the DNS providers, and so on — but the
buttons that change security-sensitive configuration are for admins only. If a
standard user tries one anyway, Charon simply refuses it.

A few areas are admin-only even to look at, because they show sensitive detail:
the CrowdSec control panel, the audit log, the remote agent management page, and
encryption management. Standard users don't see those in the menu.

---

## Adding Someone New

New accounts are always created by an existing administrator, in
**Settings → Users**. There are two ways:

1. **Invite by email** — Charon emails the person a one-time link. They click it,
   pick their own password, and they're in.
2. **Create directly** — the admin fills in the name, email, and a starting
   password and hands it over.

Either way, the admin chooses whether the new person is an administrator or a
standard user.

---

## Passwords and Sign-In

- **Same message every time.** If a sign-in doesn't work, the login page shows
  the same "invalid credentials" message, whatever the reason. The sign-in
  screen doesn't show an account's lock status.
- **Locks.** After 5 wrong passwords an account is locked for 15 minutes. Once
  the lock has expired, the count of wrong tries starts again from zero.
- **Changing your password signs out your other devices.** Use
  **Change Password** and Charon ends your other sessions, while you stay signed
  in on the device you used.
- **Resetting someone else's password.** An administrator can set a new
  password for another person without knowing the old one. That person is
  signed out everywhere and signs in with the new password. Changing your own
  password through the Users API needs your current password, and signs you out
  so you sign in again.

---

## The Very First Account

The first time you open a brand-new Charon, it shows a one-time setup screen and
asks you to create the starting administrator account. That's the only time an
account is created without an existing admin doing it. Once that account exists,
the setup screen is gone for good and all further accounts go through
**Settings → Users**.

---

## Related

- [How Charon Keeps You Safe](./security.md) — the bigger security picture
- [Access Control Lists](./access-control.md) — decide who can reach your proxied sites
- [Back to Features](../features.md)
