---
title: Application URL
description: Tell Charon the address you type into your browser, so changes made through a proxy are accepted.
---

# Application URL

The **Application URL** is the web address you type into your browser to open Charon, such as `https://charon.example.com`. Set it when Charon sits behind another proxy.

## What It Does

When you change something in Charon (for example, saving a proxy host), Charon checks that the change came from its own web page and not from some other website. It compares the address your browser used with the addresses it trusts.

Charon always trusts the address it can see on its own. If another proxy in front of Charon changes that address on the way in, the two no longer match, and Charon may refuse your changes. The Application URL fixes this by giving Charon the address your browser really uses.

## When You Need It

Set it when **Charon sits behind another proxy**, such as nginx, Traefik, Caddy, or a router's built-in proxy, and that proxy changes the address on the way in.

You can skip it if you open Charon directly, for example at `http://192.168.1.10:8080`, with no other proxy in front.

## How to Set It

1. Sign in as an administrator.
2. Click the **gear icon** in the sidebar to open **System Settings**.
3. Scroll to the **Application URL** section.
4. Type the address you use in your browser, starting with `http://` or `https://`. Leave out any path such as `/admin`.
5. Click **Validate** to check the format.
6. Click **Save Changes**.

Example: if you type `https://charon.example.com` into your browser, enter exactly that.

**Results:**

- Changing the Application URL takes effect **immediately**. You don't need to restart Charon.
- If you restore a backup, the Application URL inside that backup takes effect **after the restart** that applies the restore.

## Troubleshooting

### Changes fail with 403 behind a proxy

**What you see:** Saving a change, such as a new proxy host, shows an error or a "403 Forbidden" message. Reading pages still works.

**Why it happens:** Your proxy is passing on a different address than the one your browser uses, so Charon doesn't recognize the request as coming from its own page.

**How to fix it:**

1. Open System Settings and go to the **Application URL** section.
2. Enter the exact address shown in your browser's address bar, including `https://` or `http://`.
3. Click **Validate**, then **Save Changes**.
4. Try your change again. No restart is needed.

If the error continues after a restored backup, restart Charon once so the backup's Application URL takes effect.

## Related

- [Trusted Proxies](trusted-proxies.md) — tell Charon which proxies to trust so it sees your visitors' real addresses.
- [Troubleshooting proxy headers](../troubleshooting/proxy-headers.md)
