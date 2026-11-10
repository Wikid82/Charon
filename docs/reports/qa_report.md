# QA Report: GeoIP Database Location

- Branch: `fix/geoip-database-location` (commits `d6a39b7b5`, `d9b4ad2e2` on `development`)
- Scope: the image keeps the GeoIP database in `/app/geoip/` instead of the data folder; the database path is chosen at startup and on every reload (`CHARON_GEOIP_DB_PATH`, then `/app/data/geoip/GeoLite2-Country.mmdb` when it is a regular file, then `/app/geoip/GeoLite2-Country.mmdb`); the build marker moves to `/app/geoip/.missing`; the image no longer sets `CHARON_GEOIP_DB_PATH`; ownership of `/app/data` is set in a separate build step.
- Image under test: the repository `Dockerfile` built with `VERSION=dev` under a throwaway tag, plus a variant whose GeoIP download step was run against an unreachable address.
- Date: 2026-11-10

## Verdict

**APPROVED.** All checks pass. Two non-blocking findings are listed below for a decision.

## Gate Results

| Gate | Result | Evidence |
| --- | --- | --- |
| `make lint-backend` | PASS | 0 issues |
| `go build ./...` | PASS | Clean |
| Scoped `go test` (geoip, services, routes, handlers, caddy) | PASS | All packages ok |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | Statement 93.3%, line 92.2% (gate 87%) |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | PASS | 100% (41/41 backend lines) |
| `lefthook run pre-commit` on the branch files | PASS | All hooks passed; semgrep 0 findings |

CodeQL and Trivy were not run locally: the change is a fix with no new feature surface, and CI runs both on every PR. No model or migration changes, so the GORM scan does not apply.

## Container Checks

Throwaway containers with their own networks, volumes and host folders. The API was bound to loopback only. Clients ran in separate containers on internal networks whose subnets resolve as GB, SE and US in the bundled database. One proxy host used a country allow-list (GB), one a country block-list (SE). All containers, networks, volumes, folders and test images were removed afterwards.

### (1) Image layout (PASS)

- `/app/geoip/GeoLite2-Country.mmdb` exists, owned by `charon:charon`, mode 644; its checksum matches `GEOLITE2_COUNTRY_SHA256`.
- No `CHARON_GEOIP_DB_PATH` in the image environment.
- The image has no `/app/data/geoip`; the entrypoint creates it.

### (2) Empty host folder mounted over `/app/data` (PASS)

Started as root (as the existing deployment does) with an empty host folder at `/app/data`:

- Startup log: database loaded from `/app/geoip/GeoLite2-Country.mmdb`.
- `/security/geoip/status`: `available: true`, `loaded: true`, `db_path: /app/geoip/GeoLite2-Country.mmdb`. The proxy `geoip2` app uses `databaseDirectory: /app/geoip`.

| Client | Allow-list host (GB) | Block-list host (SE) |
| --- | --- | --- |
| GB | 200 | 200 |
| SE | 403 | 403 |
| US | 403 | 200 |

### (3) Database placed in the data folder (PASS)

- A valid country database with no addresses was copied to `/app/data/geoip/GeoLite2-Country.mmdb`. Before the reload the answers did not change. `POST /security/geoip/reload` returned 200 with `db_path: /app/data/geoip/GeoLite2-Country.mmdb`; the status and the proxy `geoip2` app switched to `/app/data/geoip`; all clients then got 403 on the allow-list host and 200 on the block-list host, as expected for an empty database.
- After a restart with that file still present, the data-folder copy was loaded at startup.
- After removing it and reloading, the bundled copy was used again and the table in (2) was restored.
- A file that is not a database, placed in the data folder and reloaded: the reload returned an error naming the file and the bundled database stayed in use with unchanged answers.
- A directory with the database file name in the data folder is ignored; the bundled copy is used.
- `CHARON_GEOIP_DB_PATH` pointing to a missing file: status `available: false` with that path, although the bundled copy exists (the variable is used exactly).

### (4) Data folder ownership and writes (PASS)

- Default user (`charon`) with a new named volume: `/app/data` and the created `geoip`, `caddy`, `crowdsec` folders are owned by `charon`; the database and other files are written; both `/app/data` and `/app/data/geoip` are writable.
- Root with an empty host folder, and root with `PUID=1001`, `PGID=988` set: the entrypoint changed the folders to `charon`, the app and the proxy run as `charon`, and `/app/data/geoip` is writable as `charon`. `PUID`/`PGID` have no effect on this image.

### (5) Image without a database (PASS)

The GeoIP build step from the `Dockerfile` was run with the download address replaced by an unreachable one:

- The build printed both warnings and completed. `/app/geoip/` holds `.missing` and `GeoLite2-Country.mmdb.placeholder`, owned by `charon`.
- At startup the log showed that the database was not found at `/app/geoip/GeoLite2-Country.mmdb` and one Info line that the image was built without the database. Status: `available: false`, `reason: geoip_unavailable`. All clients got 403 on both country hosts.
- After copying a database to `/app/data/geoip/` and reloading (200), status became available and the table in (2) was restored without a restart.

## Findings

| ID | Severity | Finding | Evidence | Suggested follow-up |
| --- | --- | --- | --- | --- |
| F1 | Medium | Installs that use a named Docker volume for `/app/data` already hold a copy of the build-time database in `/app/data/geoip/`, because Docker filled the empty volume from the earlier image on first use. After upgrading, that copy takes precedence over the bundled one, so later images with newer data are not used and country data stays at the age of the first install. Bind-mounted folders are not affected. | A volume first used by the previous image contained `geoip/GeoLite2-Country.mmdb`; the new image then reported `db_path: /app/data/geoip/GeoLite2-Country.mmdb` | Decide before merge: prefer the newer of the two files (for example by the database build date in its metadata), or have the entrypoint remove a data-folder copy that matches a previously bundled one, or document the step in the upgrade notes |
| F2 | Low | When the data-folder copy cannot be read at startup, country lists refuse all requests even though the bundled database is fine. A reload in the same situation keeps the bundled database. Behaviour is fail-closed, logged as a warning and shown in the status. | Startup with a non-database file in `/app/data/geoip/`: `available: false`, all clients 403; a reload with the same file while running kept the bundled copy | Optional: fall back to the bundled copy at startup when the preferred file cannot be opened, or document the difference |
| F3 | Low | Containers whose configuration was copied from an earlier container (some update tools copy the full environment) keep `CHARON_GEOIP_DB_PATH=/app/data/geoip/GeoLite2-Country.mmdb` from the earlier image. With a bind-mounted data folder that path does not exist, so country lists keep refusing requests until the variable is removed or a database is placed there. Containers created fresh from the new image are not affected. | The existing dev deployment's container configuration still lists that variable | Mention in the release notes; optionally treat that exact former default as unset when the file does not exist |
