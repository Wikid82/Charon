# Plex Remote Access Helper & CGNAT Solver

> **GitHub Issue Template** - Copy this content to create a new GitHub issue

---

## Issue Title
`Plex Remote Access Helper & CGNAT Solver`

## Labels
`beta`, `feature`, `plus`, `ui`, `caddy`

---

## Description
Implement a "Plex Remote Access Helper" feature that assists users stuck behind CGNAT (Carrier-Grade NAT) to properly configure their Plex Media Server for remote streaming via a reverse proxy like Caddy. This feature addresses the common pain point of Plex remote access failures when users cannot open ports due to ISP limitations.

## Parent Issue
Extends #44 (Tailscale Network Integration) and #43 (Remote Servers Management)

## Why This Feature?
- **CGNAT is increasingly common** - Many ISPs (especially mobile carriers like T-Mobile) use Carrier-Grade NAT, preventing users from forwarding ports
- **Plex is one of the most popular homelab applications** - A significant portion of Charon users will have Plex
- **Manual configuration is error-prone** - Users often struggle with the correct Caddy configuration and Plex settings
- **Tailscale/VPN integration makes this possible** - With #44, users can access their home network, but Plex requires specific proxy headers for proper remote client handling
- **User story origin** - This feature was conceived from a real user experience solving CGNAT issues with Plex + Tailscale

## Use Cases
1. **T-Mobile/Starlink Home Internet users** - Cannot port forward, need VPN tunnel + reverse proxy
2. **Apartment/Dorm residents** - Shared internet without port access
3. **Privacy-conscious users** - Prefer VPN tunnel over exposing ports
4. **Multi-server Plex setups** - Proxying to multiple Plex instances

## Tasks
- [ ] Design "Plex Mode" toggle or "Media Server Helper" option in proxy host creation
- [ ] Implement automatic header injection for Plex compatibility:
  - `X-Forwarded-For` - Client's real IP address
  - `X-Forwarded-Proto` - HTTPS
  - `X-Real-IP` - Client IP
  - `X-Plex-Client-Identifier` - Passthrough
- [ ] Create "External Domain" text input for Plex custom URL setting
- [ ] Generate copy-paste snippet for Plex Settings → Network → Custom server access URLs
- [ ] Add Plex-specific Caddy configuration template
- [ ] Implement WebSocket support toggle (required for Plex Companion)
- [ ] Create validation/test button to verify proxy is working
- [ ] Add documentation/guide for CGNAT + Tailscale + Plex setup
- [ ] Implement connection type detection (show if traffic appears Local vs Remote in proxy logs)
- [ ] Add warning about bandwidth limiting implications when headers are missing

## Acceptance Criteria
- [ ] User can enable "Plex Mode" when creating a proxy host
- [ ] Correct headers are automatically added to Caddy config
- [ ] Copy-paste snippet generated for Plex custom URL setting
- [ ] WebSocket connections work for Plex Companion features
- [ ] Documentation explains full CGNAT + Tailscale + Plex workflow
- [ ] Remote streams correctly show as "Remote" in Plex dashboard (not "Local")
- [ ] Works with both HTTP and HTTPS upstream Plex servers

## Technical Considerations

### Caddy Configuration Template
```caddyfile
plex.example.com {
    reverse_proxy localhost:32400 {
        # Required headers for proper Plex remote access
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        header_up X-Real-IP {remote_host}

        # Preserve Plex-specific headers
        header_up X-Plex-Client-Identifier {header.X-Plex-Client-Identifier}
        header_up X-Plex-Device {header.X-Plex-Device}
        header_up X-Plex-Device-Name {header.X-Plex-Device-Name}
        header_up X-Plex-Platform {header.X-Plex-Platform}
        header_up X-Plex-Platform-Version {header.X-Plex-Platform-Version}
        header_up X-Plex-Product {header.X-Plex-Product}
        header_up X-Plex-Token {header.X-Plex-Token}
        header_up X-Plex-Version {header.X-Plex-Version}

        # WebSocket support for Plex Companion
        transport http {
            read_buffer 8192
        }
    }
}
```

### Plex Settings Required
Users must configure in Plex Settings → Network:
- **Secure connections**: Preferred (not Required, to allow proxy)
- **Custom server access URLs**: `https://plex.example.com:443`

### Integration with Existing Features
- Leverage Remote Servers (#43) for Plex server discovery
- Use Tailscale integration (#44) for CGNAT bypass
- Apply to Cloudflare Tunnel (#47) for additional NAT traversal option

### Header Behavior Notes
- Without `X-Forwarded-For`: Plex sees all traffic as coming from the proxy's IP (e.g., Tailscale 100.x.x.x)
- This may cause Plex to treat remote traffic as "Local," bypassing bandwidth limits
- Users should be warned about this behavior in the UI

## UI/UX Design Notes

### Proxy Host Creation Form
Add a collapsible "Media Server Settings" section:
```
☑ Enable Plex Mode

  External Domain for Plex: [ plex.example.com          ]

  [📋 Copy Plex Custom URL]
  → https://plex.example.com:443

  ℹ️ Add this URL to Plex Settings → Network → Custom server access URLs

  ☑ Forward client IP headers (recommended)
  ☑ Enable WebSocket support
```

### Quick Start Template
In Onboarding Wizard (#30), add "Plex" as a Quick Start template option:
- Pre-configures port 32400
- Enables Plex Mode automatically
- Provides step-by-step instructions

## Documentation Sections to Add
1. **CGNAT Explained** - What is CGNAT and why it blocks remote access
2. **Tailscale + Plex Setup Guide** - Complete walkthrough
3. **Troubleshooting Remote Access** - Common issues and solutions
4. **Local vs Remote Traffic** - Explaining header behavior
5. **Bandwidth Limiting Gotcha** - Why headers matter for throttling

## Priority
Medium - Valuable user experience improvement, builds on #44

## Milestone
Beta

## Related Issues
- #44 (Tailscale Network Integration) - Provides the VPN tunnel
- #43 (Remote Servers Management) - Server discovery
- #47 (Cloudflare Tunnel Integration) - Alternative NAT traversal
- #30 (Onboarding Wizard) - Quick Start templates

## Future Extensions
- Support for other media servers (Jellyfin, Emby)
- Automatic Plex server detection via UPnP/SSDP
- Integration with Tautulli for monitoring
- Plex claim token setup assistance

---

## How to Create This Issue

1. Go to https://github.com/Wikid82/charon/issues/new
2. Use title: `Plex Remote Access Helper & CGNAT Solver`
3. Add labels: `beta`, `feature`, `plus`, `ui`, `caddy`
4. Copy the content from "## Description" through "## Future Extensions"
5. Submit the issue

---

*Issue specification created: 2025-11-27*
*Origin: Gemini-assisted Plex remote streaming solution using Tailscale*
