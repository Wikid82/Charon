---
title: Access Control Lists (ACLs)
description: Define exactly who can access what with fine-grained rules
---

# Access Control Lists (ACLs)

Define exactly who can access what. Block specific countries, allow only certain IP ranges, or require authentication for sensitive applications. Fine-grained rules give you complete control.

## Overview

Access Control Lists let you create granular rules that determine who can reach your proxied services. Rules are evaluated in order, and the first matching rule determines whether access is allowed or denied.

ACL capabilities:

- **IP Allowlists** — Only permit specific IPs or ranges
- **IP Blocklists** — Deny access from known bad actors
- **Country/Geo Blocking** — Restrict access by geographic location
- **CIDR Support** — Define rules using network ranges (e.g., `192.168.1.0/24`)

## Why Use This

- **Compliance** — Restrict access to specific regions for data sovereignty
- **Security** — Block high-risk countries or known malicious networks
- **Internal Services** — Limit access to corporate IP ranges
- **Layered Defense** — Combine with WAF and CrowdSec for comprehensive protection

## Configuration

### Creating an Access List

1. Navigate to **Access Lists** in the sidebar
2. Click **Add Access List**
3. Provide a descriptive name (e.g., "Office IPs Only")
4. Configure your rules

### Rule Types

#### IP Range Filtering

Add specific IPs or CIDR ranges:

```text
Allow: 192.168.1.0/24      # Allow entire subnet
Allow: 10.0.0.5            # Allow single IP
Deny:  0.0.0.0/0           # Deny everything else
```

Rules are processed top-to-bottom. Place more specific rules before broader ones.

#### Country/Geo Blocking

Block or allow traffic by country:

1. In the Access List editor, go to **Country Rules**
2. Select countries to **Allow** or **Deny**
3. Choose default action for unlisted countries

Common configurations:

- **Allow only your country** — Whitelist your country, deny all others
- **Block high-risk regions** — Deny specific countries, allow rest
- **Compliance zones** — Allow only EU countries for GDPR compliance

**Country lists need a country map (the GeoIP database).** Charon uses this map to find out which country each visitor is from. Charon includes one in its Docker image, so you normally do not need to do anything. If the map is ever missing or cannot be read, country lists are **not enforced**: an allow-list lets everyone in, and a block-list blocks no one. Your other lists and sites keep working. Charon shows a red alert in the notification bell (in the app only) until the map is available again. See [Country Access Lists Are Not Being Enforced](../troubleshooting/geoip-database.md).

#### The Country Database Card

Admins manage the map on the **Security** page, in the card called **Country database (GeoIP)**. Other users do not see this card.

- **Status:** **Ready** means Charon has a map it can use. **Not available** means no usable map is loaded.
- **Source:** where the map in use comes from. It says **Included with Charon**, **Your data folder** (a copy you uploaded), **Custom path set on the server**, or **No database**.
- **Build date** and **Size:** show how new the map is and how large the file is.
- **File:** the location of the file inside Charon.
- **Country lists not enforced:** appears only when at least one of your lists is affected.

The card has these buttons:

- **Reload:** Charon reads the map file again and applies your access lists. Use it after you change the file, instead of restarting Charon.
- **Replace database…:** choose a new file from your computer. Charon accepts a file only when all of these are true:
  - it ends in `.mmdb` and is no larger than 32 MiB;
  - it is a **country** database, such as GeoLite2-Country or DB-IP's country file (city and other types are refused);
  - it is as new as, or newer than, the map Charon uses now and the one included with Charon;
  - its build date is not in the future.

  If a file is refused, the card tells you why, and Charon keeps the map it had before.
- **Use the included copy:** removes your uploaded copy and goes back to the map that comes with Charon. Charon asks you to confirm first. If Charon has no included map, country lists stay unenforced until you provide one.

Results appear as a short message inside the card. **Replace database…** and **Use the included copy** are hidden when the server's map location is set by the `CHARON_GEOIP_DB_PATH` setting, or when the data folder cannot be written to. **Reload** still works in both cases.

#### Look Up an Address

Under the buttons, type an IP address (for example `203.0.113.7`) and press **Look up** to see which country it belongs to. Press **Use my address** to fill in the address you are browsing from. The lookup needs a loaded map. Without one, the lookup box is greyed out.

#### The Note Under a Country List

When you edit a country list as an admin, Charon shows a small grey note under the country fields. The note never stops you from saving.

- **"This list will not be enforced until the GeoIP database is available."** The map is missing, so this list does nothing yet.
- **"Based on the address you are using now, this list would refuse you. Saving is still allowed."** Charon checked the address you are using now against the list. Check the countries before you save, or add your address to the admin whitelist.

#### Private and Home-Network Addresses Have No Country

Addresses such as `192.168.1.20` or `10.0.0.5` belong to your local network, not to any country. A country allow-list therefore refuses them, unless the address is on your admin whitelist. A country block-list does not refuse them. The lookup says "No country found" for these addresses.

#### Adding a Map by Hand (Advanced)

If you prefer not to use the card, you can place the file yourself:

1. Get the MaxMind GeoLite2-Country database file (it ends in `.mmdb`) and name it `GeoLite2-Country.mmdb`.
2. Put it in the `geoip` folder inside your data folder. In the container that is `/app/data/geoip/`. When the image also has a copy, Charon uses whichever has the newer build date. You can also set the `CHARON_GEOIP_DB_PATH` setting to the file's full path, which takes precedence over both locations.
3. Press **Reload** on the Security page, or restart Charon. Charon then applies your country lists again.

For where to get a map and how to fix a missing one, see [Country Access Lists Are Not Being Enforced](../troubleshooting/geoip-database.md).

### Entries Required for Allow Lists

- An **enabled allow list** needs at least one entry, such as an IP address, an IP range or a country. Charon will not save an enabled allow list that is empty.
- A **block list** can be empty. An empty block list blocks nobody.

### Admin Whitelist: Your Addresses Always Get In

The admin whitelist is a list of addresses you set in **Security** settings, such as your home IP, your office IP, or a VPN or Tailscale address. Requests from these addresses are always let through, whichever access list a site uses. This is what keeps you from locking yourself out.

The admin whitelist works the same way for every list type:

- **IP allow lists** — Admin addresses get in, even if they are not on the list.
- **IP block lists** — Admin addresses are never refused, even if their address falls inside a blocked range.
- **Country lists** (allow or block) — Admin addresses get in, whatever country they are in.
- **Local network only** — Admin addresses get in, even when they are outside your local network.

A block list refuses only the addresses it lists. Everyone else gets through as usual.

**Manual blocks:** If you add a manual block for an address that is also on the admin whitelist, the block does nothing. To block that address, remove it from the admin whitelist first.

**Checking an address:** The **Test IP** tool on the Access Lists page gives the same answer the proxy gives. If it says an address is allowed, a visitor from that address is allowed too.

### Applying to Proxy Hosts

1. Edit your proxy host
2. Go to the **Access** tab
3. Select your Access List from the dropdown
4. Save changes

Each proxy host can have one Access List assigned. Create multiple lists for different access patterns.

## Rule Evaluation Order

```text
0. Check admin whitelist → Allow if matched (skips the steps below)
1. Check IP allowlist → Allow if matched
2. Check IP blocklist → Deny if matched
3. Check country rules → Allow/Deny based on geo
4. Apply default action
```

## Refused Requests

When an access list refuses a visitor, the visitor sees a 403 page (or a 429 page for rate limits). That page still includes the security headers you chose for the site. See [HTTP Security Headers](security-headers.md).

## Best Practices

| Scenario | Recommendation |
| ---------- | ---------------- |
| Internal admin panels | Allowlist office/VPN IPs only |
| Public websites | Use geo-blocking for high-risk regions |
| API endpoints | Combine IP rules with rate limiting |
| Development servers | Restrict to developer IPs |

## Related

- Proxy Hosts — Apply access lists to services (guide not yet published)
- [CrowdSec Integration](./crowdsec.md) — Automatic threat-based blocking
- [Rate Limiting](./rate-limiting.md) — Limit request frequency
- [Back to Features](../features.md)
