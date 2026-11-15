---
title: Trusted Proxies
description: Tell Charon which reverse proxies to trust so it can see your visitors' real addresses.
---

# Trusted Proxies

If you reach Charon through another reverse proxy, a tunnel, or a load balancer, Charon sees that proxy's address instead of your visitors' addresses. This page shows how to tell Charon which proxies to trust.

Charon has two front doors, and each has its own trust list:

| Door | What it serves | Where you set trust |
| --- | --- | --- |
| Ports 80 and 443 | Your websites (proxy hosts) | The **Trusted proxies** card on the **Security** page |
| Charon's own port (usually 8080) | The Charon page you are reading now, and its API | The `CHARON_TRUSTED_PROXIES` setting |

## Why It Matters

A proxy passes the real visitor address along in a note attached to each request (the `X-Forwarded-For` header). Charon reads that note only from proxies you trust, and ignores a note that arrives from anywhere else.

Charon uses one visitor address for every check: access lists, IP lists, country rules, rate limits, CrowdSec, the WAF, the `X-Real-IP` header sent to your apps, and the security log. To find it, Charon starts at the connection it received and reads the note from right to left. It stops at the first address that is not a trusted proxy. With no note, or with a visitor who connects directly, Charon uses the address of the connection itself.

If you don't list your proxy:

- Everyone appears to come from the proxy's address.
- Access lists and rate limits treat all visitors as one.
- [Login Protection](../features/login-protection.md) gives all of them one shared allowance, so a few tries can make everyone wait.
- CrowdSec and the WAF judge the proxy instead of the visitor.
- Charon can't tell whether the connection was HTTPS.

If you connect to Charon directly, with no proxy in front, leave the list empty.

## Your Websites: The Trusted Proxies Card

Administrators set this on the **Security** page, in the card called **Trusted proxies**. Other users see a note that the card is hidden for their role.

By default Charon trusts only loopback, which means the same machine (`127.0.0.1` and `::1`). Loopback is always trusted, and you can't remove it.

### Set the List

1. Sign in as an administrator and open **Security**.
2. In **Trusted proxies**, type one address or range per line, for example `172.18.0.5`.
3. Click **Save**.

Charon checks each line, saves the list, and applies it straight away. You don't need to restart. If the new list can't be applied, Charon puts the previous list back and shows an error.

To remove your saved list, click **Reset to default**. Charon then uses `CHARON_TRUSTED_PROXIES` if you set it, or loopback only if you didn't.

### What the Card Accepts

| Rule | What happens |
| --- | --- |
| One address, such as `172.18.0.5` or `fd00::5` | Accepted. Charon treats it as a single address |
| A range, such as `172.18.0.0/16` | Accepted, if it is no wider than `/8` for IPv4 or `/24` for IPv6 |
| A range wider than that, including `0.0.0.0/0` or `::/0` | Refused. Charon only accepts ranges small enough to name specific proxies |
| A whole private network, such as `10.0.0.0/8` or `192.168.0.0/16` | Accepted with a warning. Every device in that range can then vouch for visitors |
| More than 64 entries | Refused |
| Anything that is not an address or range | Refused, and the card names the entry |

Charon also tidies the list: it removes duplicates and sorts the entries.

> **Only list your proxies.** Entries such as `0.0.0.0/0` or `::/0` would cover the whole internet, so Charon refuses them.

### Which List Is in Use

The card shows where the list in use comes from:

- **Using the list saved here.** You saved a list on this card.
- **Using CHARON_TRUSTED_PROXIES from the environment until you save a list here.** You haven't saved a list, but you set the environment setting.
- **Using the default, loopback only, until you save a list here.** Neither applies.

When Charon starts with `CHARON_TRUSTED_PROXIES`, it ignores each entry that is invalid or too wide, keeps the rest, and writes one warning to its log that names the ignored entries. The card lists them too.

## Charon's Own Port: CHARON_TRUSTED_PROXIES

Charon's own port reads its trust list from the `CHARON_TRUSTED_PROXIES` setting in your Docker Compose file. Add the proxy's address, then restart Charon:

```yaml
environment:
  - CHARON_TRUSTED_PROXIES=172.18.0.5/32
```

Separate several entries with commas.

Charon checks this list when it starts. If any single entry is not a valid address, Charon **ignores the whole list** and writes a warning to its log that names the entry. Fix the entry and restart. The card on the Security page shows this list read-only, as "Charon's own port".

Until you save a list on the Security page, this setting also applies to your websites, with the same clamping as above. Once you save a list on the card, the saved list replaces it for your websites. The setting still applies to Charon's own port.

### Use Exact Addresses

List the proxy's own address, not a whole network.

| Address type | Write it as | Example |
| --- | --- | --- |
| One IPv4 address | address plus `/32` (or just the address) | `172.18.0.5/32` |
| One IPv6 address | address plus `/128` (or just the address) | `fd00::5/128` |

Avoid ranges such as `10.0.0.0/8` or `192.168.0.0/16`. They trust every device in the range, so any of them could fake a visitor address.

## Common Setups

| Your setup | Add this |
| --- | --- |
| A proxy host inside Charon that points to `localhost:8080` (or `127.0.0.1:8080`) | Nothing. Charon recognises this automatically (see below) |
| A proxy host inside Charon that points to `charon:8080` | Charon's own address on the Docker network |
| External nginx or Traefik in the same Docker network | That container's address on the Docker network |
| A proxy running on the host itself | The address Charon sees it connect from (see below) |
| A CDN, such as Cloudflare | The published address ranges of that CDN, copied onto the card. CDN ranges change, so check them now and then |
| A tunnel or sidecar on the same machine, such as cloudflared or gluetun | Nothing. It connects over loopback, which is always trusted (see below) |
| Docker's gateway address (for example `172.17.0.1`) | **Do not add it.** Visitors reaching Charon through the gateway are not sending notes through a proxy. Trusting the gateway would let a visitor's own note be believed |

### Find Your Proxy's Address

You can find the address Charon sees your proxy connect from in either of these ways:

- On the **Security** page, read the **Login Protection** card. It shows the address Charon sees you connecting from, and warns you when it ignores forwarded addresses from an untrusted proxy. It names the address to add.
- Run `docker network inspect your-network-name` on the server. Under `Containers`, find your proxy's container and copy its `IPv4Address`, without the `/16` (or similar) part.

### Proxy Hosts That Point Back at Charon

If you create a proxy host whose destination is Charon itself (for example `localhost:8080` or `127.0.0.1:8080`, on Charon's own port), Charon spots this on its own. It passes along the real visitor address that the proxy saw. You don't need to add anything for this case.

Destinations reached by a Docker network name, such as `charon:8080`, are not recognised automatically. Keep listing those in the trust list.

If another proxy terminates HTTPS in front of Charon and is not trusted, Charon sees plain HTTP. It may then not mark the sign-in cookie `Secure`. Trust that proxy so Charon uses the original address and scheme.

### Same-Machine Tunnels

A tunnel or sidecar that runs on the same machine as Charon, such as cloudflared, gluetun, or `tailscale serve`, connects to Charon over loopback. Loopback is always trusted, so Charon reads the visitor's address from the note the tunnel adds. Make sure the tunnel adds that note.

Because Charon now sees the visitor instead of `127.0.0.1`:

- An access list limited to the local network refuses internet visitors. That is the point of the list, but check it if your sites worked by accident before.
- The same applies to Charon's own interface when you reach it through the tunnel. Its management access lists judge the visitor.
- Rate limits and country rules judge the visitor.

Reset the emergency server through a same-machine tunnel and the visitor's address is judged too. A reset sent directly to the local emergency port is not affected. See [Emergency Break Glass Protocol](emergency-setup.md).

### Keep the Address from Changing

Docker can hand out a new address when a container is recreated. Give your proxy a fixed address (a static IP in your Compose network), or put it in its own small network, so the entry stays correct.

## Your Proxy Must Pass the Visitor Address Along

Your proxy has to add the visitor to `X-Forwarded-For`, or replace the header with the visitor's address. For nginx:

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

If your proxy only sets `X-Real-IP` and passes the visitor's own `X-Forwarded-For` through untouched, visitors can choose their own address. See [WebSocket troubleshooting](../troubleshooting/websocket.md) for a full nginx example.

## Live Views Behind a Proxy That Rewrites Host

Some proxies, such as nginx with `proxy_set_header Host $proxy_host`, replace the web address your browser used. Charon then refuses live views (the live logs and live stats) with a 403 error, because it can't match them to the page you are on. Changes you make may also show 403.

Fix it in either of these ways:

- Set the [Application URL](public-url.md) to the address you type in your browser.
- Add your proxy's address to `CHARON_TRUSTED_PROXIES`, so Charon accepts the address the proxy passes along.

## Mistakes in the List

- **On the Security card**, Charon checks each entry before saving. An invalid, too-wide, or surplus entry is refused, nothing is saved, and the message names the entry.
- **In `CHARON_TRUSTED_PROXIES`** for Charon's own port, one invalid entry makes Charon ignore the whole list. A warning in the log names the entry.
- **In `CHARON_TRUSTED_PROXIES`** for your websites, Charon ignores only the entries that are invalid or too wide, keeps the rest, and logs a warning that names them.
- **A saved list that no longer checks out**, for example one edited by hand in the database, makes Charon log a warning and trust loopback only until you save the list again.

## Confirm It's Working

1. Sign in as an administrator and open the **Security** page.
2. On the **Login Protection** card, read "Charon sees your browser as".
3. If it shows your device's real address, you're done.
4. If it shows your proxy's address or an internal one, the proxy isn't trusted yet. Add it to the **Trusted proxies** card, then check again.

If the address is an internal one and no warning appears, your container setup may be hiding visitor addresses. See [Login Protection](../features/login-protection.md#situation-2-your-container-setup-hides-visitor-addresses).

## Related

- [Login Protection](../features/login-protection.md)
- [Application URL](public-url.md): set it when Charon sits behind another proxy.
- [Access Control Lists](../features/access-control.md), including the management access lists for Charon's own interface.
- [Troubleshooting proxy headers](../troubleshooting/proxy-headers.md)
