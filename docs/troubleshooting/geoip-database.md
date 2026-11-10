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

### How to Add the Database Yourself

1. Get a free **GeoLite2-Country** database from MaxMind (you need a free MaxMind account). The file ends in `.mmdb`.
2. Rename it to `GeoLite2-Country.mmdb` if it has a different name.
3. Copy it into the `geoip` folder inside your data folder. In the container, that folder is `/app/data/geoip/`. On your computer, it is the `geoip` folder inside the folder you mounted as `/app/data`. Create the `geoip` folder if it does not exist.
4. Restart Charon. In a terminal, run `docker restart charon`, or restart the container from your Docker tool.

**Want to use a file in another location?** Set the `CHARON_GEOIP_DB_PATH` setting to the full path of the `.mmdb` file, then restart Charon. This path wins over the file in the `geoip` folder. The file name must end in `.mmdb`.

### Confirm It Worked

1. Wait for Charon to start again.
2. Open the notification bell. The red alert should be gone.

The alert clears by itself once Charon can read the database. You do not need to dismiss it.

### Still Seeing the Alert?

Check that the file is in the right folder, that its name ends in `.mmdb`, and that you restarted Charon after copying it. If it is still there, please open a GitHub issue so we can help. The database is normally included, so a missing file may be a bug. Include the Charon version you use and the startup log lines from `docker logs charon`.

[Open a GitHub issue](https://github.com/Wikid82/Charon/issues/new?template=bug_report.md)
