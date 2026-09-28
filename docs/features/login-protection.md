---
title: Login Protection
description: How Charon slows down repeated sign-in attempts, and what to do if everyone sees a "please wait" message.
---

# Login Protection

Charon slows down anyone who tries to sign in too many times, too fast. It is on by default. You don't need to set anything up.

## What It Does

Think of a bouncer who lets each visitor knock on the door a limited number of times per few minutes. Knock too often, and the bouncer says "please wait a moment." Everyone else can still knock as normal.

- Each visitor (each device address) gets its own allowance.
- Checking a password is the expensive part, so those requests get the strictest allowance.
- Charon also locks an account for 15 minutes after 5 wrong passwords. Login protection works alongside that lock.
- Emergency recovery access is never slowed down. See [Emergency Access](../configuration/emergency-setup.md).

## What You'll See

If someone signs in too many times, the login page shows:

> Too many attempts. Please wait 45 seconds and try again.

Wait for the time shown, then try again. That's it.

## Defaults

| Action | Allowance |
| --- | --- |
| Sign in, change your password, change your email, export a certificate key | 10 tries, then about 1 more per minute |
| Refreshing your session | 60 per minute |

Loading pages while already signed in is never counted.

## Changing the Settings

Most people should leave these alone. To change them, add these to your Docker Compose file and restart Charon.

| Setting | Default | Allowed values | What it does |
| --- | --- | --- | --- |
| `CHARON_AUTH_RATELIMIT_ENABLED` | `true` | `true` or `false` | Set to `false` to turn login protection off |
| `CHARON_AUTH_RATELIMIT_LOGIN_REQUESTS` | `10` | 1 to 10,000 | Sign-in tries allowed in a burst |
| `CHARON_AUTH_RATELIMIT_LOGIN_WINDOW` | `600` | 1 to 86,400 (seconds) | The time over which those tries refill |
| `CHARON_AUTH_RATELIMIT_SESSION_REQUESTS` | `60` | 1 to 10,000 | Session refreshes allowed in a burst |
| `CHARON_AUTH_RATELIMIT_SESSION_WINDOW` | `60` | 1 to 86,400 (seconds) | The time over which those refills happen |

If you type a value Charon can't use, it falls back to the default and writes a warning to its log. Only `false` turns protection off.

## Checking That It's Working

Sign in as an administrator and open the **Security** page. The **Login Protection** card shows whether it is on, the current allowances, and the address Charon sees for your browser.

## "Everyone Sees the Please-Wait Message"

Login protection counts each visitor by address. If Charon sees the same address for everyone, they all share one allowance, so a few tries from one person can make everyone wait.

That happens in two situations.

### Situation 1: A reverse proxy sits in front of Charon

If you reach Charon through another proxy (nginx, Traefik, a load balancer, or a proxy host you created inside Charon that points back at Charon), Charon sees the proxy's address for everyone. Tell Charon which proxy to trust so it can read the real visitor address.

The Security page card shows a warning when it spots this. Follow [Trusted Proxies](../configuration/trusted-proxies.md).

### Situation 2: Your container setup hides visitor addresses

Some setups replace every visitor's address with an internal one, and add no hint about who they were. Trusting a proxy can't fix this, because there is nothing to read.

To check, open the Security page. If "Charon sees your browser as" shows an address that isn't your device's, this is likely happening.

Ways to fix it:

1. **Rootless Docker.** Docker's own documentation says port forwarding with `docker run -p` does not pass along visitor addresses by default. With RootlessKit 3.0 or newer, its fix is to set `"userland-proxy": false` in `~/.config/docker/daemon.json` and load the `br_netfilter` kernel module. With older versions, it suggests the `slirp4netns` port driver, or `pasta` with the `implicit` port driver (Docker Engine 25.0 or newer, marked experimental). Details: [Docker rootless troubleshooting](https://docs.docker.com/engine/security/rootless/troubleshoot/) and the [RootlessKit port driver table](https://github.com/rootless-containers/rootlesskit/blob/master/docs/port.md).
2. **Rootless Podman.** Podman's networking guide says the default forwarder for rootless bridge networks does not keep visitor addresses, and describes setting `rootless_port_forwarder="pasta"` to keep them. See the [Podman networking tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/basic_networking.md). This setting depends on your Podman version, so check the guide for yours.
3. **Other setups** (desktop container apps, load balancers that rewrite addresses). Behavior varies by product and version. Check your product's documentation for keeping the original visitor address.
4. **Put a proxy on the host in front of Charon.** A proxy running directly on the host sees real addresses. Then follow [Trusted Proxies](../configuration/trusted-proxies.md).
5. **Last resort:** set `CHARON_AUTH_RATELIMIT_ENABLED=false`. Charon logs a warning at startup. You lose this protection, so use it only if the options above are impossible.

If you are locked out right now, restarting Charon clears all the waiting counters.

The upstream pages above change over time. If a step doesn't match what you see, trust the upstream page for your version.

## Related

- [Trusted Proxies](../configuration/trusted-proxies.md)
- [How Charon Keeps You Safe](security.md)
- [Troubleshooting proxy headers](../troubleshooting/proxy-headers.md)
