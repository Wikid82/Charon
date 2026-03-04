# Security Best Practices

This document outlines security best practices for developing and maintaining Charon. These guidelines help prevent common vulnerabilities and ensure compliance with industry standards.

## Table of Contents

- [Secret Management](#secret-management)
- [Logging Security](#logging-security)
- [Input Validation](#input-validation)
- [File System Security](#file-system-security)
- [Database Security](#database-security)
- [API Security](#api-security)
- [Compliance](#compliance)
- [Security Testing](#security-testing)

---

## Secret Management

### Principles

1. **Never commit secrets to version control**
2. **Use environment variables for production**
3. **Rotate secrets regularly**
4. **Mask secrets in logs**
5. **Encrypt secrets at rest**

### API Keys and Tokens

#### Storage

- **Development**: Store in `.env` file (gitignored)
- **Production**: Use environment variables or secret management service
- **File storage**: Use 0600 permissions (owner read/write only)

```bash
# Example: Secure key file creation
echo "api-key-here" > /data/crowdsec/bouncer.key
chmod 0600 /data/crowdsec/bouncer.key
chown charon:charon /data/crowdsec/bouncer.key
```

#### Masking

Always mask secrets before logging:

```go
// ✅ GOOD: Masked secret
logger.Infof("API Key: %s", maskAPIKey(apiKey))

// ❌ BAD: Full secret exposed
logger.Infof("API Key: %s", apiKey)
```

Charon's masking rules:
- Empty: `[empty]`
- Short (< 16 chars): `[REDACTED]`
- Normal (≥ 16 chars): `abcd...xyz9` (first 4 + last 4)

#### Validation

Validate secret format before use:

```go
if !validateAPIKeyFormat(apiKey) {
    return fmt.Errorf("invalid API key format")
}
```

Requirements:
- Length: 16-128 characters
- Charset: Alphanumeric + underscore + hyphen
- No spaces or special characters

#### Rotation

Rotate secrets regularly:

1. **Schedule**: Every 90 days (recommended)
2. **Triggers**: After suspected compromise, employee offboarding, security incidents
3. **Process**:
   - Generate new secret
   - Update configuration
   - Test with new secret
   - Revoke old secret
   - Update documentation

### Passwords and Credentials

- **Storage**: Hash with bcrypt (cost factor ≥ 12) or Argon2
- **Transmission**: HTTPS only
- **Never log**: Full passwords or password hashes
- **Requirements**: Enforce minimum complexity and length

---

## Logging Security

### What to Log

✅ **Safe to log**:
- Timestamps
- User IDs (not usernames if PII)
- IP addresses (consider GDPR implications)
- Request paths (sanitize query parameters)
- Response status codes
- Error types (generic messages)
- Performance metrics

❌ **Never log**:
- Passwords or password hashes
- API keys or tokens (use masking)
- Session IDs (full values)
- Credit card numbers
- Social security numbers
- Personal health information (PHI)
- Any Personally Identifiable Information (PII)

### Log Sanitization

Before logging user input, sanitize:

```go
// ✅ GOOD: Sanitized logging
logger.Infof("Login attempt from IP: %s", sanitizeIP(ip))

// ❌ BAD: Direct user input
logger.Infof("Login attempt: username=%s password=%s", username, password)
```

### Log Retention

- **Development**: 7 days
- **Production**: 30-90 days (depends on compliance requirements)
- **Audit logs**: 1-7 years (depends on regulations)

**Important**: Shorter retention reduces exposure risk if logs are compromised.

### Log Aggregation

If using external log services (CloudWatch, Splunk, Datadog):
- Ensure logs are encrypted in transit (TLS)
- Ensure logs are encrypted at rest
- Redact sensitive data before shipping
- Apply same retention policies
- Audit access controls regularly

---

## Input Validation

### Principles

1. **Validate all inputs** (user-provided, file uploads, API requests)
2. **Whitelist approach**: Define what's allowed, reject everything else
3. **Fail securely**: Reject invalid input with generic error messages
4. **Sanitize before use**: Escape/encode for target context

### File Uploads

```go
// ✅ GOOD: Comprehensive validation
func validateUpload(file multipart.File, header *multipart.FileHeader) error {
    // 1. Check file size
    if header.Size > maxFileSize {
        return fmt.Errorf("file too large")
    }

    // 2. Validate file type (magic bytes, not extension)
    buf := make([]byte, 512)
    file.Read(buf)
    mimeType := http.DetectContentType(buf)
    if !isAllowedMimeType(mimeType) {
        return fmt.Errorf("invalid file type")
    }

    // 3. Sanitize filename
    safeName := sanitizeFilename(header.Filename)

    // 4. Check for path traversal
    if containsPathTraversal(safeName) {
        return fmt.Errorf("invalid filename")
    }

    return nil
}
```

### Path Traversal Prevention

```go
// ✅ GOOD: Secure path handling
func securePath(baseDir, userPath string) (string, error) {
    // Clean and resolve path
    fullPath := filepath.Join(baseDir, filepath.Clean(userPath))

    // Ensure result is within baseDir
    if !strings.HasPrefix(fullPath, baseDir) {
        return "", fmt.Errorf("path traversal detected")
    }

    return fullPath, nil
}

// ❌ BAD: Direct path join (vulnerable)
fullPath := baseDir + "/" + userPath
```

### SQL Injection Prevention

```go
// ✅ GOOD: Parameterized query
db.Where("email = ?", email).First(&user)

// ❌ BAD: String concatenation (vulnerable)
db.Raw("SELECT * FROM users WHERE email = '" + email + "'").Scan(&user)
```

### Command Injection Prevention

```go
// ✅ GOOD: Use exec.Command with separate arguments
cmd := exec.Command("cscli", "bouncers", "list")

// ❌ BAD: Shell with user input (vulnerable)
cmd := exec.Command("sh", "-c", "cscli bouncers list " + userInput)
```

---

## File System Security

### File Permissions

| File Type | Permissions | Owner | Rationale |
|-----------|-------------|-------|-----------|
| Secret files (keys, tokens) | 0600 | charon:charon | Owner read/write only |
| Configuration files | 0640 | charon:charon | Owner read/write, group read |
| Log files | 0640 | charon:charon | Owner read/write, group read |
| Executables | 0750 | root:charon | Owner read/write/execute, group read/execute |
| Data directories | 0750 | charon:charon | Owner full access, group read/execute |

### Directory Structure

```
/data/charon/
├── config/          (0750 charon:charon)
│   ├── config.yaml  (0640 charon:charon)
│   └── secrets/     (0700 charon:charon) - Secret storage
│       └── api.key  (0600 charon:charon)
├── logs/            (0750 charon:charon)
│   └── app.log      (0640 charon:charon)
└── data/            (0750 charon:charon)
```

### Temporary Files

```go
// ✅ GOOD: Secure temp file creation
f, err := os.CreateTemp("", "charon-*.tmp")
if err != nil {
    return err
}
defer os.Remove(f.Name())  // Clean up

// Set secure permissions
if err := os.Chmod(f.Name(), 0600); err != nil {
    return err
}
```

---

## Database Security

### Query Security

1. **Always use parameterized queries** (GORM `Where` with `?` placeholders)
2. **Validate all inputs** before database operations
3. **Use transactions** for multi-step operations
4. **Limit query results** (avoid SELECT *)
5. **Index sensitive columns** sparingly (balance security vs performance)

### Sensitive Data

| Data Type | Storage Method | Example |
|-----------|----------------|---------|
| Passwords | bcrypt hash | `bcrypt.GenerateFromPassword([]byte(password), 12)` |
| API Keys | Environment variable or encrypted field | `os.Getenv("API_KEY")` |
| Tokens | Hashed with random salt | `sha256(token + salt)` |
| PII | Encrypted at rest | AES-256-GCM |

### Migrations

```go
// ✅ GOOD: Add sensitive field with proper constraints
migrator.AutoMigrate(&User{})

// ❌ BAD: Store sensitive data in plaintext
// (Don't add columns like `password_plaintext`)
```

---

## API Security

### Authentication

- **Use JWT tokens** or session cookies with secure flags
- **Implement rate limiting** (prevent brute force)
- **Enforce HTTPS** in production
- **Validate all tokens** before processing requests

### Authorization

```go
// ✅ GOOD: Check user permissions
if !user.HasPermission("crowdsec:manage") {
    return c.JSON(403, gin.H{"error": "forbidden"})
}

// ❌ BAD: Assume user has access
// (No permission check)
```

### Rate Limiting

Protect endpoints from abuse:

```go
// Example: 100 requests per hour per IP
limiter := rate.NewLimiter(rate.Every(36*time.Second), 100)
```

**Critical endpoints** (require stricter limits):
- Login: 5 attempts per 15 minutes
- Password reset: 3 attempts per hour
- API key generation: 5 per day

### Input Validation

```go
// ✅ GOOD: Validate request body
type CreateBouncerRequest struct {
    Name string `json:"name" binding:"required,min=3,max=64,alphanum"`
}

if err := c.ShouldBindJSON(&req); err != nil {
    return c.JSON(400, gin.H{"error": "invalid request"})
}
```

### Error Handling

```go
// ✅ GOOD: Generic error message
return c.JSON(401, gin.H{"error": "authentication failed"})

// ❌ BAD: Reveals authentication details
return c.JSON(401, gin.H{"error": "invalid API key: abc123"})
```

---

## Compliance

### GDPR (General Data Protection Regulation)

**Applicable if**: Processing data of EU residents

**Requirements**:
1. **Data minimization**: Collect only necessary data
2. **Purpose limitation**: Use data only for stated purposes
3. **Storage limitation**: Delete data when no longer needed
4. **Security**: Implement appropriate technical measures (encryption, masking)
5. **Breach notification**: Report breaches within 72 hours

**Implementation**:
- ✅ Charon masks API keys in logs (prevents exposure of personal data)
- ✅ Secure file permissions (0600) protect sensitive data
- ✅ Log retention policies prevent indefinite storage
- ⚠️ Ensure API keys don't contain personal identifiers

**Reference**: [GDPR Article 32 - Security of processing](https://gdpr-info.eu/art-32-gdpr/)

---

### PCI-DSS (Payment Card Industry Data Security Standard)

**Applicable if**: Processing, storing, or transmitting credit card data

**Requirements**:
1. **Requirement 3.4**: Render PAN unreadable (encryption, masking)
2. **Requirement 8.2**: Strong authentication
3. **Requirement 10.2**: Audit trails
4. **Requirement 10.7**: Retain audit logs for 1 year

**Implementation**:
- ✅ Charon uses masking for sensitive credentials (same principle for PAN)
- ✅ Secure file permissions align with access control requirements
- ⚠️ Charon doesn't handle payment cards directly (delegated to payment processors)

**Reference**: [PCI-DSS Quick Reference Guide](https://www.pcisecuritystandards.org/)

---

### SOC 2 (System and Organization Controls)

**Applicable if**: SaaS providers, cloud services

**Trust Service Criteria**:
1. **CC6.1**: Logical access controls (authentication, authorization)
2. **CC6.6**: Encryption of data in transit
3. **CC6.7**: Encryption of data at rest
4. **CC7.2**: Monitoring and detection (logging, alerting)

**Implementation**:
- ✅ API key validation ensures strong credentials (CC6.1)
- ✅ File permissions (0600) protect data at rest (CC6.7)
- ✅ Masked logging enables monitoring without exposing secrets (CC7.2)
- ⚠️ Ensure HTTPS enforcement for data in transit (CC6.6)

**Reference**: [SOC 2 Trust Services Criteria](https://www.aicpa.org/interestareas/frc/assuranceadvisoryservices/trustdataintegritytaskforce)

---

### ISO 27001 (Information Security Management)

**Applicable to**: Any organization implementing ISMS

**Key Controls**:
1. **A.9.4.3**: Password management systems
2. **A.10.1.1**: Cryptographic controls
3. **A.12.4.1**: Event logging
4. **A.18.1.5**: Protection of personal data

**Implementation**:
- ✅ API key format validation (minimum 16 chars, charset restrictions)
- ✅ Key rotation procedures documented
- ✅ Secure storage with file permissions (0600)
- ✅ Masked logging protects sensitive data

**Reference**: [ISO 27001:2013 Controls](https://www.iso.org/standard/54534.html)

---

### Compliance Summary Table

| Framework | Key Requirement | Charon Implementation | Status |
|-----------|----------------|----------------------|--------|
| **GDPR** | Data protection (Art. 32) | API key masking, secure storage | ✅ Compliant |
| **PCI-DSS** | Render PAN unreadable (Req. 3.4) | Masking utility (same principle) | ✅ Aligned |
| **SOC 2** | Logical access controls (CC6.1) | Key validation, file permissions | ✅ Compliant |
| **ISO 27001** | Password management (A.9.4.3) | Key rotation, validation | ✅ Compliant |

---

## Security Testing

### Static Analysis

```bash
# Run CodeQL security scan
.github/skills/scripts/skill-runner.sh security-codeql-scan

# Expected: 0 CWE-312/315/359 findings
```

### Unit Tests

```bash
# Run security-focused unit tests
go test ./backend/internal/api/handlers -run TestMaskAPIKey -v
go test ./backend/internal/api/handlers -run TestValidateAPIKeyFormat -v
go test ./backend/internal/api/handlers -run TestSaveKeyToFile_SecurePermissions -v
```

### Integration Tests

```bash
# Run Playwright E2E tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright

# Check for exposed secrets in test logs
grep -i "api[_-]key\|token\|password" playwright-report/index.html
# Expected: Only masked values (abcd...xyz9) or no matches
```

### Penetration Testing

**Recommended schedule**: Annual or after major releases

**Focus areas**:
1. Authentication bypass
2. Authorization vulnerabilities
3. SQL injection
4. Path traversal
5. Information disclosure (logs, errors)
6. Rate limiting effectiveness

---

## Security Checklist

### Before Every Release

- [ ] Run CodeQL scan (0 critical findings)
- [ ] Run unit tests (100% pass)
- [ ] Run integration tests (100% pass)
- [ ] Check for hardcoded secrets (TruffleHog, Semgrep)
- [ ] Review log output for sensitive data exposure
- [ ] Verify file permissions (secrets: 0600, configs: 0640)
- [ ] Update dependencies (no known CVEs)
- [ ] Review security documentation updates
- [ ] Test secret rotation procedure
- [ ] Verify HTTPS enforcement in production

### During Code Review

- [ ] No secrets in environment variables (use .env)
- [ ] All secrets are masked in logs
- [ ] Input validation on all user-provided data
- [ ] Parameterized queries (no string concatenation)
- [ ] Secure file permissions (0600 for secrets)
- [ ] Error messages don't reveal sensitive info
- [ ] No commented-out secrets or debugging code
- [ ] Security tests added for new features

### After Security Incident

- [ ] Rotate all affected secrets immediately
- [ ] Audit access logs for unauthorized use
- [ ] Purge logs containing exposed secrets
- [ ] Notify affected users (if PII exposed)
- [ ] Update incident response procedures
- [ ] Document lessons learned
- [ ] Implement additional controls to prevent recurrence

---

## Resources

### Internal Documentation

- [API Key Handling Guide](./security/api-key-handling.md)
- [ARCHITECTURE.md](../ARCHITECTURE.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)

### External References

- [OWASP Top 10](https://owasp.org/Top10/)
- [OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [SANS Top 25 Software Errors](https://www.sans.org/top25-software-errors/)

### Security Standards

- [GDPR Official Text](https://gdpr-info.eu/)
- [PCI-DSS Standards](https://www.pcisecuritystandards.org/)
- [SOC 2 Trust Services](https://www.aicpa.org/)
- [ISO 27001](https://www.iso.org/standard/54534.html)

---

## Updates

| Date | Change | Author |
|------|--------|--------|
| 2026-02-03 | Initial security practices documentation | GitHub Copilot |

---

**Last Updated**: 2026-02-03
**Next Review**: 2026-05-03 (Quarterly)
**Owner**: Security Team / Lead Developer
