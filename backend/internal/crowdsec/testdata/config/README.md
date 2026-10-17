# Config fixtures

`dist.yaml` is the config.yaml that the Charon image ships (`/etc/crowdsec.dist/config.yaml`) after the
entrypoint adjustments (listen port, database and data paths, pinned hub branch), taken from a throwaway
container of the image. It holds no secrets: the database `user`/`password` lines are comments.

The other files are `dist.yaml` with one change each, using FAKE values only (prefix `FAKE`, documentation
addresses, `.invalid` hosts):

- `mysql.yaml`, `postgres.yaml`: external database with `user` and `password` set.
- `auto_registration_token.yaml`: `api.server.auto_registration.token` set.
- `url_userinfo.yaml`: a URL with `user:password@` userinfo.
- `anchors_alias.yaml`, `anchors_alias_value.yaml`: secret keys reached through a merge key and an alias.
- `extra_secret_keys.yaml`: the generic denylist keys (`api_key`, `apikey`, `private_key`, `secret`,
  `passwd`, `token`) next to values that must stay.

`../hub_index_ambiguous.json` is an excerpt of the raw hub index (`.index.json`, `{section: {name: entry}}`)
recorded with `cscli hub update`: `crowdsecurity/crs` exists in `appsec-configs` and `appsec-rules`,
`crowdsecurity/k8s-audit` in `collections` and `parsers`.
