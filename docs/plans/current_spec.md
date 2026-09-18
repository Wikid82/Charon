# Technical Spec: Migrate Charon Docs to a Docusaurus Site

**Status:** Draft — pending Supervisor review and user approval
**Author:** Planning agent
**Date:** 2026-09-16
**Related:** `CLAUDE.md` (Commit Slicing & PR Strategy), `ARCHITECTURE.md` (Directory Structure, Deployment Architecture)

---

## 1. Introduction

### 1.1 Overview

Charon's user-facing documentation currently lives as plain Markdown in `docs/` at
the repo root, alongside a large body of internal, agent/contributor-facing
working documents (`docs/plans/`, `docs/reports/`, `docs/reviews/`, etc.). The
only public presentation layer today is `.github/workflows/docs.yml`, which
runs a hand-rolled `marked`-based Markdown→HTML converter
(`.github/pages/build-docs.sh`) against `docs/` and deploys the result to
GitHub Pages at `https://wikid82.github.io/Charon/`.

This plan replaces that hand-rolled pipeline with a proper static-site
generator — [Docusaurus](https://docusaurus.io/) (TypeScript template) — living
in a new top-level `docs-site/` directory, with its own Node project,
dependency-update coverage, and GitHub Actions deploy workflow. The existing
`docs/` directory is **not** touched in its layout or purpose: it remains the
single source of truth for both user-facing and internal-only documentation.
A repo-root build script copies the subset of `docs/` that is user-facing into
`docs-site/docs/` at build time — nothing is hand-duplicated or manually kept
in sync.

### 1.2 Objectives

1. Stand up `docs-site/` as an independent Docusaurus (TypeScript) project that builds a browsable, searchable documentation site from a curated subset of `docs/`.
2. Establish `docs/` → `docs-site/docs/` as a one-directional, scripted copy step (not a second hand-maintained copy) so there is exactly one authored source of truth per file.
3. Wire `docs-site/` into `scripts/charon_dep_update.sh`'s `NPM_MODULES` array so its dependencies get the same automated update/audit/build/type-check treatment as `frontend/` and the repo root.
4. Replace the existing ad hoc `docs.yml` GitHub Pages pipeline with a new workflow that builds and deploys `docs-site/` to GitHub Pages on merges to `main`, following this repo's existing CI conventions (pinned actions by SHA, pinned `NODE_VERSION`, `concurrency` groups, emoji step names, etc.).
5. Cross-link the deployed docs site from `README.md` (replacing the now-stale link) without disturbing the internal `docs/` directory's own navigation (`docs/index.md` stays as-is; it documents the internal tree, not the public site).
6. Leave `ARCHITECTURE.md` update as a flagged, explicitly-assigned follow-up step (for `docs-writer`), not written by this plan.

### 1.3 Non-Goals

- No changes to `docs/`'s existing internal-only subdirectories (`plans/`, `reports/`, `reviews/`, `analysis/`, `decisions/`, `issues/`, `superpowers/`, `runbooks/`, `patches/`, `testing/`, `development/`, `ci/`, `actions/`, `maintenance/`, `performance/`, `implementation/`) — these stay exactly where they are and are excluded from the site.
- No changes to `frontend/` (the Charon web app). `docs-site/` is a separate, independent Node/TypeScript project for documentation publishing — see §1.4 for why this does not violate the "Single Frontend Source" rule.
- No search backend (Algolia DocSearch, etc.) is configured in this pass — local Docusaurus search (`@easyops-cn/docusaurus-search-local` or the built-in offline search) is sufficient for launch and avoids an external service dependency, consistent with Charon's "no external dependencies" ethos. A follow-up can add hosted search later if desired.
- No versioned-docs (Docusaurus `docs-versioned` multi-version) setup. Charon ships one current version; a single `current` docs version is sufficient.
- No i18n/localization of the docs site in this pass (the app itself has i18next; the docs site starts English-only).

### 1.4 Why `docs-site/` Does Not Violate "Single Frontend Source"

`CLAUDE.md`'s rule reads: *"All frontend code MUST reside in `frontend/`. NEVER
create `backend/frontend/` or any other nested frontend directory."* That rule
governs Charon's **application** frontend — the React/TypeScript SPA that the
Go backend serves and that end users interact with when managing their proxy
(`internal/server`'s `attachFrontend`, built via Vite, mounted into the
binary). `docs-site/` is not that: it is a separate, static **documentation**
website, built and deployed independently via GitHub Pages, never bundled into
the Charon Docker image, never served by `internal/server`, and never linked
into the `frontend/dist` build output. It has no relationship to the
single-binary + static-assets deployment model described in
`ARCHITECTURE.md`'s Overview and Deployment Architecture sections. Treating
"frontend" in that rule as "any directory containing a package.json and
TypeScript" would also outlaw tooling directories the repo already accepts
implicitly (e.g. root `package.json` for Playwright/lint tooling). The rule's
intent — preventing a second, competing copy of the *app* UI — is preserved:
there is exactly one `frontend/` and it is unaffected by this change.

`ARCHITECTURE.md`'s Directory Structure section should be updated to list
`docs-site/` alongside `backend/` and `frontend/` once this lands (see §6,
Commit 5 — flagged for `docs-writer`, not written here).

---

## 2. Research Findings

### 2.1 Existing `docs/` Structure (as of this plan)

Top-level files in `docs/` (39 files) and directories, classified below.
Directories confirmed via `find docs -maxdepth 2`.

**User-facing / operator-facing (candidates for migration):**

| Path | Notes |
|---|---|
| `docs/getting-started.md` | Explicitly named in scope by user |
| `docs/features.md` | Explicitly named in scope |
| `docs/features/*.md` (30 files: `access-control.md`, `api.md`, `audit-logging.md`, `backup-remote-oauth-setup.md`, `backup-restore.md`, `caddyfile-import.md`, `crowdsec.md`, `custom-plugins.md`, `disaster-recovery.md`, `dns-autodetection.md`, `dns-auto-detection.md`, `dns-challenge.md`, `dns-providers.md`, `docker-integration.md`, `hecate.md`, `key-rotation.md`, `live-reload.md`, `localization.md`, `logs.md`, `multi-credential.md`, `notifications.md`, `orthrus.md`, `plugin-security.md`, `proxy-headers.md`, `rate-limiting.md`, `security-headers.md`, `security.md`, `ssl-certificates.md`, `supply-chain-security.md`, `ui-themes.md`, `uptime-monitoring.md`, `user-accounts.md`, `waf.md`, `websocket.md`, `web-ui.md`) | Per-feature user docs; `docs/index.md` already links two of these (`features/orthrus.md`, `features/hecate.md`) as public pages |
| `docs/configuration/emergency-setup.md` | Explicitly named in scope (dir) |
| `docs/guides/*.md` + `docs/guides/dns-providers/*.md` (`crowdsec-setup.md`, `dns-providers.md`, `local-key-management.md`, `manual-dns-provider.md`, `remote-docker-setup.md`, `supply-chain-security-developer-guide.md`, `supply-chain-security-user-guide.md`, plus `dns-providers/{azure-dns,cloudflare,digitalocean,google-cloud-dns,route53}.md`) | Explicitly named in scope (dir) |
| `docs/security.md` | Explicitly named in scope |
| `docs/troubleshooting/*.md` (`crowdsec.md`, `dns-challenges.md`, `e2e-tests.md`, `go-gopls.md`, `proxy-headers.md`, `react-production-errors.md`, `websocket.md`) | Explicitly named in scope (dir) — see note below on `e2e-tests.md`/`go-gopls.md` |
| `docs/api.md` | Explicitly named in scope |
| `docs/api/DNS_DETECTION_API.md` | Sibling of `api.md`; developer/integration-facing, same audience |
| `docs/migration-guide.md` | Explicitly named in scope |
| `docs/database-schema.md` | Explicitly named in scope |
| `docs/import-guide.md` | Explicitly named in scope |
| `docs/live-logs-guide.md` | Explicitly named in scope |
| `docs/acme-staging.md` | Operator-facing ("Testing SSL Certificates", linked from `docs/index.md`) |
| `docs/cerberus.md` | Operator-facing security suite overview, linked from `docs/index.md`'s spirit (security section) |
| `docs/database-maintenance.md` | Operator-facing maintenance task |
| `docs/crowdsec-auto-start-quickref.md` | Operator-facing quick reference |
| `docs/migration-guide-crowdsec-auto-start.md` | Operator-facing migration doc, same family as `migration-guide.md` |
| `docs/security-incident-response.md` | Operator-facing ("what do I do if I'm breached") — distinct from the internal `docs/runbooks/` |

**Judgment calls (bias toward keeping internal, listed for visibility):**

| Path | Decision | Reasoning |
|---|---|---|
| `docs/troubleshooting/e2e-tests.md`, `docs/troubleshooting/go-gopls.md` | **Migrate** (dir was explicitly named in scope wholesale; splitting the dir adds sync complexity for two files) | Slightly contributor-leaning content, but low harm being public, and user asked to migrate `troubleshooting/` as a unit |
| `docs/debugging-local-container.md` | **Stay in `docs/`** | Contributor/dev-environment debugging, not an operator running the shipped binary |
| `docs/github-setup.md` | **Stay in `docs/`** | Repo/CI setup for contributors, not product docs |
| `docs/i18n-examples.md` | **Stay in `docs/`** | Translator/contributor pattern examples, not end-user material |
| `docs/SECURITY_PRACTICES.md` | **Stay in `docs/`** | Internal engineering security practices (parallel to root `SECURITY.md`), distinct from the public `docs/security.md` |
| `docs/stats_feature_warmup.md` | **Stay in `docs/`** | Reads as an internal implementation note, not user guidance |
| `docs/security/*.md` (`ghsa-*-options.md`, `vulnerability-analysis-*.md`) | **Stay in `docs/`** | Internal vulnerability-analysis working notes, not the public `security.md` |
| `docs/maintenance/`, `docs/performance/`, `docs/implementation/` | **Stay in `docs/`** | Not in the user's explicit migrate list; read as internal engineering/ops dirs (diagnostics, implementation notes). Flag for a human/`docs-writer` follow-up pass in case any single file inside warrants promotion later. |

**Confirmed internal-only, explicitly named by the user (unchanged, not re-litigated):**
`docs/plans/`, `docs/reports/`, `docs/reviews/`, `docs/analysis/`, `docs/decisions/`, `docs/issues/`, `docs/superpowers/`, `docs/runbooks/`, `docs/patches/`, `docs/testing/`, `docs/development/`, `docs/ci/`, `docs/actions/`.

`docs/index.md` itself **stays in `docs/`** unmigrated — it is the nav page for
the *internal* tree (mixes links to internal and public docs). The Docusaurus
site gets its own generated landing page (`docs-site/src/pages/index.tsx`,
scaffolded by the template) plus an `intro`/overview doc, not a copy of
`docs/index.md`.

### 2.2 Sync Mechanism Decision: Scripted Copy, `docs/` Remains Source of Truth

**Decision: `docs/` remains the single source of truth. `docs-site/docs/` is a
build-time, scripted, git-ignored copy — never hand-edited, never committed.**

Rationale:
- A manually-duplicated second copy (both directories committed and hand-kept-in-sync) has the classic two-masters problem: nothing enforces that an editor of one updates the other, and CI has no mechanism to catch drift short of a diff-check gate on every PR. That is extra process for zero benefit here.
- A copy-and-commit-generated-copy approach (script runs, output committed) still risks a contributor editing `docs-site/docs/*.md` directly and the change silently not round-tripping back to `docs/`.
- A build-time copy that is **not committed** (git-ignored, regenerated by both the local dev script and CI before every build) has exactly one edit location (`docs/`), and Docusaurus's local dev server (`npm start`) picking up live edits requires only that the copy script also be runnable in watch/pre-dev context (see §3.3).
- Docusaurus's build pipeline is filesystem-driven (it reads whatever is in the configured `docs` dir at `docusaurus.config.ts`'s `presets[0].docs.path`, default `docs`), so pointing it at a git-ignored, freshly-populated `docs-site/docs/` directory is a supported, idiomatic pattern (equivalent to a generated `dist/` or `build/` directory) — no Docusaurus plugin fork or custom loader is required.

Mechanism:
- New script `docs-site/scripts/sync-docs.mjs` (Node, no new runtime dependency — uses `node:fs`, `node:path`) that:
  1. Reads an explicit allowlist of source paths from `docs-site/scripts/docs-manifest.json` (the file/dir list in §2.1's "user-facing" table — an explicit allowlist, not a glob-exclude of the internal dirs, so that a newly-added internal dir under `docs/` is safe-by-default and never leaks into the public site without a deliberate manifest edit).
  2. Recursively copies each allowlisted path from `docs/<path>` into `docs-site/docs/<path>` (mirroring the relative structure, e.g. `docs/features/orthrus.md` → `docs-site/docs/features/orthrus.md`), stripping nothing — Docusaurus consumes standard Markdown + optional frontmatter directly, and the existing YAML frontmatter style already seen in `docs/index.md` (`title`, `description`) is exactly what Docusaurus's `docs` plugin expects for page metadata, so files migrate with zero content rewriting in the common case.
  3. Deletes and recreates `docs-site/docs/` on every run (idempotent, no stale-file accumulation when a file is removed from the manifest).
  4. Exits non-zero with a clear message if a manifest path does not exist under `docs/` (catches typos/renames early rather than silently producing a thinner site).
- `docs-site/package.json` scripts:
  - `"presync": "node scripts/sync-docs.mjs"` is **not** used (npm's implicit pre-hooks only fire for a matching script name, not arbitrary scripts); instead `"sync-docs": "node scripts/sync-docs.mjs"` is explicit, and both `"start"` and `"build"` are wrapped: `"start": "npm run sync-docs && docusaurus start"`, `"build": "npm run sync-docs && docusaurus build"`. This guarantees the copy is always fresh before either a local preview or a CI build, with no separate manual step to forget.
- `docs-site/docs/` (the generated copy) is added to `.gitignore` (see §5) — it must never be committed, exactly like `frontend/dist/`.

### 2.3 Existing GitHub Pages Pipeline (Must Be Retired)

`.github/workflows/docs.yml` currently:
- Triggers on `workflow_run` completion of "Docker Build, Publish & Test" (on `main`) plus manual `workflow_dispatch`.
- Uses `NODE_VERSION: '24.21.0'`, pinned `actions/checkout@3d3c42e5...` (v7), pinned `actions/setup-node@820762786026...` (v7).
- Runs `npm install -g marked` then `bash .github/pages/build-docs.sh`, which hand-renders `README.md` + `docs/**/*.md` (frontmatter-aware) into `_site/`, wraps pages with nav/SEO/OG tags, rewrites paths, emits `sitemap.xml`/`robots.txt`, and copies a hand-authored `.github/pages/docs-index.html` landing page.
- Uploads via `actions/upload-pages-artifact@fc324d35...` (v5) and deploys via `actions/deploy-pages@368f8252...` (v5.0.1) to the `github-pages` environment, permissions `contents: read`, `pages: write`, `id-token: write`.
- Deployed site is linked from `README.md:136`: `https://wikid82.github.io/Charon/docs/getting-started.html`.

**This pipeline must be retired, not left running in parallel.** GitHub Pages
serves one live site per repo from the `github-pages` deployment environment;
running both `docs.yml` and a new Docusaurus deploy workflow would race to
deploy on every push to `main`, non-deterministically clobbering each other
(whichever job's `deploy-pages` step lands last wins), and would leave two
divergent copies of "the docs" (marked's flat HTML render vs. Docusaurus's
site) both partially live depending on timing. There is no reasonable
"transition period" here — GitHub Pages doesn't support two concurrent sites
from one repo without path-based sharding, which is not worth the complexity
for a docs site with one obvious owner going forward. `docs.yml`,
`.github/pages/build-docs.sh`, and `.github/pages/docs-index.html` are deleted
in the same PR that introduces the new deploy workflow (§4.4, Commit 4) so
there is never a window with two live deploy paths.

### 2.4 Repo Conventions Confirmed

- **Node version pin**: `NODE_VERSION: '24.21.0'` is the repo-wide convention (`docs.yml`, `docs-to-issues.yml`, `quality-checks.yml`). `docs-site/` follows the same pin, both in its new workflow's `env:` block and (see §3.2) implicitly via `engines` in `package.json` for local-dev clarity.
- **Action pinning**: All third-party actions pinned by full commit SHA with a trailing `# vX` comment (`actions/checkout@3d3c42e...  # v7`, `actions/setup-node@820762786026...  # v7`, `actions/upload-pages-artifact@fc324d35...  # v5`, `actions/deploy-pages@368f82528645...  # v5.0.1`). The new workflow reuses these exact pins (same major versions already vetted in this repo) rather than introducing new ones.
- **`audit:ci` pattern**: Both `package.json` (root) and `frontend/package.json` define `"audit:ci": "audit-ci --config ./audit-ci.json"`, each with a sibling `audit-ci.json` (`{"$schema": ..., "high": true, "allowlist": []}`). `charon_dep_update.sh`'s `update_npm()` calls `npm run audit:ci` unconditionally for every `NPM_MODULES` entry (`rm -rf node_modules package-lock.json && npm install && npm dedupe && npm run --if-present build && npm run --if-present type-check && npm run audit:ci`) — note `audit:ci` is **not** `--if-present` gated, so `docs-site/package.json` **must** define it or the update script hard-fails on that module. `docs-site/` gets its own `audit-ci.json` (root's is empty-allowlist `high: true`; docs-site starts the same — no known findings to allowlist yet).
- **`build`/`type-check` are `--if-present`-gated** in the update script, so they're optional for correctness, but the plan defines both anyway (`docusaurus build`, `tsc --noEmit`) since Docusaurus's TS template ships a `tsconfig.json` and both scripts are cheap, high-value CI/update-time checks.
- No `.nvmrc` exists at repo root or in `frontend/`; Node version is carried entirely via each workflow's `env.NODE_VERSION` and (for local dev) documented in each project's README/CLAUDE.md — `docs-site/` follows this pattern rather than introducing a new `.nvmrc` convention.
- `.gitignore` already has a "Docs & Plans" section (lines 7–11) for specific internal working files, and a general `node_modules/` + per-package `frontend/node_modules/`, `backend/node_modules/`, `frontend/dist/` pattern (lines 34–37, 81) — `docs-site/` additions follow the same per-package explicit-path style rather than relying on the blanket `node_modules/` alone (defense in depth, matches existing style).

---

## 3. Technical Specifications

### 3.1 Directory Structure for `docs-site/`

Docusaurus's official TypeScript classic template (`npx create-docusaurus@latest docs-site classic --typescript`), scaffolded then adapted:

```
docs-site/
├── package.json                 # New — see §3.2
├── package-lock.json            # Generated by npm install (committed, like frontend/)
├── tsconfig.json                # From template, extends @docusaurus/tsconfig
├── audit-ci.json                # New — mirrors root/frontend pattern
├── docusaurus.config.ts         # Site config — see below
├── sidebars.ts                  # Sidebar structure — see below
├── .gitignore                   # Docusaurus template default (build/, .docusaurus/, node_modules/) — superseded/reinforced by repo-root .gitignore too
├── docs/                        # GIT-IGNORED generated copy — populated by scripts/sync-docs.mjs from ../docs/. Never hand-edited.
├── scripts/
│   ├── sync-docs.mjs            # New — copy script, see §2.2
│   └── docs-manifest.json       # New — explicit allowlist of docs/ paths to migrate
├── src/
│   ├── css/
│   │   └── custom.css           # Template default; Charon brand colors applied here
│   └── pages/
│       └── index.tsx            # Landing page (template default, customized with Charon branding/links)
├── static/
│   └── img/
│       ├── favicon.ico          # Reuse existing Charon favicon asset (copy from frontend/public/ or root)
│       └── banner.webp          # Reuse existing OG image already referenced in build-docs.sh (frontend/public/banner.webp)
└── README.md                    # New — how to run/build docs-site locally, references sync-docs.mjs
```

Key `docusaurus.config.ts` settings (values, not full file — implementer fills in per Docusaurus TS template conventions):

| Setting | Value | Reasoning |
|---|---|---|
| `title` | `"Charon"` | Matches product name |
| `tagline` | `"Your server, your rules — without the headaches."` | Pulled verbatim from `ARCHITECTURE.md`'s Core Value Proposition |
| `url` | `"https://wikid82.github.io"` | GitHub Pages org/user domain |
| `baseUrl` | `"/Charon/"` | Project page path, matches current live URL structure (`https://wikid82.github.io/Charon/...`) so the README link in §3.5 stays a natural extension of the existing pattern |
| `organizationName` | `"Wikid82"` | GitHub org/user |
| `projectName` | `"Charon"` | Repo name |
| `deploymentBranch` | N/A — deploy handled by Actions workflow, not `docusaurus deploy` (see §3.4) | Avoids needing a `gh-pages` branch/token; matches existing Pages Actions-based deploy model already used by `docs.yml` |
| `presets[0].docs.path` | `"docs"` | Points at the git-ignored, sync-script-populated `docs-site/docs/` |
| `presets[0].docs.sidebarPath` | `"./sidebars.ts"` | Default |
| `presets[0].docs.routeBasePath` | `"docs"` (default, or `"/"` if the landing page should *be* the docs home — recommend keeping default `"docs"` since `src/pages/index.tsx` provides a proper marketing-style landing page, matching the current site's separate landing-page-vs-docs split in `docs-index.html`) | Matches existing UX shape |
| `presets[0].blog` | `false` (disabled) | Charon docs are reference material, not a blog; avoids an empty, confusing "Blog" nav item |
| `themeConfig.navbar.items` | Links to Getting Started, Features, Guides, API, GitHub repo | Mirrors `docs/index.md`'s "Start Here" grouping |
| Search | `@easyops-cn/docusaurus-search-local` plugin (offline, no external service) | Consistent with "no external dependencies" ethos (§1.3) |

`sidebars.ts` uses Docusaurus's `autogenerated` sidebar type pointed at the
synced `docs/` tree (`{type: 'autogenerated', dirName: '.'}`) rather than a
hand-maintained manual sidebar array — this means the sidebar tracks whatever
is in `docs-manifest.json` automatically, with per-directory ordering
controlled by lightweight `_category_.json` files added under
`docs-site/docs-category-overrides/` that the sync script merges in (or,
simpler for v1: rely on Docusaurus's default alphabetical + `sidebar_position`
frontmatter, which the copied files can gain incrementally). **Recommendation
for v1: autogenerated + alphabetical, no category overrides** — lowest
implementation cost, revisit if navigation ordering proves confusing after
launch.

### 3.2 `docs-site/package.json`

```json
{
  "name": "charon-docs-site",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "sync-docs": "node scripts/sync-docs.mjs",
    "start": "npm run sync-docs && docusaurus start",
    "build": "npm run sync-docs && docusaurus build",
    "swizzle": "docusaurus swizzle",
    "deploy": "docusaurus deploy",
    "clear": "docusaurus clear",
    "serve": "docusaurus serve",
    "write-translations": "docusaurus write-translations",
    "write-heading-ids": "docusaurus write-heading-ids",
    "type-check": "tsc --noEmit",
    "audit:ci": "audit-ci --config ./audit-ci.json"
  },
  "dependencies": {
    "@docusaurus/core": "^3.9.0",
    "@docusaurus/preset-classic": "^3.9.0",
    "@easyops-cn/docusaurus-search-local": "^0.44.5",
    "@mdx-js/react": "^3.1.1",
    "clsx": "^2.1.1",
    "prism-react-renderer": "^2.4.1",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@docusaurus/module-type-aliases": "^3.9.0",
    "@docusaurus/tsconfig": "^3.9.0",
    "@docusaurus/types": "^3.9.0",
    "audit-ci": "^7.1.0",
    "typescript": "^5.7.3"
  },
  "engines": {
    "node": ">=20.0"
  },
  "browserslist": {
    "production": [">0.5%", "not dead", "not op_mini all"],
    "development": ["last 3 chrome version", "last 3 firefox version", "last 5 safari version"]
  }
}
```

Notes:
- `typescript` is pinned to `^5.7.3`, **not** `^6.0.3` like root/frontend — Docusaurus 3.9's toolchain does not yet support TypeScript 6 (root's `charon_dep_update.sh` already carries an explicit `--reject typescript` exclusion for this exact class of problem on `frontend/`; the same constraint applies here and the exclusion list in §4.3 must be extended, not duplicated with a new mechanism).
- Exact `@docusaurus/*` versions above are current-as-of-this-plan; the implementer should run `create-docusaurus@latest` at implementation time and use whatever it scaffolds, adjusting this table to match — pinning exact versions in a spec written weeks before implementation is guaranteed to drift.
- React 18 (not 19, matching `frontend/`'s 19.2.3) is what Docusaurus 3.x's classic preset currently supports; this is an intentional, isolated exception — `docs-site/`'s React tree is fully independent of `frontend/`'s (separate `node_modules`, separate bundle, never co-loaded in a browser tab with the app), so there is no version-skew risk to the actual product.

### 3.3 `docs-site/scripts/docs-manifest.json`

```json
{
  "files": [
    "getting-started.md",
    "features.md",
    "security.md",
    "api.md",
    "migration-guide.md",
    "database-schema.md",
    "import-guide.md",
    "live-logs-guide.md",
    "acme-staging.md",
    "cerberus.md",
    "database-maintenance.md",
    "crowdsec-auto-start-quickref.md",
    "migration-guide-crowdsec-auto-start.md",
    "security-incident-response.md"
  ],
  "directories": [
    "features",
    "configuration",
    "guides",
    "troubleshooting",
    "api"
  ]
}
```

`sync-docs.mjs` reads `files` (copied to `docs-site/docs/<name>`) and
`directories` (recursively copied to `docs-site/docs/<name>/`) — see §2.2 for
the copy algorithm. Adding a new user-facing doc later is a one-line manifest
edit, not a script change.

### 3.4 New GitHub Actions Workflow: `.github/workflows/docs-deploy.yml`

Replaces `docs.yml` (deleted in the same commit, §4.4). Structure follows
`docs.yml`'s existing two-job (`build` / `deploy`) shape and permissions, swapping the build step for a Docusaurus build:

```yaml
name: Deploy Documentation Site

on:
  push:
    branches: [main]
    paths:
      - 'docs-site/**'
      - 'docs/**'
      - '.github/workflows/docs-deploy.yml'
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: "pages-${{ github.ref }}"
  cancel-in-progress: false

env:
  NODE_VERSION: '24.21.0'

jobs:
  build:
    name: Build Documentation Site
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - name: 📥 Checkout code
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7

      - name: 🔧 Set up Node.js
        uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'npm'
          cache-dependency-path: docs-site/package-lock.json

      - name: 📦 Install dependencies
        working-directory: docs-site
        run: npm ci

      - name: 📝 Build documentation site
        working-directory: docs-site
        run: npm run build

      - name: 📤 Upload artifact
        uses: actions/upload-pages-artifact@fc324d3547104276b827a68afc52ff2a11cc49c9 # v5
        with:
          path: 'docs-site/build'

  deploy:
    name: Deploy to GitHub Pages
    if: github.ref == 'refs/heads/main'
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    timeout-minutes: 5
    needs: build
    steps:
      - name: 🚀 Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346 # v5.0.1
```

Differences from `docs.yml` worth calling out to a reviewer:
- Trigger changes from `workflow_run` (chained after Docker Build) to a direct `push` on `main` filtered to relevant paths, plus manual dispatch. The old chained trigger existed because the marked-based pipeline copied `README.md` verbatim including build-status badges that implicitly depend on a successful Docker build; Docusaurus's build has no such coupling, and gating docs publishing on an unrelated Docker Hub/GHCR pipeline succeeding is not a real dependency — a path-filtered direct push trigger is simpler and more responsive.
- `npm ci` (not `npm install`) in CI for reproducible, lockfile-exact installs — matches standard CI practice and is implied-but-unstated in `docs.yml` (which had no lockfile to `ci` against since it only did a global `npm install -g marked`).
- Node's built-in npm cache (`cache: 'npm'` + `cache-dependency-path`) added since a real `package-lock.json` now exists to key off of — this affordance didn't apply to the old pipeline's single global install.

### 3.5 README.md / `docs/index.md` Cross-Linking

- `README.md:136` currently reads: `Full setup instructions and documentation are available at [https://wikid82.github.io/Charon/docs/getting-started.html](https://wikid82.github.io/Charon/docs/getting-started.html).` — updated to the Docusaurus route for the same page: `https://wikid82.github.io/Charon/docs/getting-started` (Docusaurus strips `.html` and the `docs/` `routeBasePath` prefix already matches, so only the trailing `.html` is dropped — implementer must verify the exact generated route once `sidebars.ts` autogeneration is scaffolded, since the manifest copies `getting-started.md` to the doc ID `getting-started`, and Docusaurus's default routing is `<routeBasePath>/<docId>` i.e. `docs/getting-started`).
- `README.md:198` (`[Explore All Features →](https://github.com/Wikid82/Charon/blob/main/docs/features.md)`) is left as a direct GitHub blob link, unchanged — it already works today and isn't part of the Pages pipeline; optionally could be repointed at the new site, but that's a judgment call left to `docs-writer` in the hardening commit (§4.5), not a hard requirement.
- `docs/index.md` is **not** modified — it remains the nav page for the internal `docs/` tree as browsed directly on GitHub (its links are relative Markdown links that work fine in GitHub's own Markdown renderer regardless of the Pages site's existence).
- The new Docusaurus landing page (`docs-site/src/pages/index.tsx`) gets a "Start Here" section mirroring `docs/index.md`'s grouping (Getting Started / Features / Import / Security / API / Remote Access), written against the migrated doc IDs.

### 3.6 API Design / Database Schema / Component Design

Not applicable — this is a static documentation site with no backend API
surface, no database interaction, and no `internal/models` or
`internal/api/routes` changes. No `AutoMigrate` changes. No new Go code.

### 3.7 Error Handling

| Failure mode | Handling |
|---|---|
| `sync-docs.mjs` manifest references a path that no longer exists under `docs/` (renamed/deleted upstream) | Script exits non-zero with the missing path named; fails `npm run build` / `npm start` loudly rather than silently publishing a thinner site |
| Docusaurus build fails (broken internal link, invalid frontmatter, MDX parse error) | `docusaurus build` exits non-zero (Docusaurus's default `onBrokenLinks: 'throw'` config, kept at its default rather than downgraded to `'warn'`) — CI job fails, nothing deploys, previous Pages deployment remains live (GitHub Pages does not roll back on a failed new deployment; the last successful `deploy-pages` run stays serving) |
| `npm ci` / `audit:ci` finds a high/critical vuln in `docs-site/` deps during `charon_dep_update.sh` | Script hard-fails per existing `set -euo pipefail` behavior (same as any other `NPM_MODULES` entry today) — surfaces in the dependency-update PR/run, not silently swallowed |
| Two docs-deploy workflow runs race (e.g. two quick merges to `main`) | `concurrency: {group: "pages-${{ github.ref }}", cancel-in-progress: false}` queues rather than cancels, matching `docs.yml`'s existing queuing semantics — last-queued run's content wins, no half-deployed states since each run's `build` job produces a complete artifact |

---

## 4. Implementation Plan

### Phase 1: Playwright Tests (Spec Behavior)

Not applicable in the traditional sense — `docs-site/` is a static site with
no Playwright-testable app flows in Charon's existing E2E suite (which targets
`frontend/`'s running app against a live backend). No `test.fixme` E2E specs
are added for this feature. Verification instead happens via:
- `npm run build` succeeding locally and in CI (broken-link detection is Docusaurus's own build-time check, replacing what a Playwright smoke test would otherwise catch).
- A manual/CI smoke check (`npm run serve` + `curl` the built `docs-site/build/index.html` and a couple of migrated pages) as part of Commit 4's validation gate, in place of a Playwright spec.

### Phase 2: Backend Implementation

Not applicable — no `backend/` changes.

### Phase 3 & Foundation: Docs Site Scaffold, Content Sync, Dep-Script Wiring

Covered in Commits 1–3 below (this feature has no meaningful frontend/backend
split; "frontend" here just means "the docs-site TypeScript project," which
*is* the deliverable, not a UI layer on top of a Go API).

### Phase 4: Integration and Testing

Covered in Commit 4 (CI workflow) and Commit 5 (cross-links, cleanup) below.

### Phase 5: Documentation and Deployment

- `docs-writer` updates `ARCHITECTURE.md`'s Directory Structure section to list `docs-site/` (flagged here, not written by this plan — see §1.4).
- `docs-writer` reviews the migrated content for any Docusaurus-specific formatting improvements (admonitions, tabs) as an optional follow-up, out of scope for this PR's merge bar.

---

## 5. `.gitignore` Additions

Appended near the existing `node_modules/`/`frontend/dist/` block (after line 37, following the repo's per-package explicit-path style):

```gitignore
# Docs site (Docusaurus) - generated content and build output
docs-site/node_modules/
docs-site/docs/
docs-site/build/
docs-site/.docusaurus/
docs-site/.cache-loader/
```

`docs-site/docs/` (the sync-script-generated copy, §2.2) is git-ignored
alongside the standard Docusaurus `build/`/`.docusaurus/` artifacts — this is
the enforcement mechanism that makes "`docs/` is the only source of truth"
actually true rather than aspirational (a committed `docs-site/docs/` would
immediately invite drift).

---

## 6. Commit Slicing Strategy

**Decision: single PR, five ordered commits, one feature (docs-site
migration) merged only when complete.** No PR is opened until Commit 5 is
ready; commits are pushed sequentially to the feature branch and reviewed as a
whole, per `CLAUDE.md`'s "Slice Commits, Not PRs."

### Commit 1 — Scaffold Docusaurus project (foundation, no `docs/` content yet)

- **Commit message prefix**: `chore:` — `docs-site/` is never bundled into the Docker image or served by `internal/server` (see §1.4), so per CLAUDE.md's CI/CD conventions this must not use `feat:`/`fix:`/`perf:`, which would trigger an unwanted Docker build/release for a docs-only change. All five commits in this slice use `chore:`.
- **Scope**: Create `docs-site/` via the TypeScript classic template, strip template placeholder content (default `docs/intro.md`, `blog/`, tutorial docs), configure `docusaurus.config.ts` / `sidebars.ts` per §3.1, add `docs-site/package.json` per §3.2, add `docs-site/audit-ci.json` (`{"$schema": "https://raw.githubusercontent.com/IBM/audit-ci/main/docs/schema.json", "high": true, "allowlist": []}`), add `docs-site/tsconfig.json`, add a placeholder `docs-site/docs/intro.md` *only* for this commit's local verification (removed once Commit 2 wires real sync) or simply verify against the template's stock content before Commit 2 replaces it.
- **Files**: `docs-site/package.json`, `docs-site/package-lock.json`, `docs-site/tsconfig.json`, `docs-site/audit-ci.json`, `docs-site/docusaurus.config.ts`, `docs-site/sidebars.ts`, `docs-site/src/**`, `docs-site/static/**`, `docs-site/README.md`, `docs-site/.gitignore` (template default).
- **Dependencies**: None.
- **Validation gate**: `cd docs-site && npm install && npm run type-check && npm run build` all succeed; `npm run audit:ci` reports zero high/critical findings.

### Commit 2 — Content sync mechanism + migrated docs wired in

- **Commit message prefix**: `chore:`.
- **Scope**: Add `docs-site/scripts/sync-docs.mjs` and `docs-site/scripts/docs-manifest.json` per §2.2/§3.3; update `package.json`'s `start`/`build` scripts to run `sync-docs` first; remove the Commit-1 placeholder doc; add the `@easyops-cn/docusaurus-search-local` plugin config for local search.
- **Files**: `docs-site/scripts/sync-docs.mjs`, `docs-site/scripts/docs-manifest.json`, `docs-site/package.json` (script updates), `docs-site/docusaurus.config.ts` (search plugin), `.gitignore` (add `docs-site/docs/` and friends per §5).
- **Dependencies**: Commit 1.
- **Validation gate**: `cd docs-site && npm run build` succeeds and produces a complete `docs-site/build/` with pages for every manifest entry (spot-check: `docs-site/build/docs/getting-started/index.html`, `docs-site/build/docs/features/orthrus/index.html` exist and contain expected content); `git status` shows `docs-site/docs/` is untracked/ignored, not staged.

### Commit 3 — Wire `scripts/charon_dep_update.sh`

- **Commit message prefix**: `chore:`.
- **Scope**: Add `"$REPO_ROOT/docs-site"` to the `NPM_MODULES` array only. The script's `npx npm-check-updates -u --reject typescript,@types/eslint-plugin-jsx-a11y` call runs unconditionally for every entry in the `NPM_MODULES` loop (it is not per-module and there is no separate per-module reject list) — adding `docs-site` to the array automatically gets the same TypeScript-6 rejection for free. No second exclusion-list change is needed or exists to make.
- **Files**: `scripts/charon_dep_update.sh` (single array-entry addition).
- **Dependencies**: Commits 1–2 (the module must build/type-check/audit cleanly before the update script exercises it).
- **Validation gate**: Do not run the full `bash scripts/charon_dep_update.sh npm` for this commit's gate — it bumps dependencies repo-wide (root and `frontend/` too) as a side effect, which is disproportionate for verifying a single array-line change. Instead, directly exercise the `docs-site` block's commands the way the script's loop body would: from `docs-site/`, run `rm -rf node_modules package-lock.json && npm install && npm dedupe && npm run build && npm run type-check && npm run audit:ci` and confirm all succeed, then separately confirm TypeScript in `docs-site/package.json` stays on the `^5.x` line after an `npx --yes npm-check-updates -u --reject typescript,@types/eslint-plugin-jsx-a11y` dry pass. Save the full multi-module script run for CI/pre-merge, not this commit's local gate.

### Commit 4 — CI: Pages deploy workflow, retire the old pipeline

- **Commit message prefix**: `chore:`.
- **Scope**: Add `.github/workflows/docs-deploy.yml` per §3.4; delete `.github/workflows/docs.yml`, `.github/pages/build-docs.sh`, `.github/pages/docs-index.html` (and any other files solely used by the retired pipeline — verify no other workflow references `build-docs.sh` or `docs-index.html` before deleting).
- **Files**: `.github/workflows/docs-deploy.yml` (new), `.github/workflows/docs.yml` (deleted), `.github/pages/build-docs.sh` (deleted), `.github/pages/docs-index.html` (deleted).
- **Dependencies**: Commits 1–3 (workflow assumes `docs-site/package-lock.json` exists and `npm run build` is green).
- **Validation gate**: `actionlint .github/workflows/docs-deploy.yml` (or whatever this repo's lint-workflow tooling is — check `make` targets / `lefthook` config for an existing actionlint invocation and reuse it) passes with no errors; a `workflow_dispatch` manual run against the feature branch (or a fork/test run per `devops` agent's standard verification approach) completes the `build` job successfully and produces a valid Pages artifact. Full end-to-end Pages deploy verification (the `deploy` job) is CI-on-`main`-only per this repo's standard model — cannot be fully validated pre-merge, same constraint the old `docs.yml` had.

### Commit 5 — Cross-links, `.gitignore` finalization, cleanup

- **Commit message prefix**: `chore:`.
- **Scope**: Update `README.md:136`'s link per §3.5; confirm/finalize the `.gitignore` block from §5 landed correctly (may already be done in Commit 2 — this commit is the final audit pass, ensure no `docs-site` build artifacts got accidentally staged anywhere in Commits 1–4); add `docs-site/README.md` local-dev instructions (how to run `npm start`, how the sync script works, where to add new pages via the manifest) if not already covered in Commit 1's scaffold; flag `ARCHITECTURE.md`'s Directory Structure section for a follow-up `docs-writer` update (do not write the `ARCHITECTURE.md` prose in this commit — per the task's explicit instruction, that's assigned to `docs-writer` in a later pipeline step, not this plan).
- **Files**: `README.md`, `docs-site/README.md`, `.gitignore` (final check), no `ARCHITECTURE.md` edit in this commit.
- **Dependencies**: Commits 1–4.
- **Validation gate**: `npx markdownlint-cli2 'README.md' 'docs-site/README.md'` (repo's existing `lint:md` script, scoped to touched files) passes; manual click-through of the README's updated docs link against the locally-built `docs-site/build/` (via `npm run serve`) confirms the target page exists at the expected path.

### Rollback / Contingency Notes (whole-PR level)

- Because Commit 4 deletes the old `docs.yml` pipeline, a rollback of this PR **after merge** needs to `git revert` the full commit range (not just Commit 4) to restore the working old pipeline — reverting only Commit 4 would leave `docs-site/` orphaned with no deploy path, which is a worse state than either fully-forward or fully-reverted. Document this in the PR description explicitly.
- If Commit 4's CI validation reveals GitHub Pages `baseUrl`/routing mismatches post-merge (e.g. asset 404s under `/Charon/` subpath), the fix is a `docusaurus.config.ts` `baseUrl`/`url` correction plus a re-run of the `docs-deploy` workflow — no code rollback needed, this is a config-only fix forward.
- If `docs-site/`'s dependency footprint later proves too heavy for `charon_dep_update.sh`'s runtime (Commit 3's concern), the contingency is to split `NPM_MODULES` handling per-module with independent timeouts rather than reverting the docs-site addition — flag to `devops` if this becomes an issue in practice, not a reason to hold this PR.
- Nothing in this feature touches `backend/` or `frontend/` build output, so there is no risk to the shipped Docker image / Charon binary from any part of this rollback story — worst case of a bad merge is a broken or stale docs site, never a broken product release.

---

## 7. Acceptance Criteria (Definition of Done for this PR)

- [ ] `docs-site/` builds cleanly (`npm run build`) with zero Docusaurus broken-link errors.
- [ ] `docs-site/docs/` is git-ignored and never committed; `docs/` is unmodified in layout/content by this PR (diff on `docs/**` is empty except through Commit 2's read-only sync script referencing it).
- [ ] Every path in `docs-site/scripts/docs-manifest.json` resolves to an existing `docs/` file or directory.
- [ ] `scripts/charon_dep_update.sh npm` completes successfully for all three `NPM_MODULES` entries including the new `docs-site` entry.
- [ ] `docs-site/audit-ci.json`-gated `npm run audit:ci` reports zero high/critical findings.
- [ ] `.github/workflows/docs-deploy.yml` passes actionlint (or repo-equivalent) and a manual `workflow_dispatch` build-job dry run.
- [ ] `.github/workflows/docs.yml`, `.github/pages/build-docs.sh`, `.github/pages/docs-index.html` are deleted with no remaining references elsewhere in the repo (`grep -rn "build-docs.sh\|docs-index.html" .github/` returns nothing after Commit 4).
- [ ] `README.md`'s documentation link points at a real, migrated Docusaurus page.
- [ ] `.gitignore` covers `docs-site/node_modules/`, `docs-site/docs/`, `docs-site/build/`, `docs-site/.docusaurus/`.
- [ ] `ARCHITECTURE.md` update is explicitly flagged as a follow-up for `docs-writer` in the PR description — not silently skipped, not written by this plan.
- [ ] Full repo Definition of Done from `CLAUDE.md` §"Task Completion Protocol" applies at PR level: patch coverage preflight, lefthook triage, staticcheck (N/A — no Go changes, but the gate still runs and must pass trivially), type-check (`docs-site` and unaffected `frontend`), build verification for both `backend` (unaffected, must still build) and `frontend` (unaffected, must still build).

---

## 8. Risks & Mitigations Summary

| Risk | Mitigation |
|---|---|
| Docusaurus TS-version ceiling conflicts with repo's TS 6 push elsewhere | `docs-site` pinned to TS 5.x independently, same pattern as the existing `frontend`/typescript exclusion in `charon_dep_update.sh` (§3.2, §4 Commit 3) |
| Two live Pages pipelines racing | Old pipeline deleted atomically in the same commit that adds the new one (§4 Commit 4) — no coexistence window |
| Content drift between `docs/` and the public site | Enforced structurally: `docs-site/docs/` is git-ignored and regenerated from an explicit manifest on every build/dev start (§2.2) — there is no editable second copy to drift |
| Manifest silently omits or wrongly includes a doc | Explicit allowlist (not exclude-list) means new internal `docs/` subdirs are safe-by-default; a broken/renamed path fails the build loudly (§3.7) rather than silently thinning the site |
| `baseUrl`/routing mismatch vs. old site's URL shape breaks external links (search engines, third-party links to `.../docs/getting-started.html`) | `baseUrl` chosen to match existing `/Charon/` prefix (§3.1); the `.html`-suffix old URLs will 404 under the new site regardless — acceptable one-time breakage for a docs site with low external backlink surface, not mitigated further in this pass (no redirect rules proposed; flag as a possible future `_redirects`-style addition if analytics show meaningful traffic loss) |
