#!/bin/bash
# Release script for CaddyProxyManager+
# Creates a new semantic version release with tag and GitHub release

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

success() {
    echo -e "${GREEN}$1${NC}"
}

warning() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    error "Not in a git repository"
fi

# Check for uncommitted changes
if [[ -n $(git status -s) ]]; then
    error "You have uncommitted changes. Please commit or stash them first."
fi

# Check if on correct branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" != "main" && "$CURRENT_BRANCH" != "development" ]]; then
    warning "You are on branch '$CURRENT_BRANCH'. Releases are typically from 'main' or 'development'."
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 0
    fi
fi

# Get current version from .version file
CURRENT_VERSION=$(cat .version 2>/dev/null || echo "0.0.0")
echo "Current version: $CURRENT_VERSION"

# Prompt for new version
echo ""
echo "Enter new version (e.g., 1.0.0, 1.0.0-beta.1, 1.0.0-rc.1):"
read -r NEW_VERSION

# Validate semantic version format
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    error "Invalid semantic version format. Expected: MAJOR.MINOR.PATCH[-PRERELEASE]"
fi

# Check if tag already exists
if git rev-parse "v$NEW_VERSION" >/dev/null 2>&1; then
    error "Tag v$NEW_VERSION already exists"
fi

# Update .version file
echo "$NEW_VERSION" > .version
success "Updated .version to $NEW_VERSION"

# Commit version bump
git add .version
git commit -m "chore: bump version to $NEW_VERSION"
success "Committed version bump"

# Create annotated tag
git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
success "Created tag v$NEW_VERSION"

# Show what will be pushed
echo ""
echo "Ready to push:"
echo "  - Commit: $(git rev-parse HEAD)"
echo "  - Tag: v$NEW_VERSION"
echo "  - Branch: $CURRENT_BRANCH"
echo ""

# Confirm push
read -p "Push to remote? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git push origin "$CURRENT_BRANCH"
    git push origin "v$NEW_VERSION"
    success "Pushed to remote!"
    echo ""
    success "Release workflow triggered!"
    echo "  - GitHub will create a release with changelog"
    echo "  - Docker images will be built and published"
    echo "  - View progress at: https://github.com/Wikid82/CaddyProxyManagerPlus/actions"
else
    warning "Not pushed. You can push later with:"
    echo "  git push origin $CURRENT_BRANCH"
    echo "  git push origin v$NEW_VERSION"
fi
