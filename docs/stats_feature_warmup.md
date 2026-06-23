# Enhanced Dashboard Statistics - Deployment & Warmup Guide

## Overview
The enhanced statistics feature (Issue #25) adds real-time traffic analysis to the dashboard. When deployed, users will see new widgets:
- **Traffic Volume** - Bytes sent over time
- **Top Hosts** - Most frequently accessed domains
- **Status Distribution** - HTTP status code breakdown
- **Request Counts** - Total requests over rolling windows

## Important: Warmup Period

When users upgrade to this version, **the statistics widgets will be empty initially**. This is expected behavior, not a bug.

### Why?
- Request logs are stored in a new `request_logs` database table
- The table is created automatically on first boot (via GORM AutoMigrate)
- Historical requests are not retroactively logged
- Data collection begins immediately after deployment

### Timeline
Data will populate based on traffic and the selected bucket:

| Bucket | Lookback Window | Time to Full Graph |
|--------|-----------------|-------------------|
| 1h     | 30 hours        | ~30 hours         |
| 6h     | 180 hours       | ~7.5 days         |
| 1d     | 30 days         | ~30 days          |

Users with active proxies will see:
- **First hour**: Initial data points appearing
- **First day**: Clear traffic patterns emerging
- **First week**: Comprehensive statistics with trends

## For System Administrators

### Deployment Checklist
1. ✅ Update Charon to latest version
2. ✅ Restart the backend (AutoMigrate runs automatically)
3. ✅ Verify LogWatcher started: Check logs for "Security log watcher started - stats collection enabled"
4. ✅ Inform users that stats widgets will populate over time
5. ✅ Check Stats Health endpoint: `GET /api/v1/stats/health` → should show `{"dropped_count": 0}`

### Verification
After deployment, verify data collection is working:

```bash
# Check request logs are being created
sqlite3 /path/to/data/charon.db "SELECT COUNT(*) FROM request_logs;"

# Monitor LogWatcher startup logs
docker logs <charon-container> | grep "stats collection"
```

## For Users

When you upgrade:

1. **Stats widgets will be empty** - This is normal
2. **Data collection starts immediately** - Each request to your proxies is logged
3. **Check back soon** - Data will appear within hours as traffic flows
4. **Full picture takes time** - 30-day view needs 30 days of data

### Example Timeline
- **Now**: Deploy Charon with stats feature
- **In 1 hour**: Traffic Volume shows first data points (1h bucket)
- **In 1 day**: Clear 24h trends visible
- **In 7 days**: Weekly patterns visible (6h bucket)
- **In 30 days**: Full 30-day statistics available (1d bucket)

## Technical Details

### Data Collection Pipeline
```
Caddy Access Logs
    ↓
LogWatcher (parses JSON logs)
    ↓
StatsIngester (batches writes every 500ms)
    ↓
SQLite request_logs table
    ↓
Stats API (aggregates on-demand)
    ↓
Dashboard (displays with appropriate messaging)
```

### No Data Loss
- Requests are batched and flushed every 500ms
- 100-request batch flush ensures no data loss during shutdown
- Graceful context cancellation drains remaining entries
- Database backup includes all request logs

### UI Messaging
When no data is available:
- Dashboard displays: "No data available yet"
- Helper text: "Data is being collected. Check back in a few hours..."
- Tooltip adds: "Data may take several hours to populate depending on traffic volume"

## Troubleshooting

### "No data available" persists after 24 hours
1. Check that LogWatcher is running: `curl localhost:5000/api/v1/stats/health`
2. Verify access log path is correct: Check `CHARON_CADDY_ACCESS_LOG` env var
3. Confirm Caddy is logging: Check `/var/log/caddy/access.log` has recent entries
4. Check database: `SELECT COUNT(*) FROM request_logs;` should be > 0

### Stats ingestion dropping entries
- Check: `GET /api/v1/stats/health` response
- If `dropped_count > 0`, the ingester channel buffer overflowed
- Increase `channelBufferSize` in stats_ingester.go or reduce traffic
- This is rare and indicates extreme load

### Empty request logs
Verify all components are running:
1. LogWatcher: Logs show "stats collection enabled"
2. StatsIngester: `Go routines alive, processing incoming entries`
3. Database: `SELECT COUNT(*) FROM request_logs;` > 0

## Migration Notes

- ✅ Automatic: No manual SQL needed
- ✅ Safe: Doesn't affect existing data
- ✅ Non-blocking: Won't delay server startup
- ✅ Backward compatible: Old instances continue working
