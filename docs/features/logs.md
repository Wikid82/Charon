---
title: Real-Time Logs
description: Watch requests flow through your proxy in real-time
---

# Real-Time Logs

Watch requests flow through your proxy in real-time. Filter by domain, status code, or time range to troubleshoot issues quickly. All the visibility you need without diving into container logs.

## Overview

Charon provides real-time log streaming via WebSocket, giving you instant visibility into all proxy traffic and security events. The logging system includes two main views:

- **Access Logs**: All HTTP requests flowing through Caddy
- **Security Logs**: Cerberus Dashboard showing CrowdSec decisions and WAF events

Logs stream directly to your browser with minimal latency, eliminating the need to SSH into containers or parse log files manually.

## Why Use This

- **Instant Troubleshooting**: See requests as they happen to diagnose issues in real-time
- **Security Monitoring**: Watch for blocked threats and suspicious activity
- **No CLI Required**: Everything accessible through the web interface
- **Persistent Connection**: WebSocket keeps the stream open without polling

## Log Viewer Controls

The log viewer provides intuitive controls for managing the log stream:

| Control | Function |
|---------|----------|
| **Pause/Resume** | Temporarily stop the stream to examine specific entries |
| **Clear** | Remove all displayed logs (doesn't affect server logs) |
| **Auto-scroll** | Automatically scroll to newest entries (toggle on/off) |

## Filtering Options

Filter logs to focus on what matters:

- **Level**: Filter by severity (info, warning, error)
- **Source**: Filter by service (caddy, crowdsec, cerberus)
- **Text Search**: Free-text search across all log fields
- **Time Range**: View logs from specific time periods

### Server-Side Query Parameters

For advanced filtering, use query parameters when connecting:

```text
/api/logs/stream?level=error&source=crowdsec&limit=1000
```

## WebSocket Connection

The log viewer displays connection status in the header:

- **Connected**: Green indicator, logs streaming
- **Reconnecting**: Yellow indicator, automatic retry in progress
- **Disconnected**: Red indicator, manual reconnect available

### Troubleshooting Connection Issues

If the WebSocket disconnects frequently:

1. Check browser console for errors
2. Verify no proxy is blocking WebSocket upgrades
3. Ensure the Charon container has sufficient resources
4. Check for network timeouts on long-idle connections

## Browsing Saved Log Files

Want to look at older logs instead of the live stream? Open the **Tasks → Logs** page. It shows saved log files in an easy-to-read table:

- **Sort with a click** — Click a column header (Time, Level, Status, Method, or Path) to sort the table. Click it again to flip the order.
- **Level at a glance** — A Level column shows how serious each entry is (info, warning, or error) without reading the whole line.
- **Search and filter** — Type in the search box or pick a level to narrow things down, and use the page controls to move through long files.
- **One-click download** — Save a log file to your computer. If a file can't be downloaded, you'll see a friendly message explaining what happened instead of a blank page.
- **Handles messy files** — If a log file is huge or partly damaged, the viewer still shows everything it can read, along with a small notice telling you how many unreadable lines were skipped.

## Related

- [WebSocket Support](websocket.md)
- [CrowdSec Integration](crowdsec.md)
- [Back to Features](../features.md)
