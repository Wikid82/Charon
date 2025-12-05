# Testing SSL Certificates (Without Breaking Things)

Let's Encrypt gives you free SSL certificates. But there's a catch: **you can only get 50 per week**.

If you're testing or rebuilding a lot, you'll hit that limit fast.

**The solution:** Use "staging mode" for testing. Staging gives you unlimited fake certificates. Once everything works, switch to production for real ones.

---

## What Is Staging Mode?

**Staging** = practice mode
**Production** = real certificates

In staging mode:

- ✅ Unlimited certificates (no rate limits)
- ✅ Works exactly like production
- ❌ Browsers don't trust the certificates (they show "Not Secure")

**Use staging when:**
- Testing new domains
- Rebuilding containers repeatedly
- Learning how SSL works

**Use production when:**
- Your site is ready for visitors
- You need the green lock to show up

---

## Turn On Staging Mode

Add this to your `docker-compose.yml`:

```yaml
environment:
  - CHARON_ACME_STAGING=true
```

Restart Charon:

```bash
docker-compose restart
```

Now when you add domains, they'll use staging certificates.

---

## Switch to Production

When you're ready for real certificates:

### Step 1: Turn Off Staging

Remove or change the line:

```yaml
environment:
  - CHARON_ACME_STAGING=false
```

Or just delete the line entirely.

### Step 2: Delete Staging Certificates

**Option A: Through the UI**

1. Go to **Certificates** page
2. Delete any certificates with "staging" in the name

**Option B: Through Terminal**

```bash
docker exec charon rm -rf /app/data/caddy/data/acme/acme-staging*
```

### Step 3: Restart

```bash
docker-compose restart
```

Charon will automatically get real certificates on the next request.

---

## How to Tell Which Mode You're In

### Check Your Config

Look at your `docker-compose.yml`:

- **Has `CHARON_ACME_STAGING=true`** → Staging mode
- **Doesn't have the line** → Production mode

### Check Your Browser

Visit your website:

- **"Not Secure" warning** → Staging certificate
- **Green lock** → Production certificate

---

## Let's Encrypt Rate Limits

If you hit the limit, you'll see errors like:

```
too many certificates already issued
```

**Production limits:**
- 50 certificates per domain per week
- 5 duplicate certificates per week

**Staging limits:**
- Basically unlimited (thousands per week)

**How to check current limits:** Visit [letsencrypt.org/docs/rate-limits](https://letsencrypt.org/docs/rate-limits/)

---

## Common Questions

### "Why do I see a security warning in staging?"

That's normal. Staging certificates are signed by a fake authority that browsers don't recognize. It's just for testing.

### "Can I use staging for my real website?"

No. Visitors will see "Not Secure" warnings. Use production for real traffic.

### "I switched to production but still see staging certificates"

Delete the old staging certificates (see Step 2 above). Charon won't replace them automatically.

### "Do I need to change anything else?"

No. Staging vs production is just one environment variable. Everything else stays the same.

---

## Best Practices

1. **Always start in staging** when setting up new domains
2. **Test everything** before switching to production
3. **Don't rebuild production constantly** — you'll hit rate limits
4. **Keep staging enabled in development environments**

---

## Still Getting Rate Limited?

If you hit the 50/week limit in production:

1. Switch back to staging for now
2. Wait 7 days (limits reset weekly)
3. Plan your changes so you need fewer rebuilds
4. Use staging for all testing going forward

---

## Technical Note

Under the hood, staging points to:

```
https://acme-staging-v02.api.letsencrypt.org/directory
```

Production points to:

```
https://acme-v02.api.letsencrypt.org/directory
```

You don't need to know this, but if you see these URLs in logs, that's what they mean.
