# CaddyProxyManager+ Project Planning & Issues

## Project Vision
Bridge the gap between Nginx Proxy Manager's simplicity and Caddy's modern design, targeting home-lab users running arr suite, Plex, Jellyfin, and similar services.

## Core Value Propositions
- "Set it and forget it" HTTPS (Caddy's specialty)
- Simple "front door" for all services
- Protection from bots and scanners with CrowdSec
- Modern security features with NPM-like simplicity

---

## Development Milestones

### Milestone 1: Foundation & Alpha Build
**Target**: Core functionality with basic proxy management and HTTPS

### Milestone 2: Beta Build
**Target**: Full security features, SSO, and monitoring

### Milestone 3: Production v1.0
**Target**: Polished UI, documentation, and stability

---

## Issue Structure

### 🏗️ FOUNDATION (Alpha - Phase 1)

#### Issue #1: Project Architecture & Tech Stack Selection
**Priority**: `critical`
**Labels**: `alpha`, `architecture`, `critical`
**Description**:
Define the technical foundation for CaddyProxyManager+.

**Tasks**:
- [ ] Choose backend framework (Go for native Caddy integration vs. Node.js/Python for rapid dev)
- [ ] Choose frontend framework (React, Vue, Svelte)
- [ ] Define database (SQLite for simplicity vs. PostgreSQL for scale)
- [ ] Design API architecture (REST vs. GraphQL)
- [ ] Define project structure and monorepo vs. multi-repo
- [ ] Document tech stack decisions
- [ ] Create initial project scaffold

**Acceptance Criteria**:
- Tech stack documented in README.md
- Project structure created
- Development environment setup instructions
- Build system configured

---

#### Issue #2: Caddy Integration & Configuration Management
**Priority**: `critical`
**Labels**: `alpha`, `backend`, `critical`, `caddy`
**Description**:
Build the core bridge between the web UI and Caddy server.

**Tasks**:
- [ ] Implement Caddy API client/wrapper
- [ ] Design Caddyfile generation system from database
- [ ] Implement configuration validation
- [ ] Create config reload mechanism (zero-downtime)
- [ ] Error handling and rollback on invalid configs
- [ ] Unit tests for config generation

**Acceptance Criteria**:
- Can programmatically generate valid Caddyfiles
- Can reload Caddy configuration via API
- Invalid configs are caught before reload
- Automatic rollback on failure

---

#### Issue #3: Database Schema & Models
**Priority**: `critical`
**Labels**: `alpha`, `backend`, `critical`, `database`
**Description**:
Design and implement the database layer for storing proxy configurations.

**Tasks**:
- [ ] Design database schema (hosts, certificates, users, settings)
- [ ] Implement ORM/query builder integration
- [ ] Create migration system
- [ ] Implement models for: Proxy Hosts, SSL Certificates, Access Lists, Users
- [ ] Add database seeding for development
- [ ] Write database documentation

**Acceptance Criteria**:
- Schema supports all planned features
- Migrations run cleanly
- Models have proper relationships
- Database can be backed up and restored

---

#### Issue #4: Basic Web UI Foundation
**Priority**: `critical`
**Labels**: `alpha`, `frontend`, `critical`, `ui`
**Description**:
Create the foundational web interface structure.

**Tasks**:
- [ ] Design UI/UX wireframes
- [ ] Implement authentication/login page
- [ ] Create dashboard layout with navigation
- [ ] Implement responsive design framework
- [ ] Set up state management (Redux/Vuex/etc.)
- [ ] Create reusable component library
- [ ] Implement dark/light theme support

**Acceptance Criteria**:
- Clean, modern interface inspired by NPM
- Mobile responsive
- Consistent design language
- Working navigation structure

---

#### Issue #5: Proxy Host Management (Core Feature)
**Priority**: `critical`
**Labels**: `alpha`, `feature`, `critical`
**Description**:
Implement the core proxy host creation and management.

**Tasks**:
- [ ] Create "Add Proxy Host" form (domain, scheme, forward hostname, port)
- [ ] Implement proxy host listing/grid view
- [ ] Add edit/delete functionality
- [ ] Implement proxy host enable/disable toggle
- [ ] Add WebSocket support toggle
- [ ] Implement custom locations/paths
- [ ] Add advanced options (headers, caching)

**Acceptance Criteria**:
- Can create basic proxy hosts
- Hosts appear in list immediately
- Changes reflect in Caddy config
- Can proxy HTTP/HTTPS services successfully

---

#### Issue #6: Automatic HTTPS & Certificate Management
**Priority**: `critical`
**Labels**: `alpha`, `feature`, `critical`, `ssl`
**Description**:
Implement Caddy's automatic HTTPS with UI controls.

**Tasks**:
- [ ] Implement "Force SSL" toggle per host
- [ ] Add certificate status display (valid, expiring, failed)
- [ ] Create certificate list view
- [ ] Implement HTTP to HTTPS redirect
- [ ] Add HSTS header toggle with max-age configuration
- [ ] Show certificate details (expiry, issuer, domains)
- [ ] Implement certificate renewal monitoring

**Acceptance Criteria**:
- Automatic certificate acquisition works
- Certificate status visible in UI
- Warnings for expiring certificates
- Force SSL works correctly

---

#### Issue #7: User Authentication & Authorization
**Priority**: `high`
**Labels**: `alpha`, `security`, `high`
**Description**:
Implement secure user management for the admin panel.

**Tasks**:
- [ ] Implement user registration/login system
- [ ] Add password hashing (bcrypt/argon2)
- [ ] Create session management with JWT/cookies
- [ ] Implement basic RBAC (admin vs. user roles)
- [ ] Add "Change Password" functionality
- [ ] Implement account lockout after failed attempts
- [ ] Add session timeout

**Acceptance Criteria**:
- Secure login protects admin panel
- Passwords properly hashed
- Sessions expire appropriately
- Multiple users supported with roles

---

#### Issue #8: Basic Access Logging
**Priority**: `high`
**Labels**: `alpha`, `monitoring`, `high`
**Description**:
Implement basic access logging for troubleshooting.

**Tasks**:
- [ ] Configure Caddy access logging format
- [ ] Create log storage/rotation strategy
- [ ] Implement log viewer in UI (paginated)
- [ ] Add log filtering (by host, status code, date)
- [ ] Implement log search functionality
- [ ] Add log download capability

**Acceptance Criteria**:
- All proxy requests logged
- Logs viewable in UI
- Logs searchable and filterable
- Logs rotate to prevent disk fill

---

#### Issue #9: Settings & Configuration UI
**Priority**: `high`
**Labels**: `alpha`, `ui`, `high`
**Description**:
Create settings interface for global configurations.

**Tasks**:
- [ ] Create settings page layout
- [ ] Implement default certificate email configuration
- [ ] Add Caddy admin API endpoint configuration
- [ ] Implement backup/restore settings
- [ ] Add system status display (Caddy version, uptime)
- [ ] Create health check endpoint
- [ ] Implement update check mechanism

**Acceptance Criteria**:
- All global settings configurable
- Settings persist across restarts
- System health visible at a glance

---

#### Issue #10: Docker & Deployment Configuration
**Priority**: `high`
**Labels**: `alpha`, `deployment`, `high`
**Description**:
Create easy deployment via Docker.

**Tasks**:
- [ ] Create optimized Dockerfile (multi-stage build)
- [ ] Write docker-compose.yml with volume mounts
- [ ] Configure proper networking for Caddy
- [ ] Implement environment variable configuration
- [ ] Create entrypoint script for initialization
- [ ] Add healthcheck to Docker container
- [ ] Write deployment documentation

**Acceptance Criteria**:
- Single `docker-compose up` starts everything
- Data persists in volumes
- Environment easily configurable
- Works on common NAS platforms (Synology, Unraid)

---

### 🔐 AUTHENTICATION & ACCESS CONTROL (Beta - Phase 1)

#### Issue #11: Forward Auth Integration (SSO - Easy Mode)
**Priority**: `critical`
**Labels**: `beta`, `security`, `critical`, `sso`
**Description**:
Implement forward authentication for SSO integration.

**Tasks**:
- [ ] Design forward auth configuration UI
- [ ] Implement Caddy forward_auth directive generation
- [ ] Add per-host "Enable Forward Auth" toggle
- [ ] Create forward auth provider templates (Authelia, Authentik, Pomerium)
- [ ] Add custom forward auth endpoint configuration
- [ ] Implement trusted header forwarding
- [ ] Add bypass rules (for API endpoints, webhooks)
- [ ] Create forward auth testing tool

**Acceptance Criteria**:
- Can enable forward auth per proxy host
- Templates work with popular SSO providers
- Protected services require authentication
- API endpoints can bypass auth

---

#### Issue #12: Built-in OAuth/OIDC Server (SSO - Plus Feature)
**Priority**: `high`
**Labels**: `beta`, `security`, `high`, `sso`, `plus`
**Description**:
Implement internal authentication server using caddy-security plugin.

**Tasks**:
- [ ] Integrate caddy-security plugin
- [ ] Design user/group management UI
- [ ] Implement local user creation with password hashing
- [ ] Add OAuth/OIDC provider configuration
- [ ] Create application registration system
- [ ] Implement consent screen
- [ ] Add 2FA/TOTP support
- [ ] Create identity provider dashboard

**Acceptance Criteria**:
- Can create local users for authentication
- Can protect services with built-in SSO
- 2FA works correctly
- External OIDC providers can be configured

---

#### Issue #13: HTTP Basic Authentication
**Priority**: `high`
**Labels**: `beta`, `security`, `high`
**Description**:
Implement simple HTTP Basic Auth for services.

**Tasks**:
- [ ] Add "Enable Basic Auth" toggle per host
- [ ] Create username/password input with hashing
- [ ] Implement Caddy basicauth directive generation
- [ ] Add multiple user support per host
- [ ] Create basic auth realm configuration
- [ ] Implement password strength validation
- [ ] Add basic auth testing tool

**Acceptance Criteria**:
- Basic auth protects services
- Multiple users per host supported
- Passwords securely hashed
- Browser prompts correctly

---

#### Issue #14: IP-based Access Control Lists (ACLs)
**Priority**: `high`
**Labels**: `beta`, `security`, `high`
**Description**:
Implement IP whitelisting/blacklisting and geo-blocking.

**Tasks**:
- [ ] Design ACL management UI
- [ ] Implement IP/CIDR whitelist per host
- [ ] Add blacklist functionality
- [ ] Implement "Local Network Only" toggle (RFC1918)
- [ ] Add geo-blocking with country selection
- [ ] Integrate MaxMind GeoIP2 database
- [ ] Create ACL templates (local only, US only, etc.)
- [ ] Implement ACL testing tool

**Acceptance Criteria**:
- Can restrict access by IP/CIDR
- Local network toggle works
- Geo-blocking blocks correctly
- ACLs apply to specific hosts

---

### 🛡️ THREAT PROTECTION (Beta - Phase 2)

#### Issue #15: CrowdSec Integration
**Priority**: `critical`
**Labels**: `beta`, `security`, `critical`, `crowdsec`
**Description**:
Integrate CrowdSec for active threat protection.

**Tasks**:
- [ ] Design CrowdSec integration architecture
- [ ] Implement CrowdSec bouncer for Caddy
- [ ] Create CrowdSec installation wizard in UI
- [ ] Add CrowdSec status monitoring
- [ ] Implement banned IP dashboard
- [ ] Add manual IP ban/unban functionality
- [ ] Create scenario/collection management UI
- [ ] Add CrowdSec log parsing setup

**Acceptance Criteria**:
- CrowdSec blocks malicious IPs automatically
- Banned IPs visible in dashboard
- Can manually ban/unban IPs
- CrowdSec status visible

---

#### Issue #16: Web Application Firewall (WAF) Integration
**Priority**: `high`
**Labels**: `beta`, `security`, `high`, `waf`, `plus`
**Description**:
Integrate Coraza WAF with OWASP Core Rule Set.

**Tasks**:
- [ ] Integrate caddy-coraza-filter plugin
- [ ] Implement "Enable WAF" toggle per host
- [ ] Add OWASP CRS rule set management
- [ ] Create WAF rule exclusion system (for false positives)
- [ ] Implement WAF logging and alerts
- [ ] Add WAF statistics dashboard
- [ ] Create paranoia level selector
- [ ] Implement custom WAF rules

**Acceptance Criteria**:
- WAF blocks common attacks (SQLi, XSS)
- Can enable/disable per host
- False positives manageable
- WAF events logged and visible

---

#### Issue #17: Rate Limiting & DDoS Protection
**Priority**: `high`
**Labels**: `beta`, `security`, `high`
**Description**:
Implement request rate limiting per host.

**Tasks**:
- [ ] Implement Caddy rate_limit directive integration
- [ ] Create rate limit preset templates (login, API, standard)
- [ ] Add custom rate limit configuration
- [ ] Implement per-IP rate limiting
- [ ] Add per-endpoint rate limits
- [ ] Create rate limit bypass list (trusted IPs)
- [ ] Add rate limit violation logging
- [ ] Implement rate limit testing tool

**Acceptance Criteria**:
- Rate limits prevent brute force
- Presets work correctly
- Legitimate traffic not affected
- Rate limit hits logged

---

#### Issue #18: HTTP Security Headers
**Priority**: `medium`
**Labels**: `beta`, `security`, `medium`
**Description**:
Implement automatic security header injection.

**Tasks**:
- [ ] Create security headers configuration UI
- [ ] Implement HSTS with preload support
- [ ] Add Content-Security-Policy builder
- [ ] Implement X-Frame-Options (DENY/SAMEORIGIN)
- [ ] Add X-Content-Type-Options (nosniff)
- [ ] Implement Referrer-Policy configuration
- [ ] Add Permissions-Policy headers
- [ ] Create security header presets (basic, strict, paranoid)
- [ ] Implement security score calculator

**Acceptance Criteria**:
- Security headers automatically added
- CSP configurable without breaking sites
- Presets available for easy setup
- Security score shown in UI

---

### 🚦 TRAFFIC & TLS MANAGEMENT (Beta - Phase 3)

#### Issue #19: DNS Challenge Support for Wildcard Certificates
**Priority**: `critical`
**Labels**: `beta`, `ssl`, `critical`
**Description**:
Implement DNS challenge for users behind firewalls.

**Tasks**:
- [ ] Design DNS provider configuration UI
- [ ] Implement DNS provider dropdown (Cloudflare, Route53, etc.)
- [ ] Add API token/credential secure storage
- [ ] Implement wildcard certificate support
- [ ] Add DNS provider testing/validation
- [ ] Create DNS challenge troubleshooting guide
- [ ] Implement credential encryption at rest
- [ ] Add popular provider quick-setup guides

**Acceptance Criteria**:
- Wildcard certificates work
- Popular DNS providers supported
- Credentials stored securely
- Clear error messages for failures

---

#### Issue #20: Custom Certificate Upload & Management
**Priority**: `high`
**Labels**: `beta`, `ssl`, `high`
**Description**:
Allow users to upload their own certificates.

**Tasks**:
- [ ] Create certificate upload UI
- [ ] Implement certificate validation (format, expiry)
- [ ] Add private key upload with encryption
- [ ] Support certificate chains/intermediates
- [ ] Implement certificate assignment to hosts
- [ ] Add expiry warnings for custom certificates
- [ ] Create certificate export functionality
- [ ] Support PEM, PFX, DER formats

**Acceptance Criteria**:
- Can upload custom certificates
- Certificates validated before acceptance
- Private keys securely stored
- Expiry warnings work

---

#### Issue #21: Client Certificate Authentication (mTLS)
**Priority**: `medium`
**Labels**: `beta`, `security`, `medium`, `plus`
**Description**:
Implement mutual TLS for zero-trust setups.

**Tasks**:
- [ ] Design mTLS configuration UI
- [ ] Implement client CA upload
- [ ] Add "Require Client Certificate" toggle per host
- [ ] Create client certificate generation tool
- [ ] Implement certificate revocation checking
- [ ] Add client certificate verification logging
- [ ] Create mTLS troubleshooting guide
- [ ] Implement certificate DN-based authorization

**Acceptance Criteria**:
- mTLS protects sensitive services
- Client certificates required when enabled
- Invalid certificates rejected
- mTLS events logged

---

#### Issue #22: Advanced TLS Configuration
**Priority**: `medium`
**Labels**: `beta`, `ssl`, `medium`
**Description**:
Expose advanced TLS options for power users.

**Tasks**:
- [ ] Add TLS version selector (1.2, 1.3)
- [ ] Implement cipher suite configuration
- [ ] Add ALPN protocol configuration
- [ ] Implement OCSP stapling toggle
- [ ] Add certificate transparency logging
- [ ] Create TLS security score calculator
- [ ] Implement SSL Labs integration for testing
- [ ] Add "Modern/Intermediate/Old" compatibility presets

**Acceptance Criteria**:
- TLS configuration customizable
- Presets available for common scenarios
- Security score displayed
- Changes applied correctly

---

### 📊 MONITORING & LOGGING (Beta - Phase 4)

#### Issue #23: Enhanced Dashboard with Statistics
**Priority**: `high`
**Labels**: `beta`, `monitoring`, `high`
**Description**:
Create a comprehensive dashboard with service statistics.

**Tasks**:
- [ ] Design dashboard layout with widgets
- [ ] Implement request count statistics (24h, 7d, 30d)
- [ ] Add top accessed hosts chart
- [ ] Create HTTP status code distribution chart
- [ ] Implement traffic volume graphs
- [ ] Add certificate expiry warnings
- [ ] Create service health indicators
- [ ] Implement real-time update with WebSockets

**Acceptance Criteria**:
- Dashboard shows key metrics at a glance
- Charts update in real-time
- Performance remains smooth
- Mobile responsive

---

#### Issue #24: CrowdSec Dashboard Integration
**Priority**: `high`
**Labels**: `beta`, `monitoring`, `high`, `crowdsec`
**Description**:
Embed CrowdSec metrics and decisions in the UI.

**Tasks**:
- [ ] Create CrowdSec metrics dashboard tab
- [ ] Display active bans with reasons
- [ ] Show CrowdSec scenarios triggered
- [ ] Implement ban history timeline
- [ ] Add top attacking IPs chart
- [ ] Create attack type breakdown
- [ ] Implement CrowdSec alert notifications
- [ ] Add ban export functionality

**Acceptance Criteria**:
- CrowdSec activity visible in UI
- Bans displayed with context
- Historical data available
- Notifications work

---

#### Issue #25: GoAccess Integration for Analytics
**Priority**: `medium`
**Labels**: `beta`, `monitoring`, `medium`
**Description**:
Integrate GoAccess for beautiful log analysis.

**Tasks**:
- [ ] Integrate GoAccess binary or build from source
- [ ] Create "Generate Report" button in UI
- [ ] Implement GoAccess HTML report embedding
- [ ] Add scheduled report generation
- [ ] Implement report caching for performance
- [ ] Add report date range selector
- [ ] Create per-host GoAccess reports
- [ ] Implement report export functionality

**Acceptance Criteria**:
- GoAccess reports generated successfully
- Reports embedded in UI
- Performance acceptable
- Reports accurate

---

#### Issue #26: Live Log Viewer & Search
**Priority**: `high`
**Labels**: `beta`, `monitoring`, `high`
**Description**:
Create a powerful log viewer with real-time updates.

**Tasks**:
- [ ] Implement log streaming via WebSocket
- [ ] Create log viewer UI with syntax highlighting
- [ ] Add log level filtering (error, warn, info, debug)
- [ ] Implement full-text log search
- [ ] Add timestamp range filtering
- [ ] Create per-host log filtering
- [ ] Implement log export (CSV, JSON)
- [ ] Add "Follow" mode for real-time tailing

**Acceptance Criteria**:
- Logs stream in real-time
- Search is fast and accurate
- Filters work correctly
- Export works for large logs

---

#### Issue #27: Alerting & Notifications System
**Priority**: `medium`
**Labels**: `beta`, `monitoring`, `medium`
**Description**:
Implement alerting for critical events.

**Tasks**:
- [ ] Design notification system architecture
- [ ] Implement email notifications (SMTP)
- [ ] Add webhook notifications (Discord, Slack, custom)
- [ ] Create alert rules (certificate expiry, service down, ban threshold)
- [ ] Implement notification testing
- [ ] Add notification history log
- [ ] Create alert rule templates
- [ ] Implement notification rate limiting

**Acceptance Criteria**:
- Alerts sent for critical events
- Multiple notification channels supported
- Alert rules customizable
- No spam (rate limited)

---

### 🎨 USER EXPERIENCE (Beta - Phase 5)

#### Issue #28: Onboarding Wizard & First-Time Setup
**Priority**: `high`
**Labels**: `beta`, `ui`, `high`
**Description**:
Create a smooth first-time user experience.

**Tasks**:
- [ ] Design multi-step setup wizard
- [ ] Implement admin account creation
- [ ] Add domain/email configuration
- [ ] Create first proxy host tutorial
- [ ] Implement DNS challenge provider setup wizard
- [ ] Add CrowdSec installation wizard
- [ ] Create "Quick Start" templates (Plex, Sonarr, etc.)
- [ ] Implement interactive tooltips/hints

**Acceptance Criteria**:
- New users guided through setup
- Can create first proxy host easily
- Common services have templates
- Setup completes successfully

---

#### Issue #29: Import from Nginx Proxy Manager
**Priority**: `medium`
**Labels**: `beta`, `feature`, `medium`
**Description**:
Allow users to migrate from NPM.

**Tasks**:
- [ ] Design NPM database import system
- [ ] Parse NPM SQLite/MySQL database
- [ ] Map NPM proxy hosts to Caddy config
- [ ] Import SSL certificates
- [ ] Convert NPM access lists to ACLs
- [ ] Create import preview/validation
- [ ] Implement backup before import
- [ ] Create migration guide documentation

**Acceptance Criteria**:
- Can import NPM database
- Proxy hosts converted correctly
- Certificates imported
- Rollback available if issues

---

#### Issue #30: Configuration Backup & Restore
**Priority**: `high`
**Labels**: `beta`, `feature`, `high`
**Description**:
Implement comprehensive backup system.

**Tasks**:
- [ ] Create backup format (database + configs + certificates)
- [ ] Implement one-click backup button
- [ ] Add scheduled automatic backups
- [ ] Create restore functionality with validation
- [ ] Implement backup encryption (optional)
- [ ] Add backup to remote storage (S3, SFTP)
- [ ] Create backup history management
- [ ] Implement disaster recovery guide

**Acceptance Criteria**:
- Backups contain all critical data
- Restore works flawlessly
- Automatic backups run on schedule
- Remote backup options available

---

#### Issue #31: Multi-language Support (i18n)
**Priority**: `low`
**Labels**: `beta`, `ui`, `low`
**Description**:
Internationalize the interface.

**Tasks**:
- [ ] Implement i18n framework
- [ ] Extract all strings to translation files
- [ ] Add language selector in settings
- [ ] Translate to: Spanish, French, German, Chinese
- [ ] Create translation contribution guide
- [ ] Implement date/time localization
- [ ] Add RTL language support framework
- [ ] Create translation verification tests

**Acceptance Criteria**:
- UI supports multiple languages
- Language switching works instantly
- All strings translatable
- Community can contribute translations

---

#### Issue #32: Dark Mode & Theme Customization
**Priority**: `low`
**Labels**: `beta`, `ui`, `low`
**Description**:
Implement theme system beyond basic dark/light.

**Tasks**:
- [ ] Create comprehensive theme system
- [ ] Implement custom color picker
- [ ] Add pre-built theme gallery
- [ ] Create theme import/export
- [ ] Add logo customization
- [ ] Implement CSS custom properties
- [ ] Create theme preview
- [ ] Add "Follow System" option

**Acceptance Criteria**:
- Themes change entire UI
- Custom themes saveable
- Pre-built themes available
- System theme sync works

---

### 🔧 ADVANCED FEATURES (Post-Beta)

#### Issue #33: API & CLI Tools
**Priority**: `medium`
**Labels**: `post-beta`, `feature`, `medium`
**Description**:
Expose REST API and CLI for automation.

**Tasks**:
- [ ] Design RESTful API
- [ ] Implement API authentication (API keys)
- [ ] Create API documentation (OpenAPI/Swagger)
- [ ] Build CLI tool for management
- [ ] Implement Terraform provider
- [ ] Create API client libraries (Python, Go)
- [ ] Add API rate limiting
- [ ] Create API usage examples

**Acceptance Criteria**:
- Full API coverage of UI features
- API documented comprehensively
- CLI tool functional
- Automation possible

---

#### Issue #34: High Availability & Clustering
**Priority**: `low`
**Labels**: `post-beta`, `feature`, `low`, `enterprise`
**Description**:
Support multiple Caddy instances with shared config.

**Tasks**:
- [ ] Design distributed architecture
- [ ] Implement config synchronization
- [ ] Add load balancer support
- [ ] Create shared certificate storage
- [ ] Implement cluster health monitoring
- [ ] Add automatic failover
- [ ] Create split-brain prevention
- [ ] Document HA deployment

**Acceptance Criteria**:
- Multiple instances share config
- Failover works automatically
- No downtime during updates
- Certificates shared correctly

---

#### Issue #35: Plugin System & Marketplace
**Priority**: `low`
**Labels**: `post-beta`, `feature`, `low`
**Description**:
Allow community extensions and plugins.

**Tasks**:
- [ ] Design plugin architecture
- [ ] Implement plugin loader
- [ ] Create plugin API documentation
- [ ] Build plugin marketplace UI
- [ ] Implement plugin sandboxing
- [ ] Add plugin installation/update system
- [ ] Create example plugins
- [ ] Implement plugin review process

**Acceptance Criteria**:
- Plugins extend functionality
- Marketplace browseable
- Plugins installable with one click
- Security maintained

---

#### Issue #36: Advanced Stream (TCP/UDP) Proxying
**Priority**: `medium`
**Labels**: `post-beta`, `feature`, `medium`
**Description**:
Support non-HTTP protocols via Caddy layer4 plugin.

**Tasks**:
- [ ] Integrate caddy-layer4 plugin
- [ ] Design TCP/UDP proxy UI
- [ ] Implement port mapping configuration
- [ ] Add SNI-based routing for TCP
- [ ] Create protocol detection
- [ ] Implement connection logging
- [ ] Add common protocol templates (SSH, database, etc.)
- [ ] Create stream proxy testing tool

**Acceptance Criteria**:
- Can proxy TCP/UDP services
- SNI routing works
- Common protocols templated
- Performance acceptable

---

### 📚 DOCUMENTATION & TESTING (Continuous)

#### Issue #37: Comprehensive Documentation
**Priority**: `high`
**Labels**: `documentation`, `high`
**Description**:
Create user and developer documentation.

**Tasks**:
- [ ] Write installation guide
- [ ] Create quick start tutorial
- [ ] Document all features with screenshots
- [ ] Create troubleshooting guide
- [ ] Write security best practices guide
- [ ] Create API documentation
- [ ] Write contributing guide
- [ ] Create video tutorials

**Acceptance Criteria**:
- All features documented
- Screenshots up to date
- Common issues covered
- Videos available

---

#### Issue #38: Automated Testing Suite
**Priority**: `high`
**Labels**: `testing`, `high`
**Description**:
Implement comprehensive test coverage.

**Tasks**:
- [ ] Set up testing framework
- [ ] Write unit tests (backend)
- [ ] Write integration tests (API)
- [ ] Create end-to-end tests (UI)
- [ ] Implement CI/CD pipeline
- [ ] Add test coverage reporting
- [ ] Create performance benchmarks
- [ ] Implement security scanning

**Acceptance Criteria**:
- >80% code coverage
- CI runs on every commit
- E2E tests cover critical paths
- Security scans pass

---

#### Issue #39: Community & Support Infrastructure
**Priority**: `medium`
**Labels**: `community`, `medium`
**Description**:
Build community support channels.

**Tasks**:
- [ ] Set up Discord/Matrix server
- [ ] Create GitHub Discussions
- [ ] Set up forum (Discourse)
- [ ] Create contribution templates
- [ ] Implement issue templates
- [ ] Create PR review process
- [ ] Set up community guidelines
- [ ] Create roadmap page

**Acceptance Criteria**:
- Community channels active
- Clear contribution process
- Issue templates helpful
- Roadmap transparent

---

#### Issue #40: Performance Optimization & Benchmarking
**Priority**: `medium`
**Labels**: `performance`, `medium`
**Description**:
Ensure CaddyProxyManager+ performs well under load.

**Tasks**:
- [ ] Create performance benchmark suite
- [ ] Profile database queries
- [ ] Optimize Caddyfile generation
- [ ] Implement caching where appropriate
- [ ] Test with 100+ proxy hosts
- [ ] Optimize frontend bundle size
- [ ] Test on low-resource devices (Raspberry Pi)
- [ ] Document performance characteristics

**Acceptance Criteria**:
- Handles 100+ hosts smoothly
- Config reload <1 second
- UI remains responsive
- Works on Raspberry Pi 4

---

## Label Definitions

### Priority Labels
- **critical**: Must have for the release, blocks other work
- **high**: Important feature, should be included
- **medium**: Nice to have, can be deferred
- **low**: Future enhancement, not urgent

### Category Labels
- **alpha**: Part of initial alpha release
- **beta**: Part of beta release
- **post-beta**: Post-beta enhancement
- **architecture**: System design and structure
- **backend**: Server-side code
- **frontend**: UI/UX code
- **feature**: New functionality
- **security**: Security-related
- **ssl**: SSL/TLS certificates
- **sso**: Single Sign-On
- **waf**: Web Application Firewall
- **crowdsec**: CrowdSec integration
- **caddy**: Caddy-specific
- **database**: Database-related
- **ui**: User interface
- **deployment**: Docker, installation
- **monitoring**: Logging and statistics
- **documentation**: Docs and guides
- **testing**: Test suite
- **performance**: Optimization
- **community**: Community building
- **plus**: Premium/"Plus" feature
- **enterprise**: Enterprise-grade feature

---

## Milestones Summary

### Alpha (Issues #1-10)
**Goal**: Basic proxy management with automatic HTTPS
**Target**: 2-3 months
**Key Features**:
- Core proxy host management
- Automatic HTTPS
- Basic web UI
- Docker deployment
- User authentication

### Beta (Issues #11-32)
**Goal**: Full security suite and monitoring
**Target**: 4-6 months
**Key Features**:
- Forward Auth & built-in SSO
- CrowdSec integration
- WAF with OWASP CRS
- Rate limiting
- IP ACLs & geo-blocking
- DNS challenge (wildcard certs)
- Enhanced logging & monitoring
- GoAccess integration

### Post-Beta (Issues #33-36)
**Goal**: Advanced features and enterprise capabilities
**Target**: 6+ months
**Key Features**:
- REST API & CLI
- High availability
- Plugin system
- Stream (TCP/UDP) proxying

### Continuous (Issues #37-40)
**Goal**: Documentation, testing, performance
**Ongoing throughout all phases**

---

## Quick Start Implementation Order

For maximum value delivery, implement in this order:

1. **Week 1-2**: Issues #1, #2, #3 (Foundation)
2. **Week 3-4**: Issues #4, #5, #6 (Basic UI + Proxy Management)
3. **Week 5-6**: Issues #7, #10 (Auth + Docker)
4. **Week 7-8**: Issues #8, #9 (Logging + Settings)
5. **Alpha Release** 🎉
6. **Month 3-4**: Issues #11, #15, #19 (SSO, CrowdSec, DNS Challenge)
7. **Month 4-5**: Issues #16, #17, #18, #14 (WAF, Rate Limit, ACLs)
8. **Month 5-6**: Issues #23, #24, #26, #28 (Dashboard, Monitoring, UX)
9. **Beta Release** 🎉

---

## Success Metrics

### Alpha Success Criteria
- [ ] Can replace basic NPM setup
- [ ] Automatic HTTPS works flawlessly
- [ ] Docker deployment in <5 minutes
- [ ] Manages 10+ services easily

### Beta Success Criteria
- [ ] Advanced security features functional
- [ ] CrowdSec blocks real attacks
- [ ] SSO protects services
- [ ] Community adoption started
- [ ] <10 critical bugs reported

### V1.0 Success Criteria
- [ ] 1000+ Docker Hub pulls
- [ ] Active community (Discord/forum)
- [ ] Comprehensive documentation
- [ ] <5 open critical bugs
- [ ] Featured in self-hosted communities

---

## Risk Assessment

### High Risk
1. **Caddy API Breaking Changes**: Mitigation: Pin Caddy version, test upgrades thoroughly
2. **Security Vulnerabilities**: Mitigation: Regular audits, security-focused development
3. **Performance at Scale**: Mitigation: Early benchmarking, optimization sprints

### Medium Risk
1. **Feature Creep**: Mitigation: Strict milestone adherence, MVP mindset
2. **Community Adoption**: Mitigation: Early marketing, r/selfhosted presence
3. **NPM Feature Parity**: Mitigation: Don't aim for 100% parity, focus on unique value

### Low Risk
1. **Caddy Compatibility**: Caddy's API is stable
2. **Technology Stack**: Proven technologies
3. **Docker Deployment**: Well-understood deployment model

---

## Marketing & Launch Strategy

### Alpha Launch
- [ ] Post on r/selfhosted
- [ ] Post on r/homelab
- [ ] Tweet from personal account
- [ ] Create project website
- [ ] YouTube demo video

### Beta Launch
- [ ] Write blog post: "Why We Built This"
- [ ] Submit to Awesome Self-Hosted
- [ ] Post on Hacker News
- [ ] LinuxServer.io container consideration
- [ ] Unraid Community Apps submission

### V1.0 Launch
- [ ] Press release to tech blogs
- [ ] Major YouTube creators (NetworkChuck, Techno Tim)
- [ ] Conference talk proposals
- [ ] Podcast tour (Self-Hosted, Linux Unplugged)

---

_This planning document is living and will be updated as development progresses._
