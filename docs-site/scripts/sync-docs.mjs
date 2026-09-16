#!/usr/bin/env node
// Copies the user-facing subset of the repo-root `docs/` directory into
// `docs-site/docs/` as Docusaurus content source.
//
// `docs/` is the single source of truth for Charon's documentation — this
// script never modifies it. `docs-site/docs/` is a build-time, git-ignored
// copy (see docs/plans/current_spec.md §2.2) that is deleted and recreated
// on every run so removing an entry from docs-manifest.json (or renaming a
// source file) can never leave a stale file behind.
//
// Run via `npm run sync-docs`, or automatically before `npm start` /
// `npm run build` (see package.json's `prestart`/`prebuild` hooks).

import { existsSync, cpSync, mkdirSync, rmSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { readFileSync } from 'node:fs';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const DOCS_SITE_ROOT = path.resolve(SCRIPT_DIR, '..');
const REPO_ROOT = path.resolve(DOCS_SITE_ROOT, '..');

const SOURCE_DOCS_DIR = path.join(REPO_ROOT, 'docs');
const DEST_DOCS_DIR = path.join(DOCS_SITE_ROOT, 'docs');
const MANIFEST_PATH = path.join(SCRIPT_DIR, 'docs-manifest.json');

function fail(message) {
  console.error(`[sync-docs] ERROR: ${message}`);
  process.exit(1);
}

function loadManifest() {
  let raw;
  try {
    raw = readFileSync(MANIFEST_PATH, 'utf-8');
  } catch (err) {
    fail(`could not read manifest at ${MANIFEST_PATH}: ${err.message}`);
  }

  let manifest;
  try {
    manifest = JSON.parse(raw);
  } catch (err) {
    fail(`manifest at ${MANIFEST_PATH} is not valid JSON: ${err.message}`);
  }

  const files = Array.isArray(manifest.files) ? manifest.files : [];
  const directories = Array.isArray(manifest.directories) ? manifest.directories : [];
  return { files, directories };
}

function copyFile(relativePath) {
  const source = path.join(SOURCE_DOCS_DIR, relativePath);
  if (!existsSync(source) || !statSync(source).isFile()) {
    fail(
      `manifest "files" entry "${relativePath}" does not exist as a file under ` +
        `${SOURCE_DOCS_DIR} — update docs-site/scripts/docs-manifest.json if it was renamed or removed.`,
    );
  }

  const dest = path.join(DEST_DOCS_DIR, relativePath);
  mkdirSync(path.dirname(dest), { recursive: true });
  cpSync(source, dest);
  console.log(`[sync-docs] copied file ${relativePath}`);
}

function copyDirectory(relativePath) {
  const source = path.join(SOURCE_DOCS_DIR, relativePath);
  if (!existsSync(source) || !statSync(source).isDirectory()) {
    fail(
      `manifest "directories" entry "${relativePath}" does not exist as a directory under ` +
        `${SOURCE_DOCS_DIR} — update docs-site/scripts/docs-manifest.json if it was renamed or removed.`,
    );
  }

  const dest = path.join(DEST_DOCS_DIR, relativePath);
  mkdirSync(path.dirname(dest), { recursive: true });
  cpSync(source, dest, { recursive: true });
  console.log(`[sync-docs] copied directory ${relativePath}/`);
}

function main() {
  if (!existsSync(SOURCE_DOCS_DIR)) {
    fail(`source docs directory not found at ${SOURCE_DOCS_DIR}`);
  }

  const { files, directories } = loadManifest();

  // Idempotent: wipe and recreate so a manifest entry removed since the last
  // run never leaves a stale, orphaned file in docs-site/docs/.
  rmSync(DEST_DOCS_DIR, { recursive: true, force: true });
  mkdirSync(DEST_DOCS_DIR, { recursive: true });

  for (const relativePath of files) {
    copyFile(relativePath);
  }
  for (const relativePath of directories) {
    copyDirectory(relativePath);
  }

  console.log(
    `[sync-docs] done — synced ${files.length} file(s) and ${directories.length} ` +
      `director(y/ies) from ${SOURCE_DOCS_DIR} into ${DEST_DOCS_DIR}`,
  );
}

main();
