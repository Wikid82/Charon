---
title: Trusted Proxies
description: Tell Charon which reverse proxies to trust so it can see your visitors' real addresses.
---

# Trusted Proxies

If you reach Charon through another reverse proxy, Charon only sees that proxy's address unless you tell it to trust the proxy. This page shows how.

## Why It Matters

A proxy passes along the real visitor address in a note attached to each request (the `X-Forwarded-For` header). Charon only reads that note from proxies you list. Otherwise anyone could write a fake note.

If you don't list your proxy:

- Everyone appears to come from the proxy's address.
- [Login Protection](../features/login-protection.md) gives all of them one shared allowance, so a few tries can make everyone wait.
- Charon can't tell whether the connection was HTTPS.

If you connect to Charon directly, with no proxy in front, leave this empty.

## How to Set It

Add the proxy's address to your Docker Compose file, then restart Charon:

```yaml
environment:
  - CHARON_TRUSTED_PROXIES=172.18.0.5/32
```

Separate several entries with commas.

### Use exact addresses

List the proxy's own address, not a whole network.

| Address type | Write it as | Example |
| --- | --- | --- |
| One IPv4 address | address plus `/32` (or just the address) | `172.18.0.5/32` |
| One IPv6 address | address plus `/128` (or just the address) | `fd00::5/128` |

Avoid ranges such as `10.0.0.0/8` or `192.168.0.0/16`. They trust every device in the range, so any of them could fake a visitor address.

> **Never trust everyone.** Entries such as `0.0.0.0/0` or `::/0` trust the whole internet. Anyone could pretend to be any visitor. Charon logs a warning if you do this.

## Common Setups

| Your setup | Add this |
| --- | --- |
| A proxy host inside Charon that points to `localhost:8080` | `127.0.0.1/32` and `::1/128` (list both) |
| A proxy host inside Charon that points to `charon:8080` | Charon's own address on the Docker network |
| External nginx or Traefik in the same Docker network | That container's address on the Docker network |
| A proxy running on the host itself | The address Charon sees it connect from. Find it on the Security page (see below) |

Loopback has two forms. `127.0.0.1` and `::1` are treated as different addresses, so list both if you aren't sure which your proxy uses.

### Keep the address from changing

Docker can hand out a new address when a container is recreated. Give your proxy a fixed address (a static IP in your Compose network), or put it in its own small network, so the entry stays correct.

## Your Proxy Must Pass the Visitor Address Along

Your proxy has to add the visitor to `X-Forwarded-For`, or replace the header with the visitor's address. For nginx:

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

If your proxy only sets `X-Real-IP` and passes the visitor's own `X-Forwarded-For` through untouched, visitors can choose their own address. See [WebSocket troubleshooting](../troubleshooting/websocket.md) for a full nginx example.

## Mistakes in the List

Charon checks the list when it starts. If any single entry is not a valid address, Charon **ignores the whole list** and writes a warning to its log naming the entry. Fix the entry and restart.

Before this was checked, one typo only affected part of Charon's behavior. Now it affects all of it, so a typo is easy to notice.

## Confirm It's Working

1. Sign in as an administrator and open the **Security** page.
2. On the **Login Protection** card, read "Charon sees your browser as".
3. If it shows your device's real address, you're done.
4. If it shows your proxy's address or an internal one, the proxy isn't trusted yet. The card also warns you when it sees ignored forwarded addresses, and names the proxy address to add.

If the address is an internal one and no warning appears, your container setup may be hiding visitor addresses. See [Login Protection](../features/login-protection.md#situation-2-your-container-setup-hides-visitor-addresses).

## Related

- [Login Protection](../features/login-protection.md)
- [Troubleshooting proxy headers](../troubleshooting/proxy-headers.md)
