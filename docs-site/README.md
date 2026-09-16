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
