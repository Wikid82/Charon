---
title: Country Access Lists Are Not Being Enforced
description: What the red bell alert about the GeoIP database means, how to check it, and how to add the database yourself.
---

## Country Access Lists Are Not Being Enforced

### What You See

A red alert appears in the notification bell (the bell icon in the app) with this message:

> **Country access lists are not being enforced**

### What It Means

Country access lists (rules that allow or block visitors by their country) need a small map file that tells Charon which country an address belongs to. This file is called the **GeoIP database**. Charon normally includes it in the Docker image, so you should not see this alert.

When the file is missing or cannot be read:

- **Country lists do nothing.** An allow-list lets everyone in. A block-list blocks no one.
- **Your sites stay reachable.** Nothing is locked out.
- **Everything else still works.** IP lists, the WAF, rate limits, CrowdSec and your other protections are not affected.
- **The alert is shown in the app only.** It is not sent to Discord, email or any other place.

Think of it like a bouncer who has lost the guest list for one door. The door stays open for everyone until a new list arrives, so you should fix it soon.

### How to Check

1. Open the notification bell in Charon. If the red alert is there, the database is missing.
2. Look at the logs for a warning that starts with `Country access list is not enforced`. To see the logs, run `docker logs charon` in a terminal.

### Option 1: Upload a Database from the Security Page (Recommended)

This is the easiest way. You need an admin account in Charon.

1. Get a country database file (see [Where to Get One](#where-to-get-one) below).
2. In Charon, go to **Security** and find the **Country database (GeoIP)** card.
3. Press **Replace database…** and choose the `.mmdb` file from your computer.
4. Wait for the message inside the card. It says the database was replaced when Charon is using your file.

Charon accepts the file only when it is:

- a file ending in `.mmdb`, no larger than 32 MiB;
- a **country** database. City databases and other types are refused;
- as new as, or newer than, the database Charon uses now and the one included with Charon;
- not dated in the future.

If the file is refused, the card says why. Charon then keeps the database it had before.

**Uploading through a WAF-protected address:** If you reach Charon through one of your own sites that has the WAF (web application firewall) turned on, the WAF can refuse a large upload before Charon sees it. Its limit is about 12.5 MiB. A country database is usually smaller, but if the upload fails, open Charon by its direct address (the address you use to reach Charon's own page, not a site that goes through Charon) and try again.

**Want to go back?** Press **Use the included copy** on the same card. Charon removes your uploaded copy and uses the database that came with it.

### Option 2: Place the File Yourself

1. Get a country database file (see [Where to Get One](#where-to-get-one) below).
2. Rename it to `GeoLite2-Country.mmdb` if it has a different name.
3. Copy it into the `geoip` folder inside your data folder. In the container, that folder is `/app/data/geoip/`. On your computer, it is the `geoip` folder inside the folder you mounted as `/app/data`. Create the `geoip` folder if it does not exist.
4. In Charon, go to **Security** and press **Reload** on the card. You can also restart Charon. In a terminal, run `docker restart charon`, or restart the container from your Docker tool.

**Want to use a file in another location?** Set the `CHARON_GEOIP_DB_PATH` setting to the full path of the `.mmdb` file, then restart Charon. This path wins over the file in the `geoip` folder. The file name must end in `.mmdb`. While this setting is in use, the card cannot replace or remove the file. **Reload** still works.

### Where to Get One

Charon needs a **country** database in the MaxMind `.mmdb` format. Two free options work:

- **MaxMind GeoLite2-Country:** Create a free account at MaxMind, then download the GeoLite2-Country database. MaxMind may send it as a `.tar.gz` archive. Open the archive first, and use the `GeoLite2-Country.mmdb` file inside it.
- **DB-IP IP-to-Country Lite (MMDB format):** Download the free country file from DB-IP in the MMDB format. This file is licensed under CC BY 4.0, which requires you to give credit to DB-IP. Credit DB-IP with a link to [db-ip.com](https://db-ip.com) in any place where you publish or show your setup.

Use only the **country** edition. City and ASN databases are not country databases, and Charon refuses them.

### Confirm It Worked

1. Open **Security** and look at the **Country database (GeoIP)** card. The status should say **Ready**, and the **Source** should show where the database came from.
2. Open the notification bell. The red alert should be gone.

The alert clears by itself once Charon can read the database. You do not need to dismiss it.

### Still Seeing the Alert?

Check that the file is in the right folder, that its name ends in `.mmdb`, and that it is a country database. Then press **Reload** on the card, or restart Charon. If the card says the file was refused, the message tells you why. If the alert is still there, please open a GitHub issue so we can help. The database is normally included, so a missing file may be a bug. Include the Charon version you use and the startup log lines from `docker logs charon`.

[Open a GitHub issue](https://github.com/Wikid82/Charon/issues/new?template=bug_report.md)
