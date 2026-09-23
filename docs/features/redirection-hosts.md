---
title: Redirection Hosts — Send Old Links to a New Home
description: Point an old domain at a new one with a proper HTTP redirect — no backend server required
category: features
---

# Redirection Hosts — Send Old Links to a New Home

Imagine you've moved house. You don't want the mail carrier to just give up when they reach your old address — you want a forwarding notice on the door that says "we've moved, here's the new address." A Redirection Host is that forwarding notice for a website: anyone who visits your old domain is automatically sent to your new one, with no dead links and no confused visitors.

---

## What Problem Does This Solve?

Domains change. Maybe you renamed your blog, moved a service to a new provider, or retired an old project but people still have the old link bookmarked or linked from somewhere else. Without something in place, visitors to that old address just hit a broken page.

A **Redirection Host** tells Charon: "when someone visits this domain, immediately send their browser to this other URL instead." No content is served from the old domain — visitors are simply redirected onward.

This is different from a **Proxy Host**, which is what you use for most sites in Charon:

| | Proxy Host | Redirection Host |
|---|---|---|
| What it does | Fetches content from a backend app/server and hands it to the visitor, through Charon | Tells the visitor's browser "go here instead" |
| Use it when | You're running an actual app or service you want to make available under a domain | You've moved a site/domain and want the old address to forward to the new one |
| Example | `myapp.example.com` → your Docker container running the app | `old-blog.example.com` → `https://newblog.example.com` |

If you're not sure which one you need: if there's an app or service actually running that should answer the domain, use a **Proxy Host**. If you just want visitors bounced along to somewhere else, use a **Redirection Host**.

---

## Who Is This For, and When Should You Use It?

Reach for a Redirection Host any time you have a domain with nothing left behind it except "please go somewhere else":

- You migrated your blog or site to a new domain and want old links (and search engine results) to keep working.
- You retired a service or app but still get occasional visits to its old address.
- You renamed a project and want the old brand's domain to land people on the new one.
- You want `www.example.com` to always redirect to `example.com` (or the reverse) — pick whichever you use as your primary domain.

You don't need any technical knowledge of HTTP or web servers to use this — just the old domain and the new URL you want people sent to.

---

## Where to Find It

Look for **Redirection Hosts** in the Charon sidebar, right below **Proxy Hosts**. Clicking it opens a page listing all of your redirects, with columns for Name, Domain, Target, Status Code, and an Enabled toggle so you can turn any redirect on or off without deleting it.

---

## How to Set One Up

### Step 1 — Click "Add Redirection Host"

On the Redirection Hosts page, click the **Add Redirection Host** button. A form opens with the following fields:

- **Name** — A friendly label so you can recognize this redirect later (e.g. "Old blog redirect"). Purely for your own reference.
- **Domain Names** — The old domain(s) you want to redirect *from*. You can enter more than one, separated by commas (e.g. `old-blog.example.com, www.old-blog.example.com`).
- **Target URL** — The full web address you want visitors sent *to*, including `https://` (e.g. `https://newblog.example.com`).
- **Status Code** — Which "kind" of redirect to send (explained below).
- **Preserve Path** — A toggle that decides whether the specific page a visitor requested carries over to the new domain (explained below).
- **Force SSL**, **HTTP/2 Support**, **HSTS Enabled**, **HSTS Subdomains** — the same security toggles you'd see on a Proxy Host. Leave these on unless you have a specific reason to change them.
- **Certificate** — Which SSL certificate secures the old domain. Leave this on "Auto-manage with Let's Encrypt (recommended)" and Charon takes care of getting and renewing a certificate for you.
- **Use DNS Challenge** — Turn this on only if you need a wildcard certificate (covering something like `*.example.com`) and have a DNS Provider set up. Most people can leave this off.

Fill in at minimum a domain and a target URL, then click **Save**.

> Even though this domain is only redirecting visitors elsewhere, Charon still secures it with a real SSL certificate — so the padlock in the browser is valid the instant someone lands on the old address, before they're sent on their way.

### Step 2 — Choose the Right Status Code

A "status code" is just a short numeric signal the web server sends along with the redirect, telling the visitor's browser (and search engines) what kind of move this is. Charon offers four choices:

| Code | Plain-language meaning | Pick this when... |
|---|---|---|
| **301 — Permanent** | "This has moved for good." | You're moving a site permanently and want search engines to update their listings to the new address over time. This is the right choice for most people. |
| **302 — Temporary** | "This has moved for now, but might move back." | The move is temporary — e.g. you're doing maintenance on the new site or aren't sure the new address is final yet. Search engines will keep the old address in their index. |
| **307 — Temporary (preserve method)** | Same as 302, but also makes sure that if the visitor's browser was submitting a form or sending data, it keeps sending it the same way to the new address. | You need a temporary redirect for something more than a simple page visit — for example, an app or script that's sending data (not just browsing) to the old domain. |
| **308 — Permanent (preserve method)** | Same as 301, but also preserves form/data submissions the same way 307 does. | You need a permanent redirect for the same data-submitting scenario described above. |

**Simple rule of thumb:** if you're not dealing with an app that submits data (forms, API calls) to the old domain, just pick **301** for a permanent move or **302** for a temporary one — that covers the vast majority of cases, including "I moved my blog" or "I renamed my site."

### Step 3 — Decide Whether to Preserve the Path

Turn on **Preserve Path** and Charon carries over whatever specific page (and any search/filter options in the address) the visitor was trying to reach, appending it to your target URL. Leave it off, and every visitor lands on exactly the Target URL you typed, no matter what page they were trying to reach.

**Example — Preserve Path ON:**

A visitor goes to `old-blog.example.com/posts/my-favorite-recipe?ref=newsletter`. With Target URL set to `https://newblog.example.com`, they land on:

```
https://newblog.example.com/posts/my-favorite-recipe?ref=newsletter
```

**Example — Preserve Path OFF:**

The same visitor instead lands on exactly:

```
https://newblog.example.com
```

no matter what specific page or link they originally clicked.

If you moved a whole site with all its pages intact under the new domain, turn Preserve Path **on** so old bookmarks and search results keep working page-for-page. If you're redirecting to a single landing page regardless of what was requested (e.g. everything now points to one "we've moved" announcement page), turn it **off**.

### Step 4 — Save and Confirm

Once saved, your Redirection Host appears in the list, and the redirect is live within a few seconds. Visit the old domain to confirm it lands you on the target URL you configured.

---

## A Few Things Worth Knowing

- **Domain names must be unique.** You can't set up a Redirection Host for a domain that's already in use by a Proxy Host (or another Redirection Host) — Charon will tell you if there's a conflict, so you never end up with two rules quietly fighting over the same address.
- **No redirecting a domain to itself.** Charon blocks you from pointing a domain's redirect target back at one of its own domain names — that would just send visitors in an endless loop.
- **Turning a redirect off** (via the Enabled toggle) stops it from responding without deleting your configuration, so you can easily re-enable it later.
- **This only redirects — it doesn't proxy content.** If you later decide the old domain needs to actually serve content again (rather than bounce visitors elsewhere), delete the Redirection Host and set up a Proxy Host instead.

---

*Not sure if you need a redirect or a proxy? See [Proxy Hosts](web-ui.md) for the standard way to publish an app or service through Charon.*
