#!/usr/bin/env bash
#
# build-docs.sh — assemble the GitHub Pages documentation site into ./_site/.
#
# This logic used to live inline in the "📝 Build documentation site" step of
# .github/workflows/docs.yml. It was extracted here for the same reason
# .github/pages/docs-index.html was extracted earlier: a large inline heredoc
# in that workflow step hangs actionlint's shellcheck integration. Keeping the
# logic in a committed, shellcheck-clean script lets the workflow YAML stay
# thin (a single `bash .github/pages/build-docs.sh` call).
#
# Inputs (environment variables):
#   REPO_NAME          Repository short name, e.g. "Charon".
#                      On CI this is github.event.repository.name.
#   GITHUB_REPOSITORY  "owner/repo", e.g. "Wikid82/Charon".
#                      Set automatically on GitHub Actions runners.
#
# Requirements:
#   - `marked` must be on PATH (the workflow step runs `npm install -g marked`
#     before invoking this script; install it the same way to run locally).
#   - Run from the repository root.
#
set -euo pipefail

REPO_NAME="${REPO_NAME:?REPO_NAME must be set (repository short name, e.g. Charon)}"
FULL_REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set (owner/repo, e.g. Wikid82/Charon)}"

# ---------------------------------------------------------------------------
# 1. Assemble the raw _site/ tree
# ---------------------------------------------------------------------------
mkdir -p _site

# Copy all markdown files
cp README.md _site/
cp -r docs _site/

# Landing page — content lives in .github/pages/docs-index.html, owned by the
# docs authors. Copied verbatim; NOT processed by the per-page wrap loop below.
cp .github/pages/docs-index.html _site/index.html

# ---------------------------------------------------------------------------
# 2. Convert markdown to HTML
# ---------------------------------------------------------------------------
for file in _site/docs/*.md; do
  if [ -f "$file" ]; then
    filename=$(basename "$file" .md)
    marked "$file" -o "_site/docs/${filename}.html" --gfm
  fi
done

# Convert README and CONTRIBUTING
marked _site/README.md -o _site/README.html --gfm
if [ -f "CONTRIBUTING.md" ]; then
  cp CONTRIBUTING.md _site/
  marked _site/CONTRIBUTING.md -o _site/CONTRIBUTING.html --gfm
fi

# ---------------------------------------------------------------------------
# 3. Wrap every generated page (nav header + footer)
#    The landing page (_site/index.html) is skipped — left exactly as the
#    docs authors set it.
# ---------------------------------------------------------------------------
wrap_page() {
  html_file="$1"
  temp_file="${html_file}.tmp"

  cat > "$temp_file" << 'HEADER'
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Charon - Documentation</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <style>
    body { background-color: #0f172a; color: #e2e8f0; }
    nav { background: #1e293b; padding: 1rem; margin-bottom: 2rem; }
    nav a { color: #60a5fa; margin-right: 1rem; text-decoration: none; }
    nav a:hover { color: #93c5fd; }
    main { max-width: 900px; margin: 0 auto; padding: 2rem; }
    a { color: #60a5fa; }
    code { background: #1e293b; color: #fbbf24; padding: 0.2rem 0.4rem; border-radius: 4px; }
    pre { background: #1e293b; padding: 1rem; border-radius: 8px; overflow-x: auto; }
    pre code { background: none; padding: 0; }
  </style>
</head>
<body>
  <nav>
    <a href="/charon/">🏠 Home</a>
    <a href="/charon/docs/index.html">📚 Docs</a>
    <a href="/charon/docs/getting-started.html">🚀 Get Started</a>
    <a href="https://github.com/Wikid82/charon">⭐ GitHub</a>
  </nav>
  <main>
HEADER

  # Append original content
  cat "$html_file" >> "$temp_file"

  # Add footer
  cat >> "$temp_file" << 'FOOTER'
  </main>
  <footer style="text-align: center; padding: 2rem; color: #64748b;">
    <p>Charon - Built with ❤️ for the community</p>
  </footer>
</body>
</html>
FOOTER

  mv "$temp_file" "$html_file"
}

for html_file in _site/*.html _site/docs/*.html; do
  if [ -f "$html_file" ] && [ "$html_file" != "_site/index.html" ]; then
    wrap_page "$html_file"
  fi
done

# ---------------------------------------------------------------------------
# 4. Robust dynamic path fix
# ---------------------------------------------------------------------------
echo "🔧 Calculating paths..."

# 4a. Determine BASE_PATH
if [[ "${REPO_NAME}" == *".github.io" ]]; then
  echo "  - Mode: Root domain (e.g. user.github.io)"
  BASE_PATH="/"
else
  echo "  - Mode: Sub-path (e.g. user.github.io/repo)"
  BASE_PATH="/${REPO_NAME}/"
fi

# 4b. Standard repo variables
REPO_URL="https://github.com/${FULL_REPO}"

echo "  - Repo: ${FULL_REPO}"
echo "  - URL:  ${REPO_URL}"
echo "  - Base: ${BASE_PATH}"

# 4c. Fix paths in all HTML files
find _site -name "*.html" -exec sed -i \
  -e "s|/charon/|${BASE_PATH}|g" \
  -e "s|https://github.com/Wikid82/charon|${REPO_URL}|g" \
  -e "s|Wikid82/charon|${FULL_REPO}|g" \
  {} +

echo "✅ Paths fixed successfully!"

echo "✅ Documentation site built successfully!"
