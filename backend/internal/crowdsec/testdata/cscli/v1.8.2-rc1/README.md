# Recorded cscli output

Recorded from the cscli bundled in the Charon image (`cscli version`):

```
version: v1.8.2-rc1
User-Agent: crowdsec/v1.8.2-rc1-linux
Constraint_api: v1
```

Full output: `version.txt`.

## Format

- `<name>.txt`: stdout and stderr of the command combined, exactly as the executors in Charon read it.
- `<name>.exit`: the exit status.
- `lapi_v1_*.body.json`: bodies of `GET /v1/decisions` on the local API (bouncer key authenticated; flat decision objects, `null` when nothing matches; no key is stored here). Without a valid key the local API answers `403` with an empty body.

Subset used by the crowdsec package; the complete recording is in `internal/api/handlers/testdata/cscli/v1.8.2-rc1`.
All addresses are documentation addresses (192.0.2.0/24, 198.51.100.0/24). `bouncers_add_raw.txt` holds a placeholder: the real command prints only the generated key, with no trailing newline, and keys are never recorded.

## Notes about specific recordings

- `decisions_list_empty_json.txt` is `[]` for this version. The code also accepts `null` and empty output (older versions); tests cover those inline.
- `decisions_list_lapi_down.txt` was cut off by a 30 s timeout: with the local API down, cscli keeps retrying for minutes.
- `alerts_list_*.txt` keep decisions that were deleted (negative duration); `decisions_list_*` do not.
- `console_status*.txt` were recorded on an installation that is not registered with the central API; the enrolled output cannot be recorded offline.
- `config_check.txt` is the usage text printed because `config check` is not a command.

## Refreshing

Start a throwaway container from the image with its own volume, create a machine and start the local API
(`cscli machines add <name> --auto`, `crowdsec -no-cs`), then rerun each command with `-c /etc/crowdsec/config.yaml`,
redirecting `2>&1` and saving the exit status. Replace any key with a placeholder before committing.

- `console_status_json_enrolled_synthetic.txt` is **synthetic**, not recorded: the enrolled output needs an installation registered with the central API, which cannot be done offline. It is `console_status_json.txt` with `registered`, `authenticated`, `enrolled` and `decision_management` set to `true` and `plan` set to `community`.
