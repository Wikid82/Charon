---
title: Troubleshooting Standard Proxy Headers
description: Resolve issues with Charon's X-Real-IP, X-Forwarded-Proto, and other standard proxy headers.
---

## Troubleshooting Standard Proxy Headers

This guide helps resolve issues with Charon's standard proxy headers feature.

---

## Understanding Standard Proxy Headers

When enabled, Charon adds these headers to requests sent to your backend:

- **X-Real-IP**: The actual client IP address (not Charon's)
- **X-Forwarded-Proto**: Original protocol (http or https)
- **X-Forwarded-Host**: Original hostname from the client
- **X-Forwarded-Port**: Original port number
- **X-Forwarded-For**: Chain of proxy IPs (managed by Caddy)

---

## Problem: Backend Still Sees Charon's IP

### Symptoms

Your application logs show Charon's internal IP (e.g., `172.17.0.1`) instead of the real client IP.

### Solutions

**1. Verify the feature is enabled**

- Go to **Proxy Hosts** → Edit the affected host
- Scroll to **Standard Proxy Headers** section
- Ensure the checkbox is **checked**
- Click **Save**

**2. Configure backend to trust proxy headers**

Your backend application must be configured to read these headers. Here's how:

**Express.js/Node.js:**

```javascript
// Enable trust proxy
app.set('trust proxy', true);

// Now req.ip will use X-Real-IP or X-Forwarded-For
app.get('/', (req, res) => {
  console.log('Client IP:', req.ip);
});
```

**Django:**

```python
# settings.py
USE_X_FORWARDED_HOST = True
SECURE_PROXY_SSL_HEADER = ('HTTP_X_FORWARDED_PROTO', 'https')

# For real IP detection, install django-ipware:
# pip install django-ipware
from ipware import get_client_ip
client_ip, is_routable = get_client_ip(request)
```

**Flask:**

```python
from werkzeug.middleware.proxy_fix import ProxyFix

# Apply proxy fix middleware
app.wsgi_app = ProxyFix(
    app.wsgi_app,
    x_for=1,  # Trust 1 proxy for X-Forwarded-For
    x_proto=1,  # Trust X-Forwarded-Proto
    x_host=1,  # Trust X-Forwarded-Host
    x_port=1   # Trust X-Forwarded-Port
)

# Now request.remote_addr will show the real client IP
@app.route('/')
def index():
    client_ip = request.remote_addr
    return f'Your IP: {client_ip}'
```

**Go (net/http):**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Read X-Real-IP header
    clientIP := r.Header.Get("X-Real-IP")
    if clientIP == "" {
        // Fallback to X-Forwarded-For
        clientIP = strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
    }
    if clientIP == "" {
        // Last resort: connection IP
        clientIP = r.RemoteAddr
    }

    log.Printf("Client IP: %s", strings.TrimSpace(clientIP))
}
```

**NGINX (as backend):**

```nginx
# In your server block
real_ip_header X-Real-IP;
set_real_ip_from 172.16.0.0/12;  # Charon's Docker network
real_ip_recursive on;
```

**Apache (as backend):**

```apache
# Enable mod_remoteip
<IfModule mod_remoteip.c>
    RemoteIPHeader X-Real-IP
    RemoteIPInternalProxy 172.16.0.0/12
</IfModule>
```

**3. Verify headers are present**

Use `curl` to check if headers are actually being sent:

```bash
# Replace with your backend's URL
curl -H "Host: yourdomain.com" http://your-backend:8080 -v 2>&1 | grep -i "x-"
```

Look for:

```
> X-Real-IP: 203.0.113.42
> X-Forwarded-Proto: https
> X-Forwarded-Host: yourdomain.com
> X-Forwarded-Port: 443
```

---

## Problem: HTTPS Redirect Loop

### Symptoms

Visiting `https://yourdomain.com` redirects endlessly, browser shows "too many redirects" error.

### Cause

Your backend application is checking the connection protocol instead of the `X-Forwarded-Proto` header.

### Solutions

**Update your redirect logic:**

**Express.js:**

```javascript
// BAD: Checks the direct connection (always http from Charon)
if (req.protocol !== 'https') {
  return res.redirect('https://' + req.get('host') + req.originalUrl);
}

// GOOD: Checks X-Forwarded-Proto header
if (req.get('x-forwarded-proto') !== 'https') {
  return res.redirect('https://' + req.get('host') + req.originalUrl);
}

// BEST: Use trust proxy (it checks X-Forwarded-Proto automatically)
app.set('trust proxy', true);
if (req.protocol !== 'https') {
  return res.redirect('https://' + req.get('host') + req.originalUrl);
}
```

**Django:**

```python
# settings.py
SECURE_PROXY_SSL_HEADER = ('HTTP_X_FORWARDED_PROTO', 'https')

# Now request.is_secure() will check X-Forwarded-Proto
if not request.is_secure():
    return redirect('https://' + request.get_host() + request.get_full_path())
```

**Laravel:**

```php
// app/Http/Middleware/TrustProxies.php
protected $proxies = '*';  // Trust all proxies (or specify Charon's IP)
protected $headers = Request::HEADER_X_FORWARDED_ALL;
```

---

## Problem: Application Breaks After Enabling Headers

### Symptoms

- 500 Internal Server Error
- Application behaves unexpectedly
- Features stop working

### Possible Causes

1. **Strict header validation**: Your app rejects unexpected headers
2. **Conflicting logic**: App has custom IP detection that conflicts with proxy headers
3. **Security middleware**: App blocks requests with proxy headers (anti-spoofing)

### Solutions

**1. Check application logs**

Look for errors mentioning:

- X-Real-IP
- X-Forwarded-*
- Proxy headers
- IP validation

**2. Temporarily disable the feature**

- Edit the proxy host
- Uncheck **"Enable Standard Proxy Headers"**
- Save and test if the app works again

**3. Configure security middleware**

Some security frameworks block proxy headers by default:

**Helmet.js (Express):**

```javascript
// Allow proxy headers
app.use(helmet({
  frameguard: false  // If you need to allow iframes
}));
app.set('trust proxy', true);
```

**Django Security Middleware:**

```python
# settings.py
SECURE_PROXY_SSL_HEADER = ('HTTP_X_FORWARDED_PROTO', 'https')
USE_X_FORWARDED_HOST = True
```

**4. Whitelist Charon's IP**

If your app has IP filtering:

```javascript
// Express example
const trustedProxies = ['172.17.0.1', '172.18.0.1'];
app.set('trust proxy', trustedProxies);
```

---

## Problem: Wrong IP in Rate Limiting

### Symptoms

All users share the same rate limit, or all requests appear to come from one IP.

### Cause

Rate limiting middleware is checking the connection IP instead of proxy headers.

### Solutions

**Express-rate-limit:**

```javascript
import rateLimit from 'express-rate-limit';

// Trust proxy first
app.set('trust proxy', true);

// Then apply rate limit
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000,  // 15 minutes
  max: 100,  // 100 requests per window
  standardHeaders: true,
  legacyHeaders: false,
  // Rate limit will use req.ip (which uses X-Real-IP when trust proxy is set)
});

app.use(limiter);
```

**Custom middleware:**

```javascript
function getRealIP(req) {
  return req.headers['x-real-ip'] ||
         req.headers['x-forwarded-for']?.split(',')[0] ||
         req.ip;
}
```

---

## Problem: GeoIP Location is Wrong

### Symptoms

Users from the US are detected as being from your server's location.

### Cause

GeoIP lookup is using Charon's IP instead of the client IP.

### Solutions

**Ensure proxy headers are enabled** in Charon, then:

**MaxMind GeoIP2 (Node.js):**

```javascript
import maxmind from 'maxmind';

const lookup = await maxmind.open('/path/to/GeoLite2-City.mmdb');

function getLocation(req) {
  const clientIP = req.headers['x-real-ip'] || req.ip;
  return lookup.get(clientIP);
}
```

**Python geoip2:**

```python
import geoip2.database

reader = geoip2.database.Reader('/path/to/GeoLite2-City.mmdb')

def get_location(request):
    client_ip = request.META.get('HTTP_X_REAL_IP') or request.META.get('REMOTE_ADDR')
    return reader.city(client_ip)
```

---

## Problem: Logs Show Multiple IPs in X-Forwarded-For

### Symptoms

X-Forwarded-For shows: `203.0.113.42, 172.17.0.1`

### Explanation

This is **correct behavior**. X-Forwarded-For is a comma-separated list:

1. First IP = Real client (`203.0.113.42`)
2. Second IP = Charon's IP (`172.17.0.1`)

### Get the Real Client IP

**Always read the FIRST IP** in X-Forwarded-For:

```javascript
const forwardedFor = req.headers['x-forwarded-for'];
const clientIP = forwardedFor ? forwardedFor.split(',')[0].trim() : req.ip;
```

**Or use X-Real-IP** (simpler):

```javascript
const clientIP = req.headers['x-real-ip'] || req.ip;
```

---

## Testing Headers Locally

**1. Check if headers reach your backend:**

Add temporary logging:

```javascript
// Express
app.use((req, res, next) => {
  console.log('Headers:', req.headers);
  next();
});
```

```python
# Django middleware
def log_headers(get_response):
    def middleware(request):
        print('Headers:', request.META)
        return get_response(request)
    return middleware
```

**2. Simulate client request with curl:**

```bash
# Test from outside Charon
curl -H "Host: yourdomain.com" https://yourdomain.com/test

# Check your backend logs for:
# X-Real-IP: <your actual IP>
# X-Forwarded-Proto: https
# X-Forwarded-Host: yourdomain.com
```

---

## Security Considerations

### IP Spoofing Prevention

Charon configures Caddy with `trusted_proxies` to prevent clients from spoofing headers.

**What this means:**

- Clients CANNOT inject fake X-Real-IP headers
- Caddy overwrites any client-provided proxy headers
- Only Charon's headers are trusted

**Backend security:**
Your backend should still:

1. Only trust proxy headers from Charon's IP
2. Validate IP addresses before using them for access control
3. Use a proper IP parsing library (not regex)

### Example: Trust Proxy Configuration

```javascript
// Express: Only trust specific IPs
app.set('trust proxy', ['127.0.0.1', '172.17.0.0/16']);

// Nginx: Specify allowed proxy IPs
real_ip_header X-Real-IP;
set_real_ip_from 172.17.0.0/16;  # Docker network
set_real_ip_from 10.0.0.0/8;     # Internal network
```

---

## When to Contact Support

If you've tried the above solutions and:

- ✅ Standard headers are enabled in Charon
- ✅ Backend is configured to trust proxies
- ✅ Headers are visible in logs but still not working
- ✅ No redirect loops or errors

**[Open an issue](https://github.com/Wikid82/charon/issues)** with:

1. Backend framework and version (e.g., Express 4.18.2)
2. Charon version (from Dashboard)
3. Proxy host configuration (screenshot or JSON)
4. Sample backend logs showing the headers
5. Expected vs actual behavior

---

## Additional Resources

- [Features Guide: Standard Proxy Headers](../features.md#-standard-proxy-headers)
- [Getting Started: Adding Your First Website](../getting-started.md#step-2-add-your-first-website)
- [API Documentation: Proxy Hosts](../api.md#proxy-hosts)
- [RFC 7239: Forwarded HTTP Extension](https://tools.ietf.org/html/rfc7239)
