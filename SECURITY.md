# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in Charon, please report it responsibly.

### Where to Report

**Preferred Method**: GitHub Security Advisory (Private)
1. Go to <https://github.com/Wikid82/charon/security/advisories/new>
2. Fill out the advisory form with:
   - Vulnerability description
   - Steps to reproduce
   - Proof of concept (non-destructive)
   - Impact assessment
   - Suggested fix (if applicable)

**Alternative Method**: Email
- Send to: `security@charon.dev` (if configured)
- Use PGP encryption (key available below, if applicable)
- Include same information as GitHub advisory

### What to Include

Please provide:

1. **Description**: Clear explanation of the vulnerability
2. **Reproduction Steps**: Detailed steps to reproduce the issue
3. **Impact Assessment**: What an attacker could do with this vulnerability
4. **Environment**: Charon version, deployment method, OS, etc.
5. **Proof of Concept**: Code or commands demonstrating the vulnerability (non-destructive)
6. **Suggested Fix**: If you have ideas for remediation

### What Happens Next

1. **Acknowledgment**: We'll acknowledge your report within **48 hours**
2. **Investigation**: We'll investigate and assess the severity
3. **Updates**: We'll provide regular status updates (weekly minimum)
4. **Fix Development**: We'll develop and test a fix
5. **Disclosure**: Coordinated disclosure after fix is released
6. **Credit**: We'll credit you in release notes (if desired)

### Responsible Disclosure

We ask that you:

- ✅ Give us reasonable time to fix the issue before public disclosure (90 days preferred)
- ✅ Avoid destructive testing or attacks on production systems
- ✅ Not access, modify, or delete data that doesn't belong to you
- ✅ Not perform actions that could degrade service for others

We commit to:

- ✅ Respond to your report within 48 hours
- ✅ Provide regular status updates
- ✅ Credit you in release notes (if desired)
- ✅ Not pursue legal action for good-faith security research

---

## Security Features

### Server-Side Request Forgery (SSRF) Protection

Charon implements industry-leading SSRF protection to prevent attackers from using the application to access internal resources or cloud metadata.

#### Protected Against

- **Private network access** (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- **Cloud provider metadata endpoints** (AWS, Azure, GCP)
- **Localhost and loopback addresses** (127.0.0.0/8, ::1/128)
- **Link-local addresses** (169.254.0.0/16, fe80::/10)
- **Protocol bypass attacks** (file://, ftp://, gopher://, data:)

#### Validation Process

All user-controlled URLs undergo:

1. **URL Format Validation**: Scheme, syntax, and structure checks
2. **DNS Resolution**: Hostname resolution with timeout protection
3. **IP Range Validation**: Blocked ranges include 13+ CIDR blocks
4. **Request Execution**: Timeout enforcement and redirect limiting

#### Protected Features

- Security notification webhooks
- Custom webhook notifications
- CrowdSec hub synchronization
- External URL connectivity testing (admin-only)

#### Learn More

For complete technical details, see:
- [SSRF Protection Guide](docs/security/ssrf-protection.md)
- [Implementation Report](docs/implementation/SSRF_REMEDIATION_COMPLETE.md)
- [QA Audit Report](docs/reports/qa_ssrf_remediation_report.md)

---

### Authentication & Authorization

- **JWT-based authentication**: Secure token-based sessions
- **Role-based access control**: Admin vs. user permissions
- **Session management**: Automatic expiration and renewal
- **Secure cookie attributes**: HttpOnly, Secure (HTTPS), SameSite

### Data Protection

- **Database encryption**: Sensitive data encrypted at rest
- **Secure credential storage**: Hashed passwords, encrypted API keys
- **Input validation**: All user inputs sanitized and validated
- **Output encoding**: XSS protection via proper encoding

### Infrastructure Security

- **Container isolation**: Docker-based deployment
- **Minimal attack surface**: Alpine Linux base image
- **Dependency scanning**: Regular Trivy and govulncheck scans
- **No unnecessary services**: Single-purpose container design

### Web Application Firewall (WAF)

- **Coraza WAF integration**: OWASP Core Rule Set support
- **Rate limiting**: Protection against brute-force and DoS
- **IP allowlisting/blocklisting**: Network access control
- **CrowdSec integration**: Collaborative threat intelligence

---

## Security Best Practices

### Deployment Recommendations

1. **Use HTTPS**: Always deploy behind a reverse proxy with TLS
2. **Restrict Admin Access**: Limit admin panel to trusted IPs
3. **Regular Updates**: Keep Charon and dependencies up to date
4. **Secure Webhooks**: Only use trusted webhook endpoints
5. **Strong Passwords**: Enforce password complexity policies
6. **Backup Encryption**: Encrypt backup files before storage

### Configuration Hardening

```yaml
# Recommended docker-compose.yml settings
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    restart: unless-stopped
    environment:
      - CHARON_ENV=production
      - LOG_LEVEL=info  # Don't use debug in production
    volumes:
      - ./charon-data:/app/data:rw
      - /var/run/docker.sock:/var/run/docker.sock:ro  # Read-only!
    networks:
      - charon-internal  # Isolated network
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Only if binding to ports < 1024
    security_opt:
      - no-new-privileges:true
    read_only: true  # If possible
    tmpfs:
      - /tmp:noexec,nosuid,nodev
```

### Network Security

- **Firewall Rules**: Only expose necessary ports (80, 443, 8080)
- **VPN Access**: Use VPN for admin access in production
- **Fail2Ban**: Consider fail2ban for brute-force protection
- **Intrusion Detection**: Enable CrowdSec for threat detection

---

## Security Audits & Scanning

### Automated Scanning

We use the following tools:

- **Trivy**: Container image vulnerability scanning
- **CodeQL**: Static code analysis for Go and JavaScript
- **govulncheck**: Go module vulnerability scanning
- **golangci-lint**: Go code linting (including gosec)
- **npm audit**: Frontend dependency vulnerability scanning

### Manual Reviews

- Security code reviews for all major features
- Peer review of security-sensitive changes
- Third-party security audits (planned)

### Continuous Monitoring

- GitHub Dependabot alerts
- Weekly security scans in CI/CD
- Community vulnerability reports

---

## Known Security Considerations

### Third-Party Dependencies

**CrowdSec Binaries**: As of December 2025, CrowdSec binaries shipped with Charon contain 4 HIGH-severity CVEs in Go stdlib (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729). These are upstream issues in Go 1.25.1 and will be resolved when CrowdSec releases binaries built with Go 1.25.5+.

**Impact**: Low. These vulnerabilities are in CrowdSec's third-party binaries, not in Charon's application code. They affect HTTP/2, TLS certificate handling, and archive parsing—areas not directly exposed to attackers through Charon's interface.

**Mitigation**: Monitor CrowdSec releases for updated binaries. Charon's own application code has zero vulnerabilities.

---

## Security Hall of Fame

We recognize security researchers who help improve Charon:

<!-- Add contributors here -->
- *Your name could be here!*

---

## Security Contact

- **GitHub Security Advisories**: <https://github.com/Wikid82/charon/security/advisories>
- **GitHub Discussions**: <https://github.com/Wikid82/charon/discussions>
- **GitHub Issues** (non-security): <https://github.com/Wikid82/charon/issues>

---

## License

This security policy is part of the Charon project, licensed under the MIT License.

---

**Last Updated**: December 24, 2025
**Version**: 1.1
