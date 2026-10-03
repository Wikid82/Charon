# PowerDNS Plugin for Charon

This is an example DNS provider plugin for Charon that adds support for PowerDNS Authoritative Server.

## Building

The plugin is a module in the repository workspace (`go.work`), so it is built against the exact dependency versions of the Charon backend. It **must** be built with `CGO_ENABLED=1`, the same Go toolchain, and the same build flags as the Charon binary: no `-trimpath`, no `-race`, no `-cover`. A plugin built differently is rejected when Charon loads it.

```bash
make plugin-powerdns          # build (into a temp dir) and run the plugin tests
make plugin-powerdns-smoke    # rebuild and verify the plugin loads into the host

# or manually
cd plugins/powerdns
CGO_ENABLED=1 go build -buildmode=plugin -o powerdns.so .
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
