# Uptime Monitoring

Charon's uptime monitoring system continuously checks the availability of your proxy hosts and alerts you when issues occur. The system is designed to minimize false positives while quickly detecting real problems.

## Overview

Uptime monitoring performs automated health checks on your proxy hosts at regular intervals, tracking:

- **Host availability** (TCP connectivity)
- **Response times** (latency measurements)
- **Status history** (uptime/downtime tracking)
- **Failure patterns** (debounced detection)

## How It Works

### Check Cycle

1. **Scheduled Checks**: Every 60 seconds (default), Charon checks all enabled hosts
2. **Port Detection**: Uses the proxy host's `ForwardPort` for TCP checks
3. **Connection Test**: Attempts TCP connection with configurable timeout
4. **Status Update**: Records success/failure in database
5. **Notification Trigger**: Sends alerts on status changes (if configured)

### Failure Debouncing

To prevent false alarms from transient network issues, Charon uses **failure debouncing**:

**How it works:**

- A host must **fail 2 consecutive checks** before being marked "down"
- Single failures are logged but don't trigger status changes
- Counter resets immediately on any successful check

**Why this matters:**

- Network hiccups don't cause false alarms
- Container restarts don't trigger unnecessary alerts
- Transient DNS issues are ignored
- You only get notified about real problems

**Example scenario:**

```
Check 1: ✅ Success → Status: Up, Failure Count: 0
Check 2: ❌ Failed  → Status: Up, Failure Count: 1  (no alert)
Check 3: ❌ Failed  → Status: Down, Failure Count: 2 (alert sent!)
Check 4: ✅ Success → Status: Up, Failure Count: 0  (recovery alert)
```

## Configuration

### Timeout Settings

**Default TCP timeout:** 10 seconds

This timeout determines how long Charon waits for a TCP connection before considering it failed.

**Increase timeout if:**

- You have slow networks
- Hosts are geographically distant
- Containers take time to warm up
- You see intermittent false "down" alerts

**Decrease timeout if:**

- You want faster failure detection
- Your hosts are on local network
- Response times are consistently fast

**Note:** Timeout settings are currently set in the backend configuration. A future release will make this configurable via the UI.

### Retry Behavior

When a check fails, Charon automatically retries:

- **Max retries:** 2 attempts
- **Retry delay:** 2 seconds between attempts
- **Timeout per attempt:** 10 seconds (configurable)

**Total check time calculation:**

```
Max time = (timeout × max_retries) + (retry_delay × (max_retries - 1))
         = (10s × 2) + (2s × 1)
         = 22 seconds worst case
```

### Check Interval

**Default:** 60 seconds

The interval between check cycles for all hosts.

**Performance considerations:**

- Shorter intervals = faster detection but higher CPU/network usage
- Longer intervals = lower overhead but slower failure detection
- Recommended: 30-120 seconds depending on criticality

## Enabling Uptime Monitoring

### For a Single Host

1. Navigate to **Proxy Hosts**
2. Click **Edit** on the host
3. Scroll to **Uptime Monitoring** section
4. Toggle **"Enable Uptime Monitoring"** to ON
5. Click **Save**

### For Multiple Hosts (Bulk)

1. Navigate to **Proxy Hosts**
2. Select checkboxes for hosts to monitor
3. Click **"Bulk Apply"** button
4. Find **"Uptime Monitoring"** section
5. Toggle the switch to **ON**
6. Check **"Apply to selected hosts"**
7. Click **"Apply Changes"**

## Monitoring Dashboard

### Host Status Display

Each monitored host shows:

- **Status Badge**: 🟢 Up / 🔴 Down
- **Response Time**: Last successful check latency
- **Uptime Percentage**: Success rate over time
- **Last Check**: Timestamp of most recent check

### Status Page

View all monitored hosts at a glance:

1. Navigate to **Dashboard** → **Uptime Status**
2. See real-time status of all hosts
3. Click any host for detailed history
4. Filter by status (up/down/all)

## Troubleshooting

### False Positive: Host Shown as Down but Actually Up

**Symptoms:**

- Host shows "down" in Charon
- Service is accessible directly
- Status changes back to "up" shortly after

**Common causes:**

1. **Timeout too short for slow network**

   **Solution:** Increase TCP timeout in configuration

2. **Container warmup time exceeds timeout**

   **Solution:** Use longer timeout or optimize container startup

3. **Network congestion during check**

   **Solution:** Debouncing (already enabled) should handle this automatically

4. **Firewall blocking health checks**

   **Solution:** Ensure Charon container can reach proxy host ports

5. **Multiple checks running concurrently**

   **Solution:** Automatic synchronization ensures checks complete before next cycle

**Diagnostic steps:**

```bash
# Check Charon logs for timing info
docker logs charon 2>&1 | grep "Host TCP check completed"

# Look for retry attempts
docker logs charon 2>&1 | grep "Retrying TCP check"

# Check failure count patterns
docker logs charon 2>&1 | grep "failure_count"

# View host status changes
docker logs charon 2>&1 | grep "Host status changed"
```

### False Negative: Host Shown as Up but Actually Down

**Symptoms:**

- Host shows "up" in Charon
- Service returns errors or is inaccessible
- No down alerts received

**Common causes:**

1. **TCP port open but service not responding**

   **Explanation:** Uptime monitoring only checks TCP connectivity, not application health

   **Solution:** Consider implementing application-level health checks (future feature)

2. **Service accepts connections but returns errors**

   **Solution:** Monitor application logs separately; TCP checks don't validate responses

3. **Partial service degradation**

   **Solution:** Use multiple monitoring providers for critical services

**Current limitation:** Charon performs TCP health checks only. HTTP-based health checks are planned for a future release.

### Intermittent Status Flapping

**Symptoms:**

- Status rapidly changes between up/down
- Multiple notifications in short time
- Logs show alternating success/failure

**Causes:**

1. **Marginal network conditions**

   **Solution:** Increase failure threshold (requires configuration change)

2. **Resource exhaustion on target host**

   **Solution:** Investigate target host performance, increase resources

3. **Shared network congestion**

   **Solution:** Consider dedicated monitoring network or VLAN

**Mitigation:**

The built-in debouncing (2 consecutive failures required) should prevent most flapping. If issues persist, check:

```bash
# Review consecutive check results
docker logs charon 2>&1 | grep -A 2 "Host TCP check completed" | grep "host_name"

# Check response time trends
docker logs charon 2>&1 | grep "elapsed_ms"
```

### No Notifications Received

**Checklist:**

1. ✅ Uptime monitoring is enabled for the host
2. ✅ Notification provider is configured and enabled
3. ✅ Provider is set to trigger on uptime events
4. ✅ Status has actually changed (check logs)
5. ✅ Debouncing threshold has been met (2 consecutive failures)

**Debug notifications:**

```bash
# Check for notification attempts
docker logs charon 2>&1 | grep "notification"

# Look for uptime-related notifications
docker logs charon 2>&1 | grep "uptime_down\|uptime_up"

# Verify notification service is working
docker logs charon 2>&1 | grep "Failed to send notification"
```

### High CPU Usage from Monitoring

**Symptoms:**

- Charon container using excessive CPU
- System becomes slow during check cycles
- Logs show slow check times

**Solutions:**

1. **Reduce number of monitored hosts**

   Monitor only critical services; disable monitoring for non-essential hosts

2. **Increase check interval**

   Change from 60s to 120s to reduce frequency

3. **Optimize Docker resource allocation**

   Ensure adequate CPU/memory allocated to Charon container

4. **Check for network issues**

   Slow DNS or network problems can cause checks to hang

**Monitor check performance:**

```bash
# View check duration distribution
docker logs charon 2>&1 | grep "elapsed_ms" | tail -50

# Count concurrent checks
docker logs charon 2>&1 | grep "All host checks completed"
```

## Advanced Topics

### Port Detection

Charon automatically determines which port to check:

**Priority order:**

1. **ProxyHost.ForwardPort**: Preferred, most reliable
2. **URL extraction**: Fallback for hosts without proxy configuration
3. **Default ports**: 80 (HTTP) or 443 (HTTPS) if port not specified

**Example:**

```
Host: example.com
Forward Port: 8080
→ Checks: example.com:8080

Host: api.example.com
URL: https://api.example.com/health
Forward Port: (not set)
→ Checks: api.example.com:443
```

### Concurrent Check Processing

All host checks run concurrently for better performance:

- Each host checked in separate goroutine
- WaitGroup ensures all checks complete before next cycle
- Prevents database race conditions
- No single slow host blocks other checks

**Performance characteristics:**

- **Sequential checks** (old): `time = hosts × timeout`
- **Concurrent checks** (current): `time = max(individual_check_times)`

**Example:** With 10 hosts and 10s timeout:

- Sequential: ~100 seconds minimum
- Concurrent: ~10 seconds (if all succeed on first try)

### Database Storage

Uptime data is stored efficiently:

**UptimeHost table:**

- `status`: Current status ("up"/"down")
- `failure_count`: Consecutive failure counter
- `last_check`: Timestamp of last check
- `response_time`: Last successful response time

**UptimeMonitor table:**

- Links monitors to proxy hosts
- Stores check configuration
- Tracks enabled state

**Heartbeat records** (future):

- Detailed history of each check
- Used for uptime percentage calculations
- Queryable for historical analysis

## Best Practices

### 1. Monitor Critical Services Only

Don't monitor every host. Focus on:

- Production services
- User-facing applications
- External dependencies
- High-availability requirements

**Skip monitoring for:**

- Development/test instances
- Internal tools with built-in redundancy
- Services with their own monitoring

### 2. Configure Appropriate Notifications

**Critical services:**

- Multiple notification channels (Discord + Slack)
- Immediate alerts (no batching)
- On-call team notifications

**Non-critical services:**

- Single notification channel
- Digest/batch notifications (future feature)
- Email to team (low priority)

### 3. Review False Positives

If you receive false alarms:

1. Check logs to understand why
2. Adjust timeout if needed
3. Verify network stability
4. Consider increasing failure threshold (future config option)

### 4. Regular Status Review

Weekly review of:

- Uptime percentages (identify problematic hosts)
- Response time trends (detect degradation)
- Notification frequency (too many alerts?)
- False positive rate (refine configuration)

### 5. Combine with Application Monitoring

Uptime monitoring checks **availability**, not **functionality**.

Complement with:

- Application-level health checks
- Error rate monitoring
- Performance metrics (APM tools)
- User experience monitoring

## Planned Improvements

Future enhancements under consideration:

- [ ] **HTTP health check support** - Check specific endpoints with status code validation
- [ ] **Configurable failure threshold** - Adjust consecutive failure count via UI
- [ ] **Custom check intervals per host** - Different intervals for different criticality levels
- [ ] **Response time alerts** - Notify on degraded performance, not just failures
- [ ] **Notification batching** - Group multiple alerts to reduce noise
- [ ] **Maintenance windows** - Disable alerts during scheduled maintenance
- [ ] **Historical graphs** - Visual uptime trends over time
- [ ] **Status page export** - Public status page for external visibility

## Monitoring the Monitors

How do you know if Charon's monitoring is working?

**Check Charon's own health:**

```bash
# Verify check cycle is running
docker logs charon 2>&1 | grep "All host checks completed" | tail -5

# Confirm recent checks happened
docker logs charon 2>&1 | grep "Host TCP check completed" | tail -20

# Look for any errors in monitoring system
docker logs charon 2>&1 | grep "ERROR.*uptime\|ERROR.*monitor"
```

**Expected log pattern:**

```
INFO[...] All host checks completed host_count=5
DEBUG[...] Host TCP check completed elapsed_ms=156 host_name=example.com success=true
```

**Warning signs:**

- No "All host checks completed" messages in recent logs
- Checks taking longer than expected (>30s with 10s timeout)
- Frequent timeout errors
- High failure_count values

## API Integration

Uptime monitoring data is accessible via API:

**Get uptime status:**

```bash
GET /api/uptime/hosts
Authorization: Bearer <token>
```

**Response:**

```json
{
  "hosts": [
    {
      "id": "123",
      "name": "example.com",
      "status": "up",
      "last_check": "2025-12-24T10:30:00Z",
      "response_time": 156,
      "failure_count": 0,
      "uptime_percentage": 99.8
    }
  ]
}
```

**Programmatic monitoring:**

Use this API to integrate Charon's uptime data with:

- External monitoring dashboards (Grafana, etc.)
- Incident response systems (PagerDuty, etc.)
- Custom alerting tools
- Status page generators

## Additional Resources

- [Notification Configuration Guide](notifications.md)
- [Proxy Host Setup](../getting-started.md)
- [Troubleshooting Guide](../troubleshooting/)
- [Security Best Practices](../security.md)

## Need Help?

- 💬 [Ask in Discussions](https://github.com/Wikid82/charon/discussions)
- 🐛 [Report Issues](https://github.com/Wikid82/charon/issues)
- 📖 [View Full Documentation](https://wikid82.github.io/charon/)
