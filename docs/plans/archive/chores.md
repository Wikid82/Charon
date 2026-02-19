# Chores and To-Dos for Charon

## To-Do

- in the add Proxy Host card, the ACL and Security Headers drop downs don't let me select anything.
  - Add e2e tests for Proxy Host add/edit cards to trace and fix this bug 🐛

- Turn Vite back on for Playwright coverage annalysis.

- Extensive e2e testing and triage for all save functions. Users have reported 404 errors when trying to save certain configurations. Need to ensure all save functions work as expected.

- Extensive e2e testing and triage for all import functions. Some users have reported unable to import caddyfiles and crowdsec configs. Need to ensure all import functions work as expected. Ref issue #585

- Extensive e2e testing and triage for Uptime Monitoring. There is a bug that I just cant place my finger on. Proxi Hosts I added after a certain update a while ago only come back as UP if I manually refresh the card. All others still operate as expected. Need to ensure Uptime Monitoring works as expected.

- Notifications should have a "single source of truth" for configuration. Currently, there are two places to configure notifications: the standard notification settings and the security notifications settings. This is redundant and can lead to confusion. Need to consolidate notification settings into a single area and ensure all notifications work as expected. Security notifications need move to the standard notification area. Its redundant to have two places to setup notifications. Need tests to sniff out bugs. Security notifications report a 404 error and users state the discord and gotify messages don't send. Need to ensure Security Notifications work as expected. Ref issue #623

- Admin users are regular users with admin privlages. To prevent confusion, ad min user should be listed in the user managment page. Currently there are two places to manage users: the standard user management page and the admin account page. This is redundant and can lead to confusion. Need to consolidate user management into a single area and ensure all user management functions work as expected. The user account should create cutome privlage tiers that can be assigned to users. The user management page should list all users, including the admin user, and allow for role assignment and permission management. Need to ensure User Management works as expected.

- The job failed due to several Docker errors:

    - Error response from daemon: manifest unknown
        The job is trying to pull a Docker image (likely wikid82/charon:sha-75b26c1) that doesn't exist or wasn't built/pushed correctly.
    - Error response from daemon: No such container: test-container
        The workflow attempts to access a container that was never created because the image pull failed.
    - Error response from daemon: network charon-test-net not found
        It tries to remove or use a Docker network that was never created due to the earlier failures.
      - Ref `docker_build_and_test.yml` workflow.


- Page by Page e2e tests:
    - Dashboard
    - Proxy Hosts
    - Remote Servers
    - Domains
    - Certificates
    - DNS
      - DNS Providers
      - Plugins
    - Uptime **HIGH**
    - Security
      - Dashboard
      - CrowdSec
      - Access Control Lists
      - Rate Limiting
      - WAF
      - Security Headers
      - Encryption
    - Settings
      - System
      - Notifications **CRITICAL** **USER REQUESTED**
      - Email (SMTP)
      - Admin Account
      - Account Managment
    - Tasks
      - Import
      - Caddyfile **CRITICAL** **USER REQUESTED**
        - CrowdSec **CRITICAL** **USER REQUESTED**
      - Backups
      - Logs **MEDIUM**
