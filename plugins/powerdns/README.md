# PowerDNS Plugin for Charon

This is an example DNS provider plugin for Charon that adds support for PowerDNS Authoritative Server.

## Building

To build this plugin, you **must** use `CGO_ENABLED=1` and the same Go version as the Charon binary:

```bash
cd plugins/powerdns
CGO_ENABLED=1 go build -buildmode=plugin -o ../powerdns.so main.go
```

## Installation

1. Build the plugin as shown above
2. Copy `powerdns.so` to `/app/plugins/` (or your configured plugin directory)
3. Restart Charon to load the plugin
4. The PowerDNS provider will appear in the DNS providers list

## Configuration

The PowerDNS plugin requires:

- **API URL**: The PowerDNS HTTP API endpoint (e.g., `https://pdns.example.com:8081`)
- **API Key**: Your PowerDNS API key (X-API-Key header value)
- **Server ID** (optional): PowerDNS server ID (default: `localhost`)

## Caddy Requirement

This plugin only handles the Charon UI/API integration. To use PowerDNS for DNS challenges, Caddy must be built with the [caddy-dns/powerdns](https://github.com/caddy-dns/powerdns) module.

## Security

Always verify the plugin source before loading it. Plugins run in the same process as Charon and have full access to system resources.
