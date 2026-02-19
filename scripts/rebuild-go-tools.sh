#!/usr/bin/env bash
# Rebuild Go development tools with the current Go version
# This ensures tools like golangci-lint are compiled with the same Go version as the project

set -euo pipefail

echo "🔧 Rebuilding Go development tools..."
echo "Current Go version: $(go version)"
echo ""

# Core development tools (ordered by priority)
declare -A TOOLS=(
    ["golangci-lint"]="github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    ["gopls"]="golang.org/x/tools/gopls@latest"
    ["govulncheck"]="golang.org/x/vuln/cmd/govulncheck@latest"
    ["dlv"]="github.com/go-delve/delve/cmd/dlv@latest"
)

FAILED_TOOLS=()
SUCCESSFUL_TOOLS=()

for tool_name in "${!TOOLS[@]}"; do
    tool_path="${TOOLS[$tool_name]}"
    echo "📦 Installing $tool_name..."
    if go install "$tool_path" 2>&1; then
        SUCCESSFUL_TOOLS+=("$tool_name")
        echo "✅ $tool_name installed successfully"
    else
        FAILED_TOOLS+=("$tool_name")
        echo "❌ Failed to install $tool_name"
    fi
    echo ""
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Tool rebuild complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Installed versions:"
echo ""

# Display versions for each tool
if command -v golangci-lint >/dev/null 2>&1; then
    echo "golangci-lint:"
    golangci-lint version 2>&1 | grep -E 'version|built with' | sed 's/^/  /'
else
    echo "  golangci-lint: not found in PATH"
fi
echo ""

if command -v gopls >/dev/null 2>&1; then
    echo "gopls:"
    gopls version 2>&1 | head -1 | sed 's/^/  /'
else
    echo "  gopls: not found in PATH"
fi
echo ""

if command -v govulncheck >/dev/null 2>&1; then
    echo "govulncheck:"
    govulncheck -version 2>&1 | sed 's/^/  /'
else
    echo "  govulncheck: not found in PATH"
fi
echo ""

if command -v dlv >/dev/null 2>&1; then
    echo "dlv:"
    dlv version 2>&1 | head -1 | sed 's/^/  /'
else
    echo "  dlv: not found in PATH"
fi
echo ""

# Summary
if [ ${#FAILED_TOOLS[@]} -eq 0 ]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ All tools rebuilt successfully!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
else
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "⚠️  Some tools failed to install:"
    for tool in "${FAILED_TOOLS[@]}"; do
        echo "   - $tool"
    done
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
fi
