# CodeQL `go/email-injection` Remediation - Complete

**Date**: 2026-01-10
**Status**: ✅ RESOLVED

## Summary

Successfully remediated the CodeQL `go/email-injection` finding by implementing proper email header separation according to security best practices outlined in the specification.

## Changes Implemented

### 1. Helper Functions Added to `backend/internal/services/mail_service.go`

#### `encodeSubject(subject string) (string, error)`

- Trims whitespace from subject lines
- Rejects any CR/LF characters to prevent header injection
- Uses MIME Q-encoding (RFC 2047) for UTF-8 subject lines
- Returns encoded subject suitable for email headers

#### `toHeaderUndisclosedRecipients() string`

- Returns constant `"undisclosed-recipients:;"` for RFC 5322 To: header
- Prevents request-derived email addresses from appearing in message headers
- Eliminates the CodeQL-detected taint flow from user input to SMTP message

### 2. Modified `buildEmail()` Function

**Key Security Changes:**

- Changed `To:` header to use `toHeaderUndisclosedRecipients()` instead of request-derived recipient address
- Recipient validation still performed for SMTP envelope (RCPT TO command)
- Subject encoding enforced through `encodeSubject()` helper
- Updated security documentation comments

**Critical Implementation Detail:**

- SMTP envelope recipients (`toEnvelope` in `smtp.SendMail`) remain correct for delivery
- Only RFC 5322 message headers changed
- Separation of envelope routing from message headers eliminates injection risk

### 3. Enhanced Test Coverage

#### New Tests in `backend/internal/services/mail_service_test.go`

1. **`TestMailService_BuildEmail_UndisclosedRecipients`**
   - Verifies `To:` header contains `undisclosed-recipients:;`
   - Asserts recipient email does NOT appear in message headers
   - Prevents regression of CodeQL finding

2. **`TestMailService_SendInvite_HTMLTemplateEscaping`**
   - Tests HTML template auto-escaping for special characters in `appName`
   - Verifies XSS protection in invite emails

#### Updated Tests in `backend/internal/api/handlers/user_handler_test.go`

1. **`TestUserHandler_PreviewInviteURL_Success_Unconfigured`**
   - Updated to verify `base_url` and `preview_url` are empty when `app.public_url` not configured
   - Prevents fallback to request headers

2. **`TestUserHandler_PreviewInviteURL_Unconfigured_DoesNotUseRequestHost`** (NEW)
   - Explicitly tests malicious `Host` header injection scenario
   - Asserts response does NOT contain attacker-controlled hostname
   - Verifies protection against host header poisoning attacks

## Verification Results

### Test Results ✅

```bash
cd /projects/Charon/backend/internal/services
go test -v -run "TestMail" .
```

**Result**: All mail service tests PASS

- Total mail service tests: 28 tests
- New security tests: 2 added
- Updated tests: 1 modified
- Coverage: 81.1% of statements (services package)

### CodeQL Scan Results ✅

```bash
codeql database analyze codeql-db-go \
  --format=sarif-latest \
  --output=codeql-results-go.sarif
```

**Result**:

- Total findings: 0
- `go/email-injection` findings: 0 (RESOLVED)
- Previous finding location: `backend/internal/services/mail_service.go:285`
- Status: **No longer detected**

## Security Impact

### Before Remediation

- Request-derived email addresses flowed into RFC 5322 message headers
- CodeQL identified potential for content spoofing (CWE-640)
- Malicious recipient addresses could theoretically manipulate headers
- Risk: Low (existing CRLF rejection mitigated most attacks, but CodeQL flagged it)

### After Remediation

- **Zero request-derived data** in message headers
- `To:` header uses RFC-compliant constant: `undisclosed-recipients:;`
- SMTP envelope routing unchanged (still uses validated recipient)
- Subject lines properly MIME-encoded
- Multiple layers of defense:
  1. CRLF rejection (existing)
  2. MIME encoding (new)
  3. Header isolation (new)
  4. Dot-stuffing (existing)

### Additional Protections

- Host header injection prevented in invite URL generation
- HTML template auto-escaping verified
- Comprehensive test coverage for injection scenarios

## Files Modified

1. **`backend/internal/services/mail_service.go`**
   - Added: `encodeSubject()`, `toHeaderUndisclosedRecipients()`
   - Modified: `SendEmail()`, `buildEmail()`
   - Lines changed: ~30

2. **`backend/internal/services/mail_service_test.go`**
   - Added: 2 new security-focused tests
   - Modified: 1 existing test
   - Lines changed: ~50

3. **`backend/internal/api/handlers/user_handler_test.go`**
   - Added: 1 new host header injection test
   - Modified: 1 existing test
   - Lines changed: ~20

## Compliance

- ✅ OWASP Top 10 2021: A03:2021 – Injection
- ✅ CWE-93: Improper Neutralization of CRLF Sequences in HTTP Headers
- ✅ CWE-640: Weak Password Recovery Mechanism for Forgotten Password
- ✅ RFC 5321: SMTP (envelope vs. message header separation)
- ✅ RFC 5322: Internet Message Format
- ✅ RFC 2047: MIME Message Header Extensions

## References

- Specification: `docs/plans/current_spec.md`
- CodeQL Query: `go/email-injection`
- Original SARIF: `codeql-results-go.sarif` (prior to remediation)
- Security Instructions: `.github/instructions/security-and-owasp.instructions.md`

## Conclusion

The CodeQL `go/email-injection` vulnerability has been **completely resolved** through proper separation of SMTP envelope routing from RFC 5322 message headers. The implementation follows security best practices, maintains backward compatibility with SMTP delivery, and includes comprehensive test coverage to prevent regression.

**No suppressions were used** - the vulnerability was remediated at the source by eliminating the tainted data flow.
