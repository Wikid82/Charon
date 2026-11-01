# Charon Documentation Site

The public, browsable Charon documentation site, built with
[Zensical](https://zensical.org/) (a Python-based static site generator). Deployed
to GitHub Pages at `https://wikid82.github.io/Charon/`.

This is docs tooling only. Python is used here and nowhere else in the project,
and nothing in this directory ships in the Charon container or the `frontend/`
app (see the documented exception to the "No Python" rule in `CLAUDE.md`).

`docs-site/docs/` is **git-ignored** and populated at build time by
`scripts/sync-docs.sh`, which copies a curated subset of the repo-root `docs/`
directory into it. The repo-root `docs/` remains the single source of truth for
content. Never hand-edit files under `docs-site/docs/`; they are deleted and
regenerated on every sync.

## Layout

| Path | Purpose |
| --- | --- |
| `zensical.toml` | Site configuration, theme, and the explicit navigation |
| `requirements.txt` | Pinned Zensical version (Renovate tracks it) |
| `index.md` | Landing page (copied into the generated `docs/` on every sync) |
| `assets/` | Logo, favicon, and banner (copied into the generated `docs/` on every sync) |
| `scripts/sync-docs.sh` | Builds the generated `docs/` tree from the manifest |
| `scripts/docs-manifest.json` | Allowlist of `docs/` files and directories to publish |

## Prerequisites

- Python 3.11 or newer
- `jq` (used by the sync script)

## Local Setup

Use a virtual environment outside the repository so the tooling never lands in
the working tree:

```bash
python3 -m venv ~/.venvs/charon-docs
source ~/.venvs/charon-docs/bin/activate
pip install -r docs-site/requirements.txt
```

## Local Preview

```bash
cd docs-site
bash scripts/sync-docs.sh
zensical serve
```

`zensical serve` rebuilds on change. Re-run `scripts/sync-docs.sh` after editing
anything under the repo-root `docs/` (the sync copies, it does not watch).

## Content Sync

```bash
bash docs-site/scripts/sync-docs.sh
```

The script:

1. Wipes and recreates `docs-site/docs/`, so a page removed from the manifest
   can never linger.
2. Copies each `files` entry and each `directories` entry from
   `scripts/docs-manifest.json` out of the repo-root `docs/`. A missing entry
   fails the run.
3. Copies `index.md` and `assets/` from `docs-site/`.
4. Fails if any synced page is missing from the `nav` in `zensical.toml`, so no
   page is published as an orphan.

## Build

```bash
cd docs-site
bash scripts/sync-docs.sh
zensical build --strict
```

Output goes to `docs-site/site/`. `--strict` aborts the build on any broken
internal link or anchor, matching the old Docusaurus `onBrokenLinks` and
`onBrokenAnchors: 'throw'` behavior. The CI workflow
(`.github/workflows/docs-deploy.yml`) runs the same two commands.

## Adding a Page

1. Add the Markdown file under the repo-root `docs/` as normal.
2. Add its path to `scripts/docs-manifest.json`, either in the `"files"` array
   (a single file, relative to `docs/`) or by relying on an existing
   `"directories"` entry.
3. Add the page to the `nav` in `zensical.toml`, under the right section. The
   sync script fails until you do.
4. Run the sync and strict build above to confirm.

## Deployment

`.github/workflows/docs-deploy.yml` runs on pushes to `main` that touch
`docs-site/**`, `docs/**`, or the workflow itself. It sets up Python, installs
`requirements.txt`, runs the sync, runs `zensical build --strict`, and deploys
`docs-site/site/` to GitHub Pages.

## Dependency Updates

`zensical` is pinned exactly in `requirements.txt`. Renovate opens `chore:`
PRs for it, since this is docs tooling and does not ship in the container.
