#!/bin/bash
# Script to create all CaddyProxyManager+ issues from PROJECT_PLANNING.md
# Requires: gh (GitHub CLI) - Install: https://cli.github.com/

set -e  # Exit on error

REPO="Wikid82/CaddyProxyManagerPlus"

echo "🚀 Creating CaddyProxyManager+ Issues"
echo "======================================"
echo ""

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed."
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI."
    echo "Run: gh auth login"
    exit 1
fi

echo "✅ GitHub CLI ready"
echo ""

# Function to create an issue
create_issue() {
    local title="$1"
    local labels="$2"
    local body="$3"

    echo "Creating: $title"
    gh issue create \
        --repo "$REPO" \
        --title "$title" \
        --label "$labels" \
        --body "$body" || echo "⚠️  Failed to create issue: $title"
}

echo "📋 Creating Alpha Issues (Foundation)"
echo "--------------------------------------"

# Issue #1
create_issue \
    "Project Architecture & Tech Stack Selection" \
    "alpha,critical,architecture" \
    "## Description
Define the technical foundation for CaddyProxyManager+.

## Tasks
- [ ] Choose backend framework (Go for native Caddy integration vs. Node.js/Python for rapid dev)
- [ ] Choose frontend framework (React, Vue, Svelte)
- [ ] Define database (SQLite for simplicity vs. PostgreSQL for scale)
- [ ] Design API architecture (REST vs. GraphQL)
- [ ] Define project structure and monorepo vs. multi-repo
- [ ] Document tech stack decisions
- [ ] Create initial project scaffold

## Acceptance Criteria
- [ ] Tech stack documented in README.md
- [ ] Project structure created
- [ ] Development environment setup instructions
- [ ] Build system configured

## Priority
Critical - Blocks all other development

## Milestone
Alpha"

# Issue #2
create_issue \
    "Caddy Integration & Configuration Management" \
    "alpha,critical,backend,caddy" \
    "## Description
Build the core bridge between the web UI and Caddy server.

## Tasks
- [ ] Implement Caddy API client/wrapper
- [ ] Design Caddyfile generation system from database
- [ ] Implement configuration validation
- [ ] Create config reload mechanism (zero-downtime)
- [ ] Error handling and rollback on invalid configs
- [ ] Unit tests for config generation

## Acceptance Criteria
- [ ] Can programmatically generate valid Caddyfiles
- [ ] Can reload Caddy configuration via API
- [ ] Invalid configs are caught before reload
- [ ] Automatic rollback on failure

## Priority
Critical - Core functionality

## Milestone
Alpha"

# Issue #3
create_issue \
    "Database Schema & Models" \
    "alpha,critical,backend,database" \
    "## Description
Design and implement the database layer for storing proxy configurations.

## Tasks
- [ ] Design database schema (hosts, certificates, users, settings)
- [ ] Implement ORM/query builder integration
- [ ] Create migration system
- [ ] Implement models for: Proxy Hosts, SSL Certificates, Access Lists, Users
- [ ] Add database seeding for development
- [ ] Write database documentation

## Acceptance Criteria
- [ ] Schema supports all planned features
- [ ] Migrations run cleanly
- [ ] Models have proper relationships
- [ ] Database can be backed up and restored

## Priority
Critical - Foundation for all data

## Milestone
Alpha"

# Issue #4
create_issue \
    "Basic Web UI Foundation" \
    "alpha,critical,frontend,ui" \
    "## Description
Create the foundational web interface structure.

## Tasks
- [ ] Design UI/UX wireframes
- [ ] Implement authentication/login page
- [ ] Create dashboard layout with navigation
- [ ] Implement responsive design framework
- [ ] Set up state management (Redux/Vuex/etc.)
- [ ] Create reusable component library
- [ ] Implement dark/light theme support

## Acceptance Criteria
- [ ] Clean, modern interface inspired by NPM
- [ ] Mobile responsive
- [ ] Consistent design language
- [ ] Working navigation structure

## Priority
Critical - User interface foundation

## Milestone
Alpha"

# Issue #5
create_issue \
    "Proxy Host Management (Core Feature)" \
    "alpha,critical,feature" \
    "## Description
Implement the core proxy host creation and management.

## Tasks
- [ ] Create \"Add Proxy Host\" form (domain, scheme, forward hostname, port)
- [ ] Implement proxy host listing/grid view
- [ ] Add edit/delete functionality
- [ ] Implement proxy host enable/disable toggle
- [ ] Add WebSocket support toggle
- [ ] Implement custom locations/paths
- [ ] Add advanced options (headers, caching)

## Acceptance Criteria
- [ ] Can create basic proxy hosts
- [ ] Hosts appear in list immediately
- [ ] Changes reflect in Caddy config
- [ ] Can proxy HTTP/HTTPS services successfully

## Priority
Critical - Core value proposition

## Milestone
Alpha"

# Issue #6
create_issue \
    "Automatic HTTPS & Certificate Management" \
    "alpha,critical,feature,ssl" \
    "## Description
Implement Caddy's automatic HTTPS with UI controls.

## Tasks
- [ ] Implement \"Force SSL\" toggle per host
- [ ] Add certificate status display (valid, expiring, failed)
- [ ] Create certificate list view
- [ ] Implement HTTP to HTTPS redirect
- [ ] Add HSTS header toggle with max-age configuration
- [ ] Show certificate details (expiry, issuer, domains)
- [ ] Implement certificate renewal monitoring

## Acceptance Criteria
- [ ] Automatic certificate acquisition works
- [ ] Certificate status visible in UI
- [ ] Warnings for expiring certificates
- [ ] Force SSL works correctly

## Priority
Critical - Key differentiator from NPM

## Milestone
Alpha"

# Issue #7
create_issue \
    "User Authentication & Authorization" \
    "alpha,high,security" \
    "## Description
Implement secure user management for the admin panel.

## Tasks
- [ ] Implement user registration/login system
- [ ] Add password hashing (bcrypt/argon2)
- [ ] Create session management with JWT/cookies
- [ ] Implement basic RBAC (admin vs. user roles)
- [ ] Add \"Change Password\" functionality
- [ ] Implement account lockout after failed attempts
- [ ] Add session timeout

## Acceptance Criteria
- [ ] Secure login protects admin panel
- [ ] Passwords properly hashed
- [ ] Sessions expire appropriately
- [ ] Multiple users supported with roles

## Priority
High - Security requirement

## Milestone
Alpha"

# Issue #8
create_issue \
    "Basic Access Logging" \
    "alpha,high,monitoring" \
    "## Description
Implement basic access logging for troubleshooting.

## Tasks
- [ ] Configure Caddy access logging format
- [ ] Create log storage/rotation strategy
- [ ] Implement log viewer in UI (paginated)
- [ ] Add log filtering (by host, status code, date)
- [ ] Implement log search functionality
- [ ] Add log download capability

## Acceptance Criteria
- [ ] All proxy requests logged
- [ ] Logs viewable in UI
- [ ] Logs searchable and filterable
- [ ] Logs rotate to prevent disk fill

## Priority
High - Essential for debugging

## Milestone
Alpha"

# Issue #9
create_issue \
    "Settings & Configuration UI" \
    "alpha,high,ui" \
    "## Description
Create settings interface for global configurations.

## Tasks
- [ ] Create settings page layout
- [ ] Implement default certificate email configuration
- [ ] Add Caddy admin API endpoint configuration
- [ ] Implement backup/restore settings
- [ ] Add system status display (Caddy version, uptime)
- [ ] Create health check endpoint
- [ ] Implement update check mechanism

## Acceptance Criteria
- [ ] All global settings configurable
- [ ] Settings persist across restarts
- [ ] System health visible at a glance

## Priority
High - System management

## Milestone
Alpha"

# Issue #10
create_issue \
    "Docker & Deployment Configuration" \
    "alpha,high,deployment" \
    "## Description
Create easy deployment via Docker.

## Tasks
- [ ] Create optimized Dockerfile (multi-stage build)
- [ ] Write docker-compose.yml with volume mounts
- [ ] Configure proper networking for Caddy
- [ ] Implement environment variable configuration
- [ ] Create entrypoint script for initialization
- [ ] Add healthcheck to Docker container
- [ ] Write deployment documentation

## Acceptance Criteria
- [ ] Single \`docker-compose up\` starts everything
- [ ] Data persists in volumes
- [ ] Environment easily configurable
- [ ] Works on common NAS platforms (Synology, Unraid)

## Priority
High - Deployment simplicity

## Milestone
Alpha"

echo ""
echo "🔐 Creating Beta Issues - Authentication & Access Control"
echo "--------------------------------------------------------"

# Issue #11
create_issue \
    "Forward Auth Integration (SSO - Easy Mode)" \
    "beta,critical,security,sso" \
    "## Description
Implement forward authentication for SSO integration.

## Tasks
- [ ] Design forward auth configuration UI
- [ ] Implement Caddy forward_auth directive generation
- [ ] Add per-host \"Enable Forward Auth\" toggle
- [ ] Create forward auth provider templates (Authelia, Authentik, Pomerium)
- [ ] Add custom forward auth endpoint configuration
- [ ] Implement trusted header forwarding
- [ ] Add bypass rules (for API endpoints, webhooks)
- [ ] Create forward auth testing tool

## Acceptance Criteria
- [ ] Can enable forward auth per proxy host
- [ ] Templates work with popular SSO providers
- [ ] Protected services require authentication
- [ ] API endpoints can bypass auth

## Priority
Critical - Major beta feature

## Milestone
Beta"

# Issue #12
create_issue \
    "Built-in OAuth/OIDC Server (SSO - Plus Feature)" \
    "beta,high,security,sso,plus" \
    "## Description
Implement internal authentication server using caddy-security plugin.

## Tasks
- [ ] Integrate caddy-security plugin
- [ ] Design user/group management UI
- [ ] Implement local user creation with password hashing
- [ ] Add OAuth/OIDC provider configuration
- [ ] Create application registration system
- [ ] Implement consent screen
- [ ] Add 2FA/TOTP support
- [ ] Create identity provider dashboard

## Acceptance Criteria
- [ ] Can create local users for authentication
- [ ] Can protect services with built-in SSO
- [ ] 2FA works correctly
- [ ] External OIDC providers can be configured

## Priority
High - Differentiating feature

## Milestone
Beta"

# Issue #13
create_issue \
    "HTTP Basic Authentication" \
    "beta,high,security" \
    "## Description
Implement simple HTTP Basic Auth for services.

## Tasks
- [ ] Add \"Enable Basic Auth\" toggle per host
- [ ] Create username/password input with hashing
- [ ] Implement Caddy basicauth directive generation
- [ ] Add multiple user support per host
- [ ] Create basic auth realm configuration
- [ ] Implement password strength validation
- [ ] Add basic auth testing tool

## Acceptance Criteria
- [ ] Basic auth protects services
- [ ] Multiple users per host supported
- [ ] Passwords securely hashed
- [ ] Browser prompts correctly

## Priority
High - Simple auth option

## Milestone
Beta"

# Issue #14
create_issue \
    "IP-based Access Control Lists (ACLs)" \
    "beta,high,security" \
    "## Description
Implement IP whitelisting/blacklisting and geo-blocking.

## Tasks
- [ ] Design ACL management UI
- [ ] Implement IP/CIDR whitelist per host
- [ ] Add blacklist functionality
- [ ] Implement \"Local Network Only\" toggle (RFC1918)
- [ ] Add geo-blocking with country selection
- [ ] Integrate MaxMind GeoIP2 database
- [ ] Create ACL templates (local only, US only, etc.)
- [ ] Implement ACL testing tool

## Acceptance Criteria
- [ ] Can restrict access by IP/CIDR
- [ ] Local network toggle works
- [ ] Geo-blocking blocks correctly
- [ ] ACLs apply to specific hosts

## Priority
High - Access control

## Milestone
Beta"

echo ""
echo "🛡️ Creating Beta Issues - Threat Protection"
echo "-------------------------------------------"

# Issue #15
create_issue \
    "CrowdSec Integration" \
    "beta,critical,security,crowdsec" \
    "## Description
Integrate CrowdSec for active threat protection.

## Tasks
- [ ] Design CrowdSec integration architecture
- [ ] Implement CrowdSec bouncer for Caddy
- [ ] Create CrowdSec installation wizard in UI
- [ ] Add CrowdSec status monitoring
- [ ] Implement banned IP dashboard
- [ ] Add manual IP ban/unban functionality
- [ ] Create scenario/collection management UI
- [ ] Add CrowdSec log parsing setup

## Acceptance Criteria
- [ ] CrowdSec blocks malicious IPs automatically
- [ ] Banned IPs visible in dashboard
- [ ] Can manually ban/unban IPs
- [ ] CrowdSec status visible

## Priority
Critical - Core security feature

## Milestone
Beta"

# Issue #16
create_issue \
    "Web Application Firewall (WAF) Integration" \
    "beta,high,security,waf,plus" \
    "## Description
Integrate Coraza WAF with OWASP Core Rule Set.

## Tasks
- [ ] Integrate caddy-coraza-filter plugin
- [ ] Implement \"Enable WAF\" toggle per host
- [ ] Add OWASP CRS rule set management
- [ ] Create WAF rule exclusion system (for false positives)
- [ ] Implement WAF logging and alerts
- [ ] Add WAF statistics dashboard
- [ ] Create paranoia level selector
- [ ] Implement custom WAF rules

## Acceptance Criteria
- [ ] WAF blocks common attacks (SQLi, XSS)
- [ ] Can enable/disable per host
- [ ] False positives manageable
- [ ] WAF events logged and visible

## Priority
High - Plus feature

## Milestone
Beta"

# Issue #17
create_issue \
    "Rate Limiting & DDoS Protection" \
    "beta,high,security" \
    "## Description
Implement request rate limiting per host.

## Tasks
- [ ] Implement Caddy rate_limit directive integration
- [ ] Create rate limit preset templates (login, API, standard)
- [ ] Add custom rate limit configuration
- [ ] Implement per-IP rate limiting
- [ ] Add per-endpoint rate limits
- [ ] Create rate limit bypass list (trusted IPs)
- [ ] Add rate limit violation logging
- [ ] Implement rate limit testing tool

## Acceptance Criteria
- [ ] Rate limits prevent brute force
- [ ] Presets work correctly
- [ ] Legitimate traffic not affected
- [ ] Rate limit hits logged

## Priority
High - Essential protection

## Milestone
Beta"

# Issue #18
create_issue \
    "HTTP Security Headers" \
    "beta,medium,security" \
    "## Description
Implement automatic security header injection.

## Tasks
- [ ] Create security headers configuration UI
- [ ] Implement HSTS with preload support
- [ ] Add Content-Security-Policy builder
- [ ] Implement X-Frame-Options (DENY/SAMEORIGIN)
- [ ] Add X-Content-Type-Options (nosniff)
- [ ] Implement Referrer-Policy configuration
- [ ] Add Permissions-Policy headers
- [ ] Create security header presets (basic, strict, paranoid)
- [ ] Implement security score calculator

## Acceptance Criteria
- [ ] Security headers automatically added
- [ ] CSP configurable without breaking sites
- [ ] Presets available for easy setup
- [ ] Security score shown in UI

## Priority
Medium - Hardening feature

## Milestone
Beta"

echo ""
echo "🚦 Creating Beta Issues - Traffic & TLS Management"
echo "--------------------------------------------------"

# Issue #19
create_issue \
    "DNS Challenge Support for Wildcard Certificates" \
    "beta,critical,ssl" \
    "## Description
Implement DNS challenge for users behind firewalls.

## Tasks
- [ ] Design DNS provider configuration UI
- [ ] Implement DNS provider dropdown (Cloudflare, Route53, etc.)
- [ ] Add API token/credential secure storage
- [ ] Implement wildcard certificate support
- [ ] Add DNS provider testing/validation
- [ ] Create DNS challenge troubleshooting guide
- [ ] Implement credential encryption at rest
- [ ] Add popular provider quick-setup guides

## Acceptance Criteria
- [ ] Wildcard certificates work
- [ ] Popular DNS providers supported
- [ ] Credentials stored securely
- [ ] Clear error messages for failures

## Priority
Critical - Home lab requirement

## Milestone
Beta"

# Issue #20
create_issue \
    "Custom Certificate Upload & Management" \
    "beta,high,ssl" \
    "## Description
Allow users to upload their own certificates.

## Tasks
- [ ] Create certificate upload UI
- [ ] Implement certificate validation (format, expiry)
- [ ] Add private key upload with encryption
- [ ] Support certificate chains/intermediates
- [ ] Implement certificate assignment to hosts
- [ ] Add expiry warnings for custom certificates
- [ ] Create certificate export functionality
- [ ] Support PEM, PFX, DER formats

## Acceptance Criteria
- [ ] Can upload custom certificates
- [ ] Certificates validated before acceptance
- [ ] Private keys securely stored
- [ ] Expiry warnings work

## Priority
High - Advanced SSL management

## Milestone
Beta"

# Issue #21
create_issue \
    "Client Certificate Authentication (mTLS)" \
    "beta,medium,security,plus" \
    "## Description
Implement mutual TLS for zero-trust setups.

## Tasks
- [ ] Design mTLS configuration UI
- [ ] Implement client CA upload
- [ ] Add \"Require Client Certificate\" toggle per host
- [ ] Create client certificate generation tool
- [ ] Implement certificate revocation checking
- [ ] Add client certificate verification logging
- [ ] Create mTLS troubleshooting guide
- [ ] Implement certificate DN-based authorization

## Acceptance Criteria
- [ ] mTLS protects sensitive services
- [ ] Client certificates required when enabled
- [ ] Invalid certificates rejected
- [ ] mTLS events logged

## Priority
Medium - Advanced security

## Milestone
Beta"

# Issue #22
create_issue \
    "Advanced TLS Configuration" \
    "beta,medium,ssl" \
    "## Description
Expose advanced TLS options for power users.

## Tasks
- [ ] Add TLS version selector (1.2, 1.3)
- [ ] Implement cipher suite configuration
- [ ] Add ALPN protocol configuration
- [ ] Implement OCSP stapling toggle
- [ ] Add certificate transparency logging
- [ ] Create TLS security score calculator
- [ ] Implement SSL Labs integration for testing
- [ ] Add \"Modern/Intermediate/Old\" compatibility presets

## Acceptance Criteria
- [ ] TLS configuration customizable
- [ ] Presets available for common scenarios
- [ ] Security score displayed
- [ ] Changes applied correctly

## Priority
Medium - Power user feature

## Milestone
Beta"

echo ""
echo "📊 Creating Beta Issues - Monitoring & Logging"
echo "----------------------------------------------"

# Issue #23
create_issue \
    "Enhanced Dashboard with Statistics" \
    "beta,high,monitoring" \
    "## Description
Create a comprehensive dashboard with service statistics.

## Tasks
- [ ] Design dashboard layout with widgets
- [ ] Implement request count statistics (24h, 7d, 30d)
- [ ] Add top accessed hosts chart
- [ ] Create HTTP status code distribution chart
- [ ] Implement traffic volume graphs
- [ ] Add certificate expiry warnings
- [ ] Create service health indicators
- [ ] Implement real-time update with WebSockets

## Acceptance Criteria
- [ ] Dashboard shows key metrics at a glance
- [ ] Charts update in real-time
- [ ] Performance remains smooth
- [ ] Mobile responsive

## Priority
High - User visibility

## Milestone
Beta"

# Issue #24
create_issue \
    "CrowdSec Dashboard Integration" \
    "beta,high,monitoring,crowdsec" \
    "## Description
Embed CrowdSec metrics and decisions in the UI.

## Tasks
- [ ] Create CrowdSec metrics dashboard tab
- [ ] Display active bans with reasons
- [ ] Show CrowdSec scenarios triggered
- [ ] Implement ban history timeline
- [ ] Add top attacking IPs chart
- [ ] Create attack type breakdown
- [ ] Implement CrowdSec alert notifications
- [ ] Add ban export functionality

## Acceptance Criteria
- [ ] CrowdSec activity visible in UI
- [ ] Bans displayed with context
- [ ] Historical data available
- [ ] Notifications work

## Priority
High - Security visibility

## Milestone
Beta"

# Issue #25
create_issue \
    "GoAccess Integration for Analytics" \
    "beta,medium,monitoring" \
    "## Description
Integrate GoAccess for beautiful log analysis.

## Tasks
- [ ] Integrate GoAccess binary or build from source
- [ ] Create \"Generate Report\" button in UI
- [ ] Implement GoAccess HTML report embedding
- [ ] Add scheduled report generation
- [ ] Implement report caching for performance
- [ ] Add report date range selector
- [ ] Create per-host GoAccess reports
- [ ] Implement report export functionality

## Acceptance Criteria
- [ ] GoAccess reports generated successfully
- [ ] Reports embedded in UI
- [ ] Performance acceptable
- [ ] Reports accurate

## Priority
Medium - Nice analytics

## Milestone
Beta"

# Issue #26
create_issue \
    "Live Log Viewer & Search" \
    "beta,high,monitoring" \
    "## Description
Create a powerful log viewer with real-time updates.

## Tasks
- [ ] Implement log streaming via WebSocket
- [ ] Create log viewer UI with syntax highlighting
- [ ] Add log level filtering (error, warn, info, debug)
- [ ] Implement full-text log search
- [ ] Add timestamp range filtering
- [ ] Create per-host log filtering
- [ ] Implement log export (CSV, JSON)
- [ ] Add \"Follow\" mode for real-time tailing

## Acceptance Criteria
- [ ] Logs stream in real-time
- [ ] Search is fast and accurate
- [ ] Filters work correctly
- [ ] Export works for large logs

## Priority
High - Troubleshooting essential

## Milestone
Beta"

# Issue #27
create_issue \
    "Alerting & Notifications System" \
    "beta,medium,monitoring" \
    "## Description
Implement alerting for critical events.

## Tasks
- [ ] Design notification system architecture
- [ ] Implement email notifications (SMTP)
- [ ] Add webhook notifications (Discord, Slack, custom)
- [ ] Create alert rules (certificate expiry, service down, ban threshold)
- [ ] Implement notification testing
- [ ] Add notification history log
- [ ] Create alert rule templates
- [ ] Implement notification rate limiting

## Acceptance Criteria
- [ ] Alerts sent for critical events
- [ ] Multiple notification channels supported
- [ ] Alert rules customizable
- [ ] No spam (rate limited)

## Priority
Medium - Proactive monitoring

## Milestone
Beta"

echo ""
echo "🎨 Creating Beta Issues - User Experience"
echo "-----------------------------------------"

# Issue #28
create_issue \
    "Onboarding Wizard & First-Time Setup" \
    "beta,high,ui" \
    "## Description
Create a smooth first-time user experience.

## Tasks
- [ ] Design multi-step setup wizard
- [ ] Implement admin account creation
- [ ] Add domain/email configuration
- [ ] Create first proxy host tutorial
- [ ] Implement DNS challenge provider setup wizard
- [ ] Add CrowdSec installation wizard
- [ ] Create \"Quick Start\" templates (Plex, Sonarr, etc.)
- [ ] Implement interactive tooltips/hints

## Acceptance Criteria
- [ ] New users guided through setup
- [ ] Can create first proxy host easily
- [ ] Common services have templates
- [ ] Setup completes successfully

## Priority
High - First impressions matter

## Milestone
Beta"

# Issue #29
create_issue \
    "Import from Nginx Proxy Manager" \
    "beta,medium,feature" \
    "## Description
Allow users to migrate from NPM.

## Tasks
- [ ] Design NPM database import system
- [ ] Parse NPM SQLite/MySQL database
- [ ] Map NPM proxy hosts to Caddy config
- [ ] Import SSL certificates
- [ ] Convert NPM access lists to ACLs
- [ ] Create import preview/validation
- [ ] Implement backup before import
- [ ] Create migration guide documentation

## Acceptance Criteria
- [ ] Can import NPM database
- [ ] Proxy hosts converted correctly
- [ ] Certificates imported
- [ ] Rollback available if issues

## Priority
Medium - Migration path

## Milestone
Beta"

# Issue #30
create_issue \
    "Configuration Backup & Restore" \
    "beta,high,feature" \
    "## Description
Implement comprehensive backup system.

## Tasks
- [ ] Create backup format (database + configs + certificates)
- [ ] Implement one-click backup button
- [ ] Add scheduled automatic backups
- [ ] Create restore functionality with validation
- [ ] Implement backup encryption (optional)
- [ ] Add backup to remote storage (S3, SFTP)
- [ ] Create backup history management
- [ ] Implement disaster recovery guide

## Acceptance Criteria
- [ ] Backups contain all critical data
- [ ] Restore works flawlessly
- [ ] Automatic backups run on schedule
- [ ] Remote backup options available

## Priority
High - Data safety

## Milestone
Beta"

# Issue #31
create_issue \
    "Multi-language Support (i18n)" \
    "beta,low,ui" \
    "## Description
Internationalize the interface.

## Tasks
- [ ] Implement i18n framework
- [ ] Extract all strings to translation files
- [ ] Add language selector in settings
- [ ] Translate to: Spanish, French, German, Chinese
- [ ] Create translation contribution guide
- [ ] Implement date/time localization
- [ ] Add RTL language support framework
- [ ] Create translation verification tests

## Acceptance Criteria
- [ ] UI supports multiple languages
- [ ] Language switching works instantly
- [ ] All strings translatable
- [ ] Community can contribute translations

## Priority
Low - International audience

## Milestone
Beta"

# Issue #32
create_issue \
    "Dark Mode & Theme Customization" \
    "beta,low,ui" \
    "## Description
Implement theme system beyond basic dark/light.

## Tasks
- [ ] Create comprehensive theme system
- [ ] Implement custom color picker
- [ ] Add pre-built theme gallery
- [ ] Create theme import/export
- [ ] Add logo customization
- [ ] Implement CSS custom properties
- [ ] Create theme preview
- [ ] Add \"Follow System\" option

## Acceptance Criteria
- [ ] Themes change entire UI
- [ ] Custom themes saveable
- [ ] Pre-built themes available
- [ ] System theme sync works

## Priority
Low - Personalization

## Milestone
Beta"

echo ""
echo "🔧 Creating Post-Beta Issues - Advanced Features"
echo "------------------------------------------------"

# Issue #33
create_issue \
    "API & CLI Tools" \
    "post-beta,medium,feature" \
    "## Description
Expose REST API and CLI for automation.

## Tasks
- [ ] Design RESTful API
- [ ] Implement API authentication (API keys)
- [ ] Create API documentation (OpenAPI/Swagger)
- [ ] Build CLI tool for management
- [ ] Implement Terraform provider
- [ ] Create API client libraries (Python, Go)
- [ ] Add API rate limiting
- [ ] Create API usage examples

## Acceptance Criteria
- [ ] Full API coverage of UI features
- [ ] API documented comprehensively
- [ ] CLI tool functional
- [ ] Automation possible

## Priority
Medium - Advanced users

## Milestone
Post-Beta"

# Issue #34
create_issue \
    "High Availability & Clustering" \
    "post-beta,low,feature,enterprise" \
    "## Description
Support multiple Caddy instances with shared config.

## Tasks
- [ ] Design distributed architecture
- [ ] Implement config synchronization
- [ ] Add load balancer support
- [ ] Create shared certificate storage
- [ ] Implement cluster health monitoring
- [ ] Add automatic failover
- [ ] Create split-brain prevention
- [ ] Document HA deployment

## Acceptance Criteria
- [ ] Multiple instances share config
- [ ] Failover works automatically
- [ ] No downtime during updates
- [ ] Certificates shared correctly

## Priority
Low - Enterprise feature

## Milestone
Post-Beta"

# Issue #35
create_issue \
    "Plugin System & Marketplace" \
    "post-beta,low,feature" \
    "## Description
Allow community extensions and plugins.

## Tasks
- [ ] Design plugin architecture
- [ ] Implement plugin loader
- [ ] Create plugin API documentation
- [ ] Build plugin marketplace UI
- [ ] Implement plugin sandboxing
- [ ] Add plugin installation/update system
- [ ] Create example plugins
- [ ] Implement plugin review process

## Acceptance Criteria
- [ ] Plugins extend functionality
- [ ] Marketplace browseable
- [ ] Plugins installable with one click
- [ ] Security maintained

## Priority
Low - Extensibility

## Milestone
Post-Beta"

# Issue #36
create_issue \
    "Advanced Stream (TCP/UDP) Proxying" \
    "post-beta,medium,feature" \
    "## Description
Support non-HTTP protocols via Caddy layer4 plugin.

## Tasks
- [ ] Integrate caddy-layer4 plugin
- [ ] Design TCP/UDP proxy UI
- [ ] Implement port mapping configuration
- [ ] Add SNI-based routing for TCP
- [ ] Create protocol detection
- [ ] Implement connection logging
- [ ] Add common protocol templates (SSH, database, etc.)
- [ ] Create stream proxy testing tool

## Acceptance Criteria
- [ ] Can proxy TCP/UDP services
- [ ] SNI routing works
- [ ] Common protocols templated
- [ ] Performance acceptable

## Priority
Medium - Advanced use case

## Milestone
Post-Beta"

echo ""
echo "📚 Creating Continuous Issues - Documentation & Testing"
echo "-------------------------------------------------------"

# Issue #37
create_issue \
    "Comprehensive Documentation" \
    "documentation,high" \
    "## Description
Create user and developer documentation.

## Tasks
- [ ] Write installation guide
- [ ] Create quick start tutorial
- [ ] Document all features with screenshots
- [ ] Create troubleshooting guide
- [ ] Write security best practices guide
- [ ] Create API documentation
- [ ] Write contributing guide
- [ ] Create video tutorials

## Acceptance Criteria
- [ ] All features documented
- [ ] Screenshots up to date
- [ ] Common issues covered
- [ ] Videos available

## Priority
High - Essential for adoption

## Milestone
Continuous"

# Issue #38
create_issue \
    "Automated Testing Suite" \
    "testing,high" \
    "## Description
Implement comprehensive test coverage.

## Tasks
- [ ] Set up testing framework
- [ ] Write unit tests (backend)
- [ ] Write integration tests (API)
- [ ] Create end-to-end tests (UI)
- [ ] Implement CI/CD pipeline
- [ ] Add test coverage reporting
- [ ] Create performance benchmarks
- [ ] Implement security scanning

## Acceptance Criteria
- [ ] >80% code coverage
- [ ] CI runs on every commit
- [ ] E2E tests cover critical paths
- [ ] Security scans pass

## Priority
High - Quality assurance

## Milestone
Continuous"

# Issue #39
create_issue \
    "Community & Support Infrastructure" \
    "community,medium" \
    "## Description
Build community support channels.

## Tasks
- [ ] Set up Discord/Matrix server
- [ ] Create GitHub Discussions
- [ ] Set up forum (Discourse)
- [ ] Create contribution templates
- [ ] Implement issue templates
- [ ] Create PR review process
- [ ] Set up community guidelines
- [ ] Create roadmap page

## Acceptance Criteria
- [ ] Community channels active
- [ ] Clear contribution process
- [ ] Issue templates helpful
- [ ] Roadmap transparent

## Priority
Medium - Community building

## Milestone
Continuous"

# Issue #40
create_issue \
    "Performance Optimization & Benchmarking" \
    "performance,medium" \
    "## Description
Ensure CaddyProxyManager+ performs well under load.

## Tasks
- [ ] Create performance benchmark suite
- [ ] Profile database queries
- [ ] Optimize Caddyfile generation
- [ ] Implement caching where appropriate
- [ ] Test with 100+ proxy hosts
- [ ] Optimize frontend bundle size
- [ ] Test on low-resource devices (Raspberry Pi)
- [ ] Document performance characteristics

## Acceptance Criteria
- [ ] Handles 100+ hosts smoothly
- [ ] Config reload <1 second
- [ ] UI remains responsive
- [ ] Works on Raspberry Pi 4

## Priority
Medium - Performance matters

## Milestone
Continuous"

echo ""
echo "======================================"
echo "✅ Issue Creation Complete!"
echo ""
echo "📊 Summary:"
echo "  - Alpha Issues: 10"
echo "  - Beta Issues: 22"
echo "  - Post-Beta Issues: 4"
echo "  - Continuous Issues: 4"
echo "  - Total: 40 issues"
echo ""
echo "🎯 Next Steps:"
echo "  1. View issues: https://github.com/$REPO/issues"
echo "  2. Check project board: https://github.com/users/Wikid82/projects/7"
echo "  3. All issues should be auto-labeled and added to board"
echo "  4. Start with Alpha issues #1-10"
echo ""
echo "🚀 Happy coding!"
