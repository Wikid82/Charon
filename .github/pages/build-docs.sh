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
# What it does:
#   1. Copy README.md, docs/ and the landing page into _site/.
#   2. Strip YAML front matter, then render every markdown file to HTML.
#   3. Wrap each generated page with a nav header + SEO metadata + footer.
#      Per-page <title>, <meta name="description">, canonical URL, robots and
#      Open Graph / Twitter Card tags are derived from the page's front matter
#      (preferred) or its first heading / first prose line.
#   4. Rewrite repo-relative paths for the deployed base path.
#   5. Emit sitemap.xml and robots.txt.
#
# The landing page (_site/index.html, from .github/pages/docs-index.html) is
# copied verbatim and is NOT processed by the per-page wrap loop — it carries
# its own hand-authored metadata.
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
#   - `perl` (present on GitHub-hosted runners and used only for text munging).
#   - Run from the repository root.
#
set -euo pipefail

REPO_NAME="${REPO_NAME:?REPO_NAME must be set (repository short name, e.g. Charon)}"
FULL_REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set (owner/repo, e.g. Wikid82/Charon)}"

# Deployed site root. Repo-name casing is preserved on purpose: GitHub Pages is
# served from https://wikid82.github.io/Charon/ (capital C).
SITE_URL_BASE="https://wikid82.github.io/${REPO_NAME}"
OG_IMAGE="https://raw.githubusercontent.com/Wikid82/Charon/refs/heads/main/frontend/public/banner.webp"
GENERIC_DESCRIPTION="Charon is the self-hosted, open-source web-UI alternative to Nginx Proxy Manager and Traefik, built on Caddy. Automatic HTTPS, WAF, and CrowdSec included."
BUILD_DATE="$(date +%F)"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# strip_frontmatter <file>  — echo <file> to stdout with a leading YAML front
# matter block (--- ... --- / ...) removed. Files without front matter pass
# through byte-for-byte.
strip_frontmatter() {
  perl -0777 -pe 's/\A---\r?\n.*?\r?\n(?:---|\.\.\.)\r?\n//s' "$1"
}

# site_rel <_site/path>  — relative URL path for a file under _site/, with a
# trailing "index.html" collapsed to a directory URL.
site_rel() {
  local p="${1#_site/}"
  case "$p" in
    index.html)   p="" ;;
    */index.html) p="${p%index.html}" ;;
  esac
  printf '%s' "$p"
}

# derive_meta <src_md> <rendered_html> <basename>  — print two lines:
#   line 1: page <title>  ("<Heading> — Charon Docs")
#   line 2: meta description (<= ~155 chars, generic fallback)
# Both are HTML-attribute-escaped, ready to drop straight into the template.
# Front matter title/description win; otherwise the first <h1> / first prose
# line is used, with a titlecased filename as the last-resort title.
derive_meta() {
  perl -CSDA - "$1" "$2" "$3" "$GENERIC_DESCRIPTION" <<'PERL'
use strict;
use warnings;

my ($md_path, $html_path, $basename, $generic) = @ARGV;

my ($fm_title, $fm_desc);
my @body;

if (open(my $fh, '<:encoding(UTF-8)', $md_path)) {
    my @lines = <$fh>;
    close $fh;
    chomp @lines;
    if (@lines && $lines[0] =~ /^---\s*$/) {
        my $i = 1;
        while ($i <= $#lines && $lines[$i] !~ /^(?:---|\.\.\.)\s*$/) {
            if    ($lines[$i] =~ /^title:\s*(.+?)\s*$/)       { $fm_title = unquote($1); }
            elsif ($lines[$i] =~ /^description:\s*(.+?)\s*$/)  { $fm_desc  = unquote($1); }
            $i++;
        }
        @body = ($i < $#lines) ? @lines[($i + 1) .. $#lines] : ();
    } else {
        @body = @lines;
    }
}

sub unquote { my $s = shift; $s =~ s/^["']//; $s =~ s/["']$//; return $s; }

sub clean_inline {
    my $s = shift // '';
    $s =~ s/<[^>]+>//g;                       # strip tags
    $s =~ s/&amp;/&/g; $s =~ s/&lt;/</g; $s =~ s/&gt;/>/g;
    $s =~ s/&quot;/"/g; $s =~ s/&#0*39;/'/g; $s =~ s/&#x0*27;/'/gi;
    # drop emoji / dingbats / arrows / variation selectors / combining marks
    $s =~ s/[\x{1F000}-\x{1FAFF}\x{2190}-\x{21FF}\x{2300}-\x{27BF}\x{2B00}-\x{2BFF}\x{FE00}-\x{FE0F}\x{200D}\x{20D0}-\x{20FF}\x{2600}-\x{26FF}]//g;
    $s =~ s/\s+/ /g;
    $s =~ s/^\s+|\s+$//g;
    return $s;
}

# ---- Title -------------------------------------------------------------------
my $heading = $fm_title;
if (!defined $heading || $heading eq '') {
    if (open(my $hh, '<:encoding(UTF-8)', $html_path)) {
        local $/;
        my $html = <$hh>;
        close $hh;
        $heading = $1 if $html =~ /<h1[^>]*>(.*?)<\/h1>/s;
    }
}
$heading = clean_inline($heading) if defined $heading;
if (!defined $heading || $heading eq '') {
    ($heading = $basename) =~ s/[-_]+/ /g;
    $heading =~ s/\b(\w)/\U$1/g;
}
my $title = "$heading — Charon Docs";

# ---- Description -----------------------------------------------------------
my $desc = $fm_desc;
if (!defined $desc || $desc eq '') {
    my $in_fence = 0;
    for my $ln (@body) {
        if ($ln =~ /^\s*(?:```|~~~)/) { $in_fence = !$in_fence; next; }
        next if $in_fence;
        next if $ln =~ /^\s*$/;               # blank
        next if $ln =~ /^\s*#/;               # heading
        next if $ln =~ /^\s*</;               # html block / tag
        next if $ln =~ /^\s*[-*_]{3,}\s*$/;   # thematic break
        next if $ln =~ /^\s*!?\[!\[/;         # badge line
        next if $ln =~ /^\s*!\[/;             # image
        next if $ln =~ /^\s*>/;               # blockquote
        $desc = $ln;
        last;
    }
}
$desc //= '';
$desc =~ s/!\[[^\]]*\]\([^)]*\)//g;           # images
$desc =~ s/\[([^\]]+)\]\([^)]*\)/$1/g;        # links -> text
$desc =~ s/[`*_]//g;                          # inline emphasis / code marks
$desc = clean_inline($desc);
# Anything shorter than this is metadata noise ("**Version:** v0.9.0+"), not a
# real summary sentence — fall back to the generic description instead.
$desc = $generic if length($desc) < 40;
if (length($desc) > 155) {
    my $cut = substr($desc, 0, 155);
    $cut =~ s/\s+\S*$//;                      # break on a word boundary
    $desc = "$cut…";
}

# HTML-attribute escape (order matters: ampersand first).
for my $v ($title, $desc) {
    $v =~ s/&/&amp;/g;
    $v =~ s/</&lt;/g;
    $v =~ s/>/&gt;/g;
    $v =~ s/"/&quot;/g;
}

print "$title\n$desc\n";
PERL
}

# emit_head <title> <description> <canonical>  — write the SEO <head> + nav.
# This is the per-page template: the placeholders are shell variables expanded
# by the heredoc, which keeps values with '&', '/' or quotes intact. A literal
# sed / `${var//}` template would need fragile escaping here — and bash >= 5.2
# treats a bare '&' in a `${var//pat/repl}` replacement as the matched text.
# title / description arrive already HTML-attribute-escaped from derive_meta.
emit_head() {
  local title_esc="$1" desc_esc="$2" canonical="$3"
  cat <<HDR
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${title_esc}</title>
  <meta name="description" content="${desc_esc}">
  <meta name="robots" content="index,follow">
  <link rel="canonical" href="${canonical}">
  <meta property="og:type" content="article">
  <meta property="og:site_name" content="Charon">
  <meta property="og:title" content="${title_esc}">
  <meta property="og:description" content="${desc_esc}">
  <meta property="og:url" content="${canonical}">
  <meta property="og:image" content="${OG_IMAGE}">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="${title_esc}">
  <meta name="twitter:description" content="${desc_esc}">
  <meta name="twitter:image" content="${OG_IMAGE}">
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
HDR
}

# wrap_page <_site/....html>  — replace the bare marked output with the full
# page: SEO <head> + nav + original body + footer.
wrap_page() {
  local html_file="$1"
  local temp_file="${html_file}.tmp"
  local src_md="${html_file%.html}.md"
  local basename title desc canonical
  basename="$(basename "$html_file" .html)"

  local meta
  mapfile -t meta < <(derive_meta "$src_md" "$html_file" "$basename")
  title="${meta[0]:-$basename — Charon Docs}"
  desc="${meta[1]:-$GENERIC_DESCRIPTION}"
  canonical="${SITE_URL_BASE}/$(site_rel "$html_file")"

  emit_head "$title" "$desc" "$canonical" > "$temp_file"
  cat "$html_file" >> "$temp_file"
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

# ---------------------------------------------------------------------------
# 1. Assemble the raw _site/ tree
# ---------------------------------------------------------------------------
mkdir -p _site

cp README.md _site/
cp -r docs _site/
cp .github/pages/docs-index.html _site/index.html
[ -f CONTRIBUTING.md ] && cp CONTRIBUTING.md _site/

# ---------------------------------------------------------------------------
# 2. Convert markdown to HTML (front matter stripped first)
# ---------------------------------------------------------------------------
convert_md() {
  local md="$1" out="$2" tmp
  tmp="$(mktemp)"
  strip_frontmatter "$md" > "$tmp"
  marked "$tmp" -o "$out" --gfm
  rm -f "$tmp"
}

for file in _site/docs/*.md; do
  [ -f "$file" ] || continue
  convert_md "$file" "_site/docs/$(basename "$file" .md).html"
done

convert_md _site/README.md _site/README.html
[ -f _site/CONTRIBUTING.md ] && convert_md _site/CONTRIBUTING.md _site/CONTRIBUTING.html

# ---------------------------------------------------------------------------
# 3. Wrap every generated page (all except the landing index.html)
# ---------------------------------------------------------------------------
for html_file in _site/*.html _site/docs/*.html; do
  if [ -f "$html_file" ] && [ "$html_file" != "_site/index.html" ]; then
    wrap_page "$html_file"
  fi
done

# ---------------------------------------------------------------------------
# 4. Robust dynamic path fix
# ---------------------------------------------------------------------------
echo "🔧 Calculating paths..."

if [[ "${REPO_NAME}" == *".github.io" ]]; then
  echo "  - Mode: Root domain (e.g. user.github.io)"
  BASE_PATH="/"
else
  echo "  - Mode: Sub-path (e.g. user.github.io/repo)"
  BASE_PATH="/${REPO_NAME}/"
fi

REPO_URL="https://github.com/${FULL_REPO}"

echo "  - Repo: ${FULL_REPO}"
echo "  - URL:  ${REPO_URL}"
echo "  - Base: ${BASE_PATH}"

# Rules target lowercase "/charon/" and "github.com/Wikid82/charon" only, so the
# canonical / Open Graph URLs (which use the real "Charon" casing) are untouched.
find _site -name "*.html" -exec sed -i \
  -e "s|/charon/|${BASE_PATH}|g" \
  -e "s|https://github.com/Wikid82/charon|${REPO_URL}|g" \
  -e "s|Wikid82/charon|${FULL_REPO}|g" \
  {} +

echo "✅ Paths fixed successfully!"

# ---------------------------------------------------------------------------
# 5. sitemap.xml + robots.txt
# ---------------------------------------------------------------------------
{
  echo '<?xml version="1.0" encoding="UTF-8"?>'
  echo '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'
  while IFS= read -r f; do
    printf '  <url>\n    <loc>%s/%s</loc>\n    <lastmod>%s</lastmod>\n  </url>\n' \
      "$SITE_URL_BASE" "$(site_rel "$f")" "$BUILD_DATE"
  done < <(find _site -name '*.html' | sort)
  echo '</urlset>'
} > _site/sitemap.xml

{
  echo 'User-agent: *'
  echo 'Allow: /'
  echo ''
  echo "Sitemap: ${SITE_URL_BASE}/sitemap.xml"
} > _site/robots.txt

echo "✅ sitemap.xml + robots.txt written."
echo "✅ Documentation site built successfully!"
