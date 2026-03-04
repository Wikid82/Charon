# Maintenance Documentation

This directory contains operational maintenance guides for keeping Charon running smoothly.

## Available Guides

### [GeoLite2 Database Checksum Update](geolite2-checksum-update.md)

**When to use:** Docker build fails with GeoLite2-Country.mmdb checksum mismatch

**Topics covered:**
- Automated weekly checksum verification workflow
- Manual checksum update procedures (5 minutes)
- Verification script for checking upstream changes
- Troubleshooting common checksum issues
- Alternative sources if upstream mirrors are unavailable

**Quick fix:**
```bash
# Download and update checksum automatically
NEW_CHECKSUM=$(curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum | cut -d' ' -f1)
sed -i "s/ARG GEOLITE2_COUNTRY_SHA256=.*/ARG GEOLITE2_COUNTRY_SHA256=${NEW_CHECKSUM}/" Dockerfile
docker build --no-cache -t test .
```

---

## Contributing

Found a maintenance issue not covered here? Please:

1. **Create an issue** describing the problem
2. **Document the solution** in a new guide
3. **Update this index** with a link to your guide

**Format:**
```markdown
### [Guide Title](filename.md)

**When to use:** Brief description of when this guide applies

**Topics covered:**
- List key topics

**Quick command:** (if applicable)
```

## Related Documentation

- **[Troubleshooting](../troubleshooting/)** — Common runtime issues and fixes
- **[Runbooks](../runbooks/)** — Emergency procedures and incident response
- **[Configuration](../configuration/)** — Setup and configuration guides
- **[Development](../development/)** — Developer environment and workflows

---

**Last Updated:** February 2, 2026
