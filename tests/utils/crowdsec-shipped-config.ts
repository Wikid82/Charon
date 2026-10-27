/**
 * Restores the CrowdSec configuration files that a fresh instance ships with.
 *
 * Import specs replace config/config.yaml with minimal archives. Those archives have no
 * config_paths section, and the engine refuses to start from them: it exits at once with
 * "while loading configuration file: no configuration paths provided", and a start request then
 * waits out its whole LAPI timeout. Specs that import or write CrowdSec files call
 * `restoreShippedCrowdsecConfig` from an afterEach hook. Specs that start the engine call it before
 * they start it, so they do not depend on which spec ran earlier.
 *
 * The contents are the files the container entrypoint produces from /etc/crowdsec.dist: config.yaml
 * with the runtime paths and LAPI port, and the three companion files copied unchanged.
 */
import { expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

const FILE_ROUTE = '/api/v1/admin/crowdsec/file';
const RESTORE_TIMEOUT_MS = 60_000;

/** Shipped contents keyed by the path the file API lists and reads. */
export const SHIPPED_CROWDSEC_FILES: Readonly<Record<string, string>> = {
  'config/config.yaml': `common:
  daemonize: true
  log_media: file
  log_level: info
  log_dir: /var/log/crowdsec/
  log_max_size: 20
  compress_logs: true
  log_max_files: 10
config_paths:
  config_dir: /etc/crowdsec/
  data_dir: /app/data/crowdsec/data/
  simulation_path: /etc/crowdsec/simulation.yaml
  hub_dir: /etc/crowdsec/hub/
  index_path: /etc/crowdsec/hub/.index.json
  notification_dir: /etc/crowdsec/notifications/
  plugin_dir: /usr/local/lib/crowdsec/plugins/
crowdsec_service:
  #console_context_path: /etc/crowdsec/console/context.yaml
  acquisition_path: /etc/crowdsec/acquis.yaml
  acquisition_dir: /etc/crowdsec/acquis.d
  parser_routines: 1
cscli:
  hub_branch: master
  output: human
  color: auto
db_config:
  log_level: info
  type: sqlite
  db_path: /app/data/crowdsec/data/crowdsec.db
  #max_open_conns: 100
  #user:
  #password:
  #db_name:
  #host:
  #port:
  flush:
    max_items: 5000
    max_age: 7d
plugin_config:
  user: nobody # plugin process would be ran on behalf of this user
  group: nogroup # plugin process would be ran on behalf of this group
api:
  client:
    insecure_skip_verify: false
    credentials_path: /etc/crowdsec/local_api_credentials.yaml
  server:
    log_level: info
    listen_uri: 127.0.0.1:8085
    profiles_path: /etc/crowdsec/profiles.yaml
    console_path: /etc/crowdsec/console.yaml
    online_client: # Central API credentials (to push signals and receive bad IPs)
      credentials_path: /etc/crowdsec/online_api_credentials.yaml
    trusted_ips: # IP ranges, or IPs which can have admin API access
      - 127.0.0.1
      - ::1
#    tls:
#      cert_file: /etc/crowdsec/ssl/cert.pem
#      key_file: /etc/crowdsec/ssl/key.pem
prometheus:
  enabled: true
`,
  'config/profiles.yaml': `name: default_ip_remediation
#debug: true
filters:
 - Alert.Remediation == true && Alert.GetScope() == "Ip"
decisions:
 - type: ban
   duration: 4h
#duration_expr: Sprintf('%dh', (GetDecisionsCount(Alert.GetValue()) + 1) * 4)
# notifications:
#   - slack_default  # Set the webhook in /etc/crowdsec/notifications/slack.yaml before enabling this.
#   - splunk_default # Set the splunk url and token in /etc/crowdsec/notifications/splunk.yaml before enabling this.
#   - http_default   # Set the required http parameters in /etc/crowdsec/notifications/http.yaml before enabling this.
#   - email_default  # Set the required email parameters in /etc/crowdsec/notifications/email.yaml before enabling this.
on_success: break
---
name: default_range_remediation
#debug: true
filters:
 - Alert.Remediation == true && Alert.GetScope() == "Range"
decisions:
 - type: ban
   duration: 4h
#duration_expr: Sprintf('%dh', (GetDecisionsCount(Alert.GetValue()) + 1) * 4)
# notifications:
#   - slack_default  # Set the webhook in /etc/crowdsec/notifications/slack.yaml before enabling this.
#   - splunk_default # Set the splunk url and token in /etc/crowdsec/notifications/splunk.yaml before enabling this.
#   - http_default   # Set the required http parameters in /etc/crowdsec/notifications/http.yaml before enabling this.
#   - email_default  # Set the required email parameters in /etc/crowdsec/notifications/email.yaml before enabling this.
on_success: break
`,
  'config/acquis.yaml': `# Charon/Caddy Log Acquisition Configuration
# This file tells CrowdSec what logs to monitor

# Caddy access logs (JSON format)
source: file
filenames:
  - /var/log/caddy/access.log
  - /var/log/caddy/*.log
labels:
  type: caddy
`,
  'config/simulation.yaml': `simulation: false
# exclusions:
#  - crowdsecurity/ssh-bf
`,
};

/** Writes one shipped file unless the instance already holds exactly that content. */
async function restoreFile(request: APIRequestContext, path: string, content: string): Promise<void> {
  const current = await request.get(`${FILE_ROUTE}?path=${encodeURIComponent(path)}`);
  if (current.ok() && ((await current.json()) as { content?: string }).content === content) {
    return;
  }

  // A CrowdSec job that holds the instance answers 409 crowdsec_busy, so the write is retried until it is accepted.
  await expect
    .poll(
      async () => (await request.post(FILE_ROUTE, { data: { path, content }, timeout: RESTORE_TIMEOUT_MS })).status(),
      { message: `restore ${path}`, timeout: RESTORE_TIMEOUT_MS },
    )
    .toBe(200);
}

/**
 * Puts the shipped CrowdSec configuration back, so a real engine start works whatever spec ran before.
 * Files that already match are not rewritten, which keeps backup files from piling up.
 */
export async function restoreShippedCrowdsecConfig(request: APIRequestContext): Promise<void> {
  for (const [path, content] of Object.entries(SHIPPED_CROWDSEC_FILES)) {
    await restoreFile(request, path, content);
  }
}
