# Charon Documentation Site

The public, browsable Charon documentation site, built with
[Docusaurus](https://docusaurus.io/). Deployed to GitHub Pages at
`https://wikid82.github.io/Charon/`.

This is a standalone Node/TypeScript project, independent of `frontend/`
(the Charon app UI) — see `docs/plans/current_spec.md` §1.4 for why this does
not violate the repo's "Single Frontend Source" rule.

`docs/` (this directory) is **git-ignored** and populated at build time by
`scripts/sync-docs.mjs`, which copies a curated subset of the repo-root
`docs/` directory in. `docs/` at the repo root remains the single source of
truth for content — never hand-edit files under `docs-site/docs/` directly,
they will be overwritten on the next `npm start`/`npm run build`.

## Installation

```bash
npm install
```

## Local Development

```bash
npm start
```

Starts a local dev server with live reload at `http://localhost:3000/Charon/`.
The `start` script runs `npm run sync-docs` first (`"start": "npm run
sync-docs && docusaurus start"` in `package.json`) so the site always
reflects the current repo-root `docs/` content before the dev server boots.

## Content Sync

```bash
npm run sync-docs
```

Runs `scripts/sync-docs.mjs`, which wipes and repopulates `docs-site/docs/`
from the curated subset of the repo-root `docs/` directory listed in
`scripts/docs-manifest.json`. It also runs automatically as the first step
of both `npm start` and `npm run build` (chained via `&&` in `package.json`),
so you rarely need to invoke it directly — it's idempotent and safe to run
any time. Never hand-edit files under `docs-site/docs/` directly; they are
deleted and regenerated on every sync.

### Adding a New Page

1. Add the Markdown file to the repo-root `docs/` directory as normal (this
   remains the single source of truth for content).
2. Add its path to `docs-site/scripts/docs-manifest.json` — either to the
   `"files"` array (a single file, path relative to `docs/`) or the
   `"directories"` array (an entire subdirectory, migrated as a unit). Only
   paths listed in this manifest are copied into the site; anything else
   under `docs/` stays internal/contributor-facing.
3. Run `npm start` (or `npm run sync-docs`) to pick up the change — the
   sidebar is auto-generated from whatever lands in `docs-site/docs/`, so no
   manual sidebar entry is needed.

## Build

```bash
npm run build
```

Generates static content into `docs-site/build/`, deployable to any static
hosting (GitHub Pages in this repo's case, via
`.github/workflows/docs-deploy.yml`).

## Type Checking

```bash
npm run type-check
```

## Dependency Audit

```bash
npm run audit:ci
```

Fails on any high/critical vulnerability per `audit-ci.json` — same pattern
as the repo root and `frontend/`.
