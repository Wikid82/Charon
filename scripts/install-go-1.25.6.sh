#!/usr/bin/env bash
set -euo pipefail

# Script to install Go 1.25.6 to /usr/local/go
# Usage: sudo ./scripts/install-go-1.25.6.sh

GO_VERSION="1.25.6"
ARCH="linux-amd64"
TARFILE="go${GO_VERSION}.${ARCH}.tar.gz"
TMPFILE="/tmp/${TARFILE}"
# Ensure GOPATH is set
: ${GOPATH:=$HOME/go}
: ${GOBIN:=${GOPATH}/bin}

# Download
if [ ! -f "$TMPFILE" ]; then
  echo "Downloading go${GO_VERSION}..."
  curl -sSfL -o "$TMPFILE" "https://go.dev/dl/${TARFILE}"
fi

# Remove existing installation
if [ -d "/usr/local/go" ]; then
  echo "Removing existing /usr/local/go..."
  sudo rm -rf /usr/local/go
fi

# Extract
echo "Extracting to /usr/local..."
sudo tar -C /usr/local -xzf "$TMPFILE"

# Setup system PATH via /etc/profile.d
echo "Creating /etc/profile.d/go.sh to export /usr/local/go/bin and GOPATH/bin"
sudo tee /etc/profile.d/go.sh > /dev/null <<'EOF'
export PATH=/usr/local/go/bin:$GOPATH/bin:$PATH
EOF
sudo chmod +x /etc/profile.d/go.sh

# Update current session PATH
export PATH=/usr/local/go/bin:$GOPATH/bin:$PATH

# Verify
echo "Installed go: $(go version)"

# Optionally install gopls
echo "Installing gopls..."
# renovate: datasource=go depName=golang.org/x/tools
go install golang.org/x/tools/gopls@v0.42.0

GOPLS_PATH="$GOPATH/bin/gopls"
if [ -f "$GOPLS_PATH" ]; then
  echo "gopls installed at $GOPLS_PATH"
  $GOPLS_PATH version || true
else
  echo "gopls not installed in GOPATH/bin"
fi

cat <<'EOF'
Done. Please restart your shell or run:
  source /etc/profile.d/go.sh
and restart your editor's Go language server (Go: Restart Language Server in VS Code)
EOF
