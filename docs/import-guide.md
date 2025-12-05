# Import Your Old Caddy Setup

Already using Caddy? You can bring your existing configuration into Charon instead of starting from scratch.

---

## What Gets Imported?

Charon reads your Caddyfile and creates proxy hosts for you automatically. It understands:

- ✅ Domain names
- ✅ Reverse proxy addresses
- ✅ SSL settings
- ✅ Multiple domains per site

---

## How to Import

### Step 1: Go to the Import Page

Click **"Import Caddy Config"** in the sidebar.

### Step 2: Choose Your Method

**Option A: Upload a File**

- Click "Choose File"
- Select your Caddyfile
- Click "Upload"

**Option B: Paste Text**

- Click the "Paste" tab
- Copy your Caddyfile contents
- Paste them into the box
- Click "Parse"

### Step 3: Review What Was Found

Charon shows you a preview:

```
Found 3 sites:
✅ example.com → localhost:3000
✅ api.example.com → localhost:8080
⚠️  files.example.com → (file server - not supported)
```

Green checkmarks = will import
Yellow warnings = can't import (but tells you why)

### Step 4: Handle Conflicts

If you already have a proxy for `example.com`, Charon asks what to do:

- **Keep Existing** — Don't import this one, keep what you have
- **Overwrite** — Replace your current config with the imported one
- **Skip** — Same as "Keep Existing"

Choose what makes sense for each conflict.

### Step 5: Click "Import"

Charon creates proxy hosts for everything you selected. Done!

---

## Example: Simple Caddyfile

**Your Caddyfile:**

```caddyfile
blog.example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    reverse_proxy https://backend:8080
}
```

**What Charon creates:**

- Proxy host: `blog.example.com` → `http://localhost:3000`
- Proxy host: `api.example.com` → `https://backend:8080`

---

## What Doesn't Work (Yet)

Some Caddy features can't be imported:

### File Servers

```caddyfile
static.example.com {
    file_server
    root * /var/www
}
```

**Why:** Charon only handles reverse proxies, not static files.

**Solution:** Keep this in a separate Caddyfile or use a different tool for static hosting.

### Path-Based Routing

```caddyfile
example.com {
    route /api/* {
        reverse_proxy localhost:8080
    }
    route /web/* {
        reverse_proxy localhost:3000
    }
}
```

**Why:** Charon treats each domain as one proxy, not multiple paths.

**Solution:** Create separate subdomains instead:
- `api.example.com` → localhost:8080
- `web.example.com` → localhost:3000

### Environment Variables

```caddyfile
{$DOMAIN} {
    reverse_proxy {$BACKEND}
}
```

**Why:** Charon doesn't know what your environment variables are.

**Solution:** Replace them with actual values before importing.

### Import Statements

```caddyfile
import snippets/common.caddy
```

**Why:** Charon needs the full config in one file.

**Solution:** Combine all files into one before importing.

---

## Tips for Successful Imports

### 1. Simplify First

Remove unsupported directives before importing. Focus on just the reverse_proxy parts.

### 2. Test with One Site

Import a single site first to make sure it works. Then import the rest.

### 3. Keep a Backup

Don't delete your original Caddyfile. Keep it as a backup just in case.

### 4. Review Before Committing

Always check the preview carefully. Make sure addresses and ports are correct.

---

## Troubleshooting

### "No hosts found"

**Problem:** Your Caddyfile only has file servers or other unsupported features.

**Solution:** Add at least one `reverse_proxy` directive or add sites manually through the UI.

### "Parse error"

**Problem:** Your Caddyfile has syntax errors.

**Solution:**
1. Run `caddy validate --config Caddyfile` on your server
2. Fix any errors it reports
3. Try importing again

### "Some hosts failed to import"

**Problem:** Some sites have unsupported features.

**Solution:** Import what works, add the rest manually through the UI.

---

## After Importing

Once imported, you can:

- Edit any proxy host through the UI
- Add SSL certificates (automatic with Let's Encrypt)
- Add security features
- Delete ones you don't need

Everything is now managed by Charon!

---

## What About Nginx Proxy Manager?

NPM import is planned for a future update. For now:

1. Export your NPM config (if possible)
2. Look at which domains point where
3. Add them manually through Charon's UI (it's pretty quick)

---

## Need Help?

**[Ask on GitHub Discussions](https://github.com/Wikid82/charon/discussions)** — Bring your Caddyfile and we'll help you figure out how to import it.
