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
- `lapi_v1_alerts*.body.json`: bodies of `GET /v1/alerts` on the local API, read with a machine login (the alert objects are the same nested shape `cscli alerts list -o json` prints). `lapi_v1_alerts_bouncer_key.txt` is the raw HTTP answer to the same request with a bouncer key: `401`, because the endpoint needs a machine login. The login and its token are not stored.

Names follow the command, for example `decisions_list_json` is `cscli -c <config> decisions list -o json`.
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

## Recordings for ban, unban and alerts

Recorded with the same method (throwaway container, own volume, a machine and the local API running). `decisions_add_dur_<form>` names the duration form with every character that is not a letter or digit written as `_` (`_1h` is `-1h`, `2_5h` is `2.5h`).

- `decisions_add_*`: `decisions add` for an IPv6 address, ranges with `-r` (also IPv6 and an unmasked `192.0.2.77/24`, which is accepted as given), an address with `-r` and a range with `-i` (both exit 1: `is not a valid range` / `is not a valid ip`), invalid and empty values, a zone (`fe80::1%eth0`, refused), values that start with a dash, reasons (empty, newline, 500 characters, starting with a dash: all accepted) and durations.
- A value that starts with a dash is taken as the flag's value (`-i -x` reads `-x`), so cscli answers `-x is not a valid ip` with exit 1. It is not read as a flag.
- Durations are checked by the local API after the request is built: `4h`, `30m`, `1d`, `90s`, `1h30m`, `2.5h`, `0`, `0s`, `-1h`, `10000d` and `100000000d` exit 0; an invalid, empty, `9999999h`, `1w`, `1h1d`, `4 h`, upper-case `4H` (`decisions_add_dur_upper_4H`, named apart because file names must differ without case) or `24h5` duration exits 1 with `decision duration '<value>': ...`. A missing `-d` is accepted.
- `decisions_delete_*`: `decisions delete` prints `N decision(s) deleted` and exits 0 for every N, including `0` for an address or range that has no decision. Exit 1 is for a value cscli cannot parse (`is not a valid ip` / `is not a valid range`) or for no value at all (usage text). An address (`-i`) also deletes the ranges that cover it (`3 decision(s) deleted` for two decisions on the address and one covering range), and a range (`-r`) deletes the stored ranges that overlap it (`decisions_delete_range_missing` removed a wider stored range). `-r` with an address and `-i` with a range exit 1.
- `alerts_list_*_json`: `alerts list -o json` for an IPv6 address (`-i`), a range (`-r`), a scenario and limit, a limit only, and an invalid limit (exit 1). The alert objects carry `machine_id`, `uuid`, `source` (`ip`, `scope`, `value`), `events_count`, `start_at`, `stop_at`, `created_at` and the nested `decisions`. An alert for a ban without a reason has the scenario `manual 'ban' from '<machine>'`.
- `console_status_json_enrolled_synthetic.txt` is **synthetic**, not recorded: the enrolled output needs an installation registered with the central API, which cannot be done offline. It is `console_status_json.txt` with `registered`, `authenticated`, `enrolled` and `decision_management` set to `true` and `plan` set to `community`.
