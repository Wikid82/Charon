# Container image tags

The `charon` and `orthrus` images are published to Docker Hub and GHCR by the
build workflows (`.github/workflows/docker-build.yml` and
`.github/workflows/orthrus-build.yml`). This page explains what each mutable
tag points to. Release tags (`1.2.3`, `1.2`, `1`) are cut by the release
workflow and are never moved.

| Tag | Moves when | Built by |
| --- | --- | --- |
| `latest` | A push to `main` builds. This is the stable release line. | `docker-build.yml`, `orthrus-build.yml` |
| `dev` | A push to `development` builds. | `docker-build.yml`, `orthrus-build.yml` |
| `nightly` | A nightly build of the `nightly` branch completes. | `docker-build.yml` |
| `beta` | A pull request into `development` from a branch in this repository builds. Only the most recent such build is tagged `beta`. | `docker-build.yml`, `orthrus-build.yml` |
| `pr-<n>` | A pull request builds. `<n>` is the pull request number. Used by the security scan jobs in `docker-build.yml`. | `docker-build.yml` |
| `pr-<n>-<sha>` | A pull request builds. `<sha>` is the short commit SHA of that build. Each push to the pull request creates a new tag. | `orthrus-build.yml` |
| `<sha>` (short) | A push to `main`, `development` or `nightly` builds. | `docker-build.yml` |

## What `beta` means

`beta` is a convenience tag for trying the next `development` build. It is not a
release channel, and it can change at any time.

- Only pull requests opened from a branch in this repository get the tag.
  Pull requests from forks never receive it, because fork builds do not have
  registry credentials.
- Each new qualifying build moves `beta`. When several pull requests are open,
  the one built last holds the tag.
- To run a specific build, use its `pr-<n>-<sha>` tag instead.

## Pull request images

Pull request images are published to the registries so that scanners and
reviewers can pull them. They are not promoted anywhere. The `pr-<n>` and
`pr-<n>-<sha>` tags are removed by the weekly prune workflow once they are old
enough (see `docs/ci/container-prune.md`).
