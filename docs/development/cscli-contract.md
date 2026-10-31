# The cscli contract

Charon does not talk to CrowdSec directly for most tasks. It runs the `cscli` command
that ships in the Charon image and reads what it prints. If `cscli` changes its commands
or output, the CrowdSec pages can quietly break. This page lists what Charon relies on and
how that is checked.

## Bundled version

The image bundles CrowdSec **v1.8.2-rc1** (at the time of writing). The version is set by
`ARG CROWDSEC_VERSION` at the top of the `Dockerfile`; Renovate tracks it. The image build
runs `cscli version` and fails if it does not report that version.

## Commands Charon runs

Handlers run `cscli` through one helper, `runCSCLI`
(`backend/internal/api/handlers/crowdsec_cscli.go`). It puts `-c <config>` in front of the
arguments when the engine's config file exists and runs the command under a deadline:
**30 s** for listings (`cscliListTimeout`) and **15 s** for add and delete
(`cscliChangeTimeout`). A command that runs out of time becomes a coded 504
(`cscli_timeout`, "CrowdSec did not answer in time"). The output we read is stdout and
stderr combined. Every handler call goes through the helper, including bouncer key
validation and registration (5 s and 10 s deadlines); a test fails if a handler calls
`CmdExec.Execute("cscli"` outside it. Listing endpoints answer soft errors with an empty
list and an `error` code: `cscli_timeout`, `cscli_output_too_large` or
`cscli_unavailable`; the UI shows a passive note.

Every `cscli` call, including hub `inspect`, verify and the startup bouncer commands, passes
`-c <engine config>` so it reads the same configuration and hub as the running engine. The one exception is the
`cscli version` presence probe, which needs no configuration.

| Command | How the result is used |
| --- | --- |
| `cscli decisions list -o json` | Parsed as a list of alerts, each with nested `decisions`; the decisions table shows one row per nested decision. Empty output, `null` and `[]` mean "no decisions". |
| `cscli decisions add -i <ip> -d <duration> -R <reason> -t ban` | Exit status only. A range uses `-r <range>` in place of `-i` (see "Ban and unban rules"). |
| `cscli decisions delete -i <ip>` | Exit status, plus the `N decision(s) deleted` line. A range uses `-r <range>`. |
| `cscli -c <config> config show` | Exit status only. This is the configuration check (exit 0 means valid). **`cscli config check` does not exist**; it prints usage text. |
| `cscli bouncers list -o json` | Parsed for bouncer names (to see if Charon's bouncer is registered). |
| `cscli bouncers add <name> -o raw` | Prints only the generated key, with no trailing newline. |
| `cscli bouncers delete <name>` | Exit status only (done before re-adding). |
| `cscli alerts list -o json` | Parsed as a list of alerts for the dashboard, then reduced to the typed alert view (see "Alerts"). Deleted decisions stay visible here. |
| `cscli console status -o json` | The `console.enrolled` boolean decides enrollment. If the output is not JSON, the text is matched for enrolled / not enrolled wording as before. |
| `cscli lapi status` | Exit status only (is the local API up). |
| `cscli capi status` | Exit status; the text is shown in diagnostics. |
| `cscli version` | Exit status only (is `cscli` present). |
| `cscli hub list [-a] -o json` | Parsed for installed items and the hub index. |
| `cscli hub update` | Exit status only. |
| `cscli <type> inspect <name> -o json` | Parsed for the item's state. `path` is the item's own file relative to the hub directory (for example `collections/crowdsecurity/nginx.yaml`) and is what a preset apply saves before an in-place upgrade; `local_path` is the installed link in the configuration and is only a fallback. |
| `cscli <type> install <name>` | Exit status. **`cscli hub install` does not exist**; items are installed per type (`collections`, `parsers`, ...). |
| `cscli <type> upgrade <name>` | Exit status. |
| `cscli -c <derived> ...` with `cscli.__hub_url_template__` set (hidden test hook; used for the hub mirror retry) | Charon runs the hub commands above a second time under a derived copy of the configuration when the hub address fails. Verified on cscli 1.8.2 only; re-check on every CrowdSec bump. A cscli that drops the key runs the same request once more and reports the original error. |

Not every path in the table is fixture-covered (see the recordings below); the fixtures
are the authority for what each covered command prints.

## Output rules

- **Progress lines before JSON.** With `-o json`, `cscli` can print lines such as
  `Loaded: 168 parsers, ...` before the document. Use `crowdsec.UnmarshalCSCLIOutput`
  (`backend/internal/crowdsec/cscli_output.go`) to decode such output: it takes the first
  value that starts a line with `{` or `[` and decodes completely, and ignores the rest.
  Do not call `json.Unmarshal` on raw combined output.
- **Output cap.** Every executor reads output through `limitedBuffer`
  (`backend/internal/crowdsec/cscli_exec.go`, via `CombinedOutputLimited`). The cap is the parser limit of 32 MiB plus 1 MiB.
  Past it the command and its process group are killed and `ErrOutputLimit` is returned.
  The error never carries the output.

- **Oversized output.** `UnmarshalCSCLIOutput` returns `ErrCSCLIOutputTooLarge` above the
  parser limit. Handlers answer it with the same soft error as the executor cap.

## Ban and unban rules

Recorded against `v1.8.2-rc1` (`decisions_add_*`, `decisions_delete_*`):

- `-i` takes one address and rejects a range (`... is not a valid ip`); `-r` takes a range
  and rejects a single address (`... is not a valid range`). Charon picks the flag after
  parsing the value itself.
- A value that starts with `-` is taken as the flag's value, not as a new flag, and then
  fails validation. Charon still rejects such values before running `cscli`.
- IPv6 addresses are accepted; a zone (`fe80::1%eth0`) is refused.
- Durations: Go forms (`4h`, `30m`, `90s`, `1h30m`, `2.5h`) and whole days (`1d`) are
  accepted. `1w`, upper-case units (`4H`) and absurd values (`9999999h`) are refused by
  `cscli`. `cscli` also accepts `0` and negative values; Charon refuses them and allows
  only 1 minute to 365 days.
- `decisions delete` prints `N decision(s) deleted` and exits 0 even when `N` is 0.
  Charon reads that count: 0 becomes a "not found" answer. Unban matches the exact value
  only. A successful unban writes a `security_decisions` row with action `unblock`, which
  the top-IPs and summary queries exclude.

## Alerts

Charon passes the requested range as `--since <range>`. `cscli` has no offset flag, so
Charon fetches `limit + offset` alerts (at most 1000) and drops the first `offset` itself;
paging is best-effort and the total is the number fetched.

`cscli alerts list -o json` returns a list of alerts. Each has a nested `source` (with
`ip` and `value`) and a nested `decisions` list (with `duration`, `type`, `origin`).
Charon keeps only what the page uses: `id`, `scenario`, `ip`, `message`, `events_count`,
`start_at`, `stop_at`, `created_at`, `duration`, `type`, `origin`. Everything else (for
example `machine_id`) is dropped.

The local API's `GET /v1/alerts` needs a machine login. The bouncer key gets
`401 cookie token is empty` (`lapi_v1_alerts_bouncer_key.txt`), so Charon uses `cscli` for
alerts.

## Direct local API calls

For calls that use the bouncer key (decisions stream, active-decision count), `lapiKey()`
looks in this order: the saved key file (`bouncer_key` in the data folder), then
`CHARON_SECURITY_CROWDSEC_API_KEY`, `CROWDSEC_API_KEY`, `CROWDSEC_BOUNCER_API_KEY`,
`CERBERUS_SECURITY_CROWDSEC_API_KEY`, `CPM_SECURITY_CROWDSEC_API_KEY`. Every other reader of
the key (the key status and bouncer info endpoints, the startup check, the proxy config) uses
the same order through the shared `bouncerkey` package, so they cannot disagree. Key writes
use `crowdsec.SaveBouncerKey`.

## Recorded output (fixtures)

Real output from the bundled `cscli` is stored per version:

- `backend/internal/api/handlers/testdata/cscli/v1.8.2-rc1/` is the complete recording.
- `backend/internal/crowdsec/testdata/cscli/v1.8.2-rc1/` is the subset used by the
  `crowdsec` package.

Each directory has a `README.md` with the format. In short: `<name>.txt` is stdout and
stderr combined, `<name>.exit` is the exit status, and `lapi_v1_*.body.json` are local API
response bodies. Names follow the command (`decisions_list_json` is
`cscli -c <config> decisions list -o json`). The contract tests are
`crowdsec_cscli_contract_test.go` (handlers) and `cscli_contract_test.go` (crowdsec).

`console_status_json_enrolled_synthetic.txt` is **synthetic**: an enrolled console needs an
installation registered with the central API, which cannot be done offline. It is the
recorded not-registered JSON with the enrolled fields set to `true`.

### Never record

- Bouncer keys or any API key (use a placeholder; `bouncers_add_raw.txt` holds one).
- Machine passwords or credentials.
- Real IP addresses or hostnames. Use documentation ranges only: `192.0.2.0/24` and
  `198.51.100.0/24`.

## Re-recording against a new version

1. Start a throwaway container from the new image with its own volume (not your data).
2. Create a machine and start the local API: `cscli machines add <name> --auto`, then
   `crowdsec -no-cs`.
3. Run each command from the table with `-c /etc/crowdsec/config.yaml`, redirecting
   `2>&1` and saving the exit status next to the output. Include the failure cases already
   recorded (local API down, missing or malformed config, duplicate bouncer, missing
   decision). Cut off commands that retry for minutes (local API down) with a timeout.
4. Replace keys with placeholders and addresses with documentation addresses.
5. Save the files under a new `v<version>/` directory and update its `README.md`.

## Checklist when bumping `CROWDSEC_VERSION`

1. Bump the `Dockerfile` and build the image.
2. Re-record the fixtures as above.
3. Run the contract tests:
   `cd backend && go test ./internal/api/handlers ./internal/crowdsec -run 'Contract'`
4. Fix any difference in the code, not in the fixtures.
5. Run the local checks against the real `cscli` in the built image (a typed `<type> install`, decisions
   list/add/delete, configuration check).
