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

Commands that take a config use `-c <data dir>/config.yaml` when the file exists. The
output we read is stdout and stderr combined.

| Command | How the result is used |
| --- | --- |
| `cscli decisions list -o json` | Parsed as a list of alerts, each with nested `decisions`; the decisions table shows one row per nested decision. Empty output, `null` and `[]` mean "no decisions". |
| `cscli decisions add -i <ip> -d <duration> -R <reason> -t ban` | Exit status only. |
| `cscli decisions delete -i <ip>` | Exit status only. |
| `cscli -c <config> config show` | Exit status only. This is the configuration check (exit 0 means valid). **`cscli config check` does not exist**; it prints usage text. |
| `cscli bouncers list -o json` | Parsed for bouncer names (to see if Charon's bouncer is registered). |
| `cscli bouncers add <name> -o raw` | Prints only the generated key, with no trailing newline. |
| `cscli bouncers delete <name>` | Exit status only (done before re-adding). |
| `cscli alerts list -o json` | Parsed as a list of alerts for the dashboard. Deleted decisions stay visible here. |
| `cscli console status` | Text matched for enrolled / not enrolled wording. |
| `cscli lapi status` | Exit status only (is the local API up). |
| `cscli capi status` | Exit status; the text is shown in diagnostics. |
| `cscli version` | Exit status only (is `cscli` present). |
| `cscli hub list [-a] -o json` | Parsed for installed items and the hub index. |
| `cscli hub update` | Exit status only. |
| `cscli <type> inspect <name> -o json` | Parsed for the item's state. |
| `cscli <type> install <name>` | Exit status. **`cscli hub install` does not exist**; items are installed per type (`collections`, `parsers`, ...). |
| `cscli <type> upgrade <name>` | Exit status. |

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
