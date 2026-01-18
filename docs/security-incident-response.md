---
title: Security Incident Response Plan
description: Industry-standard incident response procedures for Charon deployments, including detection, containment, recovery, and post-incident review.
---

## Security Incident Response Plan (SIRP)

This document provides a structured approach to handling security incidents in Charon deployments. Following these procedures ensures consistent, effective responses that minimize damage and recovery time.

---

## Incident Classification

### Severity Levels

| Level | Name | Description | Response Time | Examples |
|-------|------|-------------|---------------|----------|
| **P1** | Critical | Active exploitation, data breach, or complete service compromise | Immediate (< 15 min) | Confirmed data exfiltration, ransomware, root access compromise |
| **P2** | High | Attempted exploitation, security control bypass, or significant vulnerability | < 1 hour | WAF bypass detected, brute-force attack in progress, credential stuffing |
| **P3** | Medium | Suspicious activity, minor vulnerability, or policy violation | < 4 hours | Unusual traffic patterns, failed authentication spike, misconfiguration |
| **P4** | Low | Informational security events, minor policy deviations | < 24 hours | Routine blocked requests, scanner traffic, expired certificates |

### Classification Criteria

**Escalate to P1 immediately if:**

- ❌ Confirmed unauthorized access to sensitive data
- ❌ Active malware or ransomware detected
- ❌ Complete loss of security controls
- ❌ Evidence of data exfiltration
- ❌ Critical infrastructure compromise

**Escalate to P2 if:**

- ⚠️ Multiple failed bypass attempts from same source
- ⚠️ Vulnerability actively being probed
- ⚠️ Partial security control failure
- ⚠️ Credential compromise suspected

---

## Detection Methods

### Automated Detection

**Cerberus Security Dashboard:**

1. Navigate to **Cerberus → Dashboard**
2. Monitor the **Live Activity** section for real-time events
3. Review **Security → Decisions** for blocked requests
4. Check alert notifications (Discord, Slack, email)

**Key Indicators to Monitor:**

- Sudden spike in blocked requests
- Multiple blocks from same IP/network
- WAF rules triggering on unusual patterns
- CrowdSec decisions for known threat actors
- Rate limiting thresholds exceeded

**Log Analysis:**

```bash
# View recent security events
docker logs charon | grep -E "(BLOCK|DENY|ERROR)" | tail -100

# Check CrowdSec decisions
docker exec charon cscli decisions list

# Review WAF activity
docker exec charon cat /var/log/coraza-waf.log | tail -50
```

### Manual Detection

**Regular Security Reviews:**

- [ ] Weekly review of Cerberus Dashboard
- [ ] Monthly review of access patterns
- [ ] Quarterly penetration testing
- [ ] Annual security audit

---

## Containment Procedures

### Immediate Actions (All Severity Levels)

1. **Document the incident start time**
2. **Preserve evidence** — Do NOT restart containers until logs are captured
3. **Assess scope** — Determine affected systems and data

### P1/P2 Containment

**Step 1: Isolate the Threat**

```bash
# Block attacking IP immediately
docker exec charon cscli decisions add --ip <ATTACKER_IP> --duration 720h --reason "Incident response"

# If compromise confirmed, stop the container
docker stop charon

# Preserve container state for forensics
docker commit charon charon-incident-$(date +%Y%m%d%H%M%S)
```

**Step 2: Preserve Evidence**

```bash
# Export all logs
docker logs charon > /tmp/incident-logs-$(date +%Y%m%d%H%M%S).txt 2>&1

# Export CrowdSec decisions
docker exec charon cscli decisions list -o json > /tmp/crowdsec-decisions.json

# Copy data directory
cp -r ./charon-data /tmp/incident-backup-$(date +%Y%m%d%H%M%S)
```

**Step 3: Notify Stakeholders**

- System administrators
- Security team (if applicable)
- Management (P1 only)
- Legal/compliance (if data breach)

### P3/P4 Containment

1. Block offending IPs via Cerberus Dashboard
2. Review and update access lists if needed
3. Document the event in incident log
4. Continue monitoring

---

## Recovery Steps

### Pre-Recovery Checklist

- [ ] Incident fully contained
- [ ] Evidence preserved
- [ ] Root cause identified (or investigation ongoing)
- [ ] Clean backups available

### Recovery Procedure

**Step 1: Verify Backup Integrity**

```bash
# List available backups
ls -la ./charon-data/backups/

# Verify backup can be read
docker run --rm -v ./charon-data/backups:/backups debian:bookworm-slim ls -la /backups
```

**Step 2: Restore from Clean State**

```bash
# Stop compromised instance
docker stop charon

# Rename compromised data
mv ./charon-data ./charon-data-compromised-$(date +%Y%m%d)

# Restore from backup
cp -r ./charon-data-backup-YYYYMMDD ./charon-data

# Start fresh instance
docker-compose up -d
```

**Step 3: Apply Security Hardening**

1. Review and update all access lists
2. Rotate any potentially compromised credentials
3. Update Charon to latest version
4. Enable additional security features if not already active

**Step 4: Verify Recovery**

```bash
# Check Charon is running
docker ps | grep charon

# Verify LAPI status
docker exec charon cscli lapi status

# Test proxy functionality
curl -I https://your-proxied-domain.com
```

### Communication During Recovery

- Update stakeholders every 30 minutes (P1) or hourly (P2)
- Document all recovery actions taken
- Prepare user communication if service was affected

---

## Post-Incident Review

### Review Meeting Agenda

Schedule within 48 hours of incident resolution.

**Attendees:** All involved responders, system owners, management (P1/P2)

**Agenda:**

1. Incident timeline reconstruction
2. What worked well?
3. What could be improved?
4. Action items and owners
5. Documentation updates needed

### Post-Incident Checklist

- [ ] Incident fully documented
- [ ] Timeline created with all actions taken
- [ ] Root cause analysis completed
- [ ] Lessons learned documented
- [ ] Security controls reviewed and updated
- [ ] Monitoring/alerting improved
- [ ] Team training needs identified
- [ ] Documentation updated

### Incident Report Template

```markdown
## Incident Report: [INCIDENT-YYYY-MM-DD-###]

**Severity:** P1/P2/P3/P4
**Status:** Resolved / Under Investigation
**Duration:** [Start Time] to [End Time]

### Summary
[Brief description of what happened]

### Timeline
- [HH:MM] - Event detected
- [HH:MM] - Containment initiated
- [HH:MM] - Root cause identified
- [HH:MM] - Recovery completed

### Impact
- Systems affected: [List]
- Data affected: [Yes/No, details]
- Users affected: [Count/scope]
- Service downtime: [Duration]

### Root Cause
[Technical explanation of what caused the incident]

### Actions Taken
1. [Action 1]
2. [Action 2]
3. [Action 3]

### Lessons Learned
- [Learning 1]
- [Learning 2]

### Follow-up Actions
| Action | Owner | Due Date | Status |
|--------|-------|----------|--------|
| [Action] | [Name] | [Date] | Open |
```

---

## Communication Templates

### Internal Notification (P1/P2)

```
SECURITY INCIDENT ALERT

Severity: [P1/P2]
Time Detected: [YYYY-MM-DD HH:MM UTC]
Status: [Active / Contained / Resolved]

Summary:
[Brief description]

Current Actions:
- [Action being taken]

Next Update: [Time]

Contact: [Incident Commander Name/Channel]
```

### User Communication (If Service Affected)

```
Service Notification

We are currently experiencing [brief issue description].

Status: [Investigating / Identified / Monitoring / Resolved]
Started: [Time]
Expected Resolution: [Time or "Under investigation"]

We apologize for any inconvenience and will provide updates as available.

Last Updated: [Time]
```

### Post-Incident Summary (External)

```
Security Incident Summary

On [Date], we identified and responded to a security incident affecting [scope].

What happened:
[Non-technical summary]

What we did:
[Response actions taken]

What we're doing to prevent this:
[Improvements being made]

Was my data affected?
[Clear statement about data impact]

Questions?
[Contact information]
```

---

## Emergency Contacts

Maintain an up-to-date contact list:

| Role | Contact Method | Escalation Time |
|------|----------------|-----------------|
| Primary On-Call | [Phone/Pager] | Immediate |
| Security Team | [Email/Slack] | < 15 min (P1/P2) |
| System Administrator | [Phone] | < 1 hour |
| Management | [Phone] | P1 only |

---

## Quick Reference Card

### P1 Critical — Immediate Response

1. ⏱️ Start timer, document everything
2. 🔒 Isolate: `docker stop charon`
3. 📋 Preserve: `docker logs charon > incident.log`
4. 📞 Notify: Security team, management
5. 🔍 Investigate: Determine scope and root cause
6. 🔧 Recover: Restore from clean backup
7. 📝 Review: Post-incident meeting within 48h

### P2 High — Urgent Response

1. 🔒 Block attacker: `cscli decisions add --ip <IP>`
2. 📋 Capture logs before they rotate
3. 📞 Notify: Security team
4. 🔍 Investigate root cause
5. 🔧 Apply fixes
6. 📝 Document and review

### Key Commands

```bash
# Block IP immediately
docker exec charon cscli decisions add --ip <IP> --duration 720h

# List all active blocks
docker exec charon cscli decisions list

# Export logs
docker logs charon > incident-$(date +%s).log 2>&1

# Check security status
docker exec charon cscli lapi status
```

---

## Document Maintenance

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-21 | Security Team | Initial SIRP creation |

**Review Schedule:** Quarterly or after any P1/P2 incident

**Owner:** Security Team

**Last Reviewed:** 2025-12-21

```
