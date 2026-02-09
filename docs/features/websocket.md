---
title: WebSocket Support
description: Real-time WebSocket connections work out of the box
---

# WebSocket Support

Real-time applications like chat servers, live dashboards, and collaborative tools work out of the box. Charon handles WebSocket connections automatically with no special configuration needed.

## Overview

WebSocket connections enable persistent, bidirectional communication between browsers and servers. Unlike traditional HTTP requests, WebSockets maintain an open connection for real-time data exchange.

Charon automatically detects and handles WebSocket upgrade requests, proxying them to your backend services transparently. This works for any application that uses WebSockets—no special configuration required.

## Why Use This

- **Zero Configuration**: WebSocket proxying works automatically
- **Full Protocol Support**: Handles all WebSocket features including subprotocols
- **Transparent Proxying**: Your applications don't know they're behind a proxy
- **TLS Termination**: Secure WebSocket (wss://) connections handled automatically

## Common Use Cases

WebSocket support enables proxying for:

| Application Type | Examples |
|-----------------|----------|
| **Chat Applications** | Slack alternatives, support chat widgets |
| **Live Dashboards** | Monitoring tools, analytics platforms |
| **Collaborative Tools** | Real-time document editing, whiteboards |
| **Gaming** | Multiplayer game servers, matchmaking |
| **Notifications** | Push notifications, live alerts |
| **Streaming** | Live data feeds, stock tickers |

## How It Works

When Caddy receives a request with WebSocket upgrade headers:

1. Caddy detects the `Upgrade: websocket` header
2. The connection is upgraded from HTTP to WebSocket
3. Traffic flows bidirectionally through the proxy
4. Connection remains open until either side closes it

### Technical Details

Caddy handles these WebSocket aspects automatically:

- **Connection Upgrade**: Properly forwards upgrade headers
- **Protocol Negotiation**: Passes through subprotocol selection
- **Keep-Alive**: Maintains connection through proxy timeouts
- **Graceful Close**: Handles WebSocket close frames correctly

## Configuration

No configuration is needed. Simply create a proxy host pointing to your WebSocket-enabled backend:

```text
Backend: http://your-app:3000
```

Your application's WebSocket connections (both `ws://` and `wss://`) will work automatically.

## Troubleshooting

If WebSocket connections fail:

1. **Check Backend**: Ensure your app listens for WebSocket connections
2. **Verify Port**: WebSocket uses the same port as HTTP
3. **Test Directly**: Try connecting to the backend without the proxy
4. **Check Logs**: Look for connection errors in real-time logs

## Related

- [Real-Time Logs](logs.md)
- [Proxy Hosts](proxy-hosts.md)
- [Back to Features](../features.md)
