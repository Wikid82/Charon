# Go Version Upgrades

**Last Updated:** 2026-02-12

## The Short Version

When Charon upgrades to a new Go version, your development tools (like golangci-lint) break. Here's how to fix it:

```bash
# Step 1: Pull latest code
git pull

# Step 2: Update your Go installation
.github/skills/scripts/skill-runner.sh utility-update-go-version

# Step 3: Rebuild tools
./scripts/rebuild-go-tools.sh

# Step 4: Restart your IDE
# VS Code: Cmd/Ctrl+Shift+P → "Developer: Reload Window"
```

That's it! Keep reading if you want to understand why.

---

## What's Actually Happening?

### The Problem (In Plain English)

Think of Go tools like a Swiss Army knife. When you upgrade Go, it's like switching from metric to imperial measurements—your old knife still works, but the measurements don't match anymore.

Here's what breaks:

1. **Renovate updates the project** to Go 1.26.0
2. **Your tools are still using** Go 1.25.6
3. **Pre-commit hooks fail** with confusing errors
4. **Your IDE gets confused** and shows red squiggles everywhere

### Why Tools Break

Development tools like golangci-lint are compiled programs. They were built with Go 1.25.6 and expect Go 1.25.6's features. When you upgrade to Go 1.26.0:

- New language features exist that old tools don't understand
- Standard library functions change
- Your tools throw errors like: `undefined: someNewFunction`

**The Fix:** Rebuild tools with the new Go version so they match your project.

---

## Step-by-Step Upgrade Guide

### Step 1: Know When an Upgrade Happened

Renovate (our automated dependency manager) will open a PR titled something like:

```
chore(deps): update golang to v1.26.0
```

When this gets merged, you'll need to update your local environment.

### Step 2: Pull the Latest Code

```bash
cd /projects/Charon
git checkout development
git pull origin development
```

### Step 3: Update Your Go Installation

**Option A: Use the Automated Skill (Recommended)**

```bash
.github/skills/scripts/skill-runner.sh utility-update-go-version
```

This script:

- Detects the required Go version from `go.work`
- Downloads it from golang.org
- Installs it to `~/sdk/go{version}/`
- Updates your system symlink to point to it
- Rebuilds your tools automatically

**Option B: Manual Installation**

If you prefer to install Go manually:

1. Go to [go.dev/dl](https://go.dev/dl/)
2. Download the version mentioned in the PR (e.g., 1.26.0)
3. Install it following the official instructions
4. Verify: `go version` should show the new version
5. Continue to Step 4

### Step 4: Rebuild Development Tools

Even if you used Option A (which rebuilds automatically), you can always manually rebuild:

```bash
./scripts/rebuild-go-tools.sh
```

This rebuilds:

- **golangci-lint** — Pre-commit linter (critical)
- **gopls** — IDE language server (critical)
- **govulncheck** — Security scanner
- **dlv** — Debugger

**Duration:** About 30 seconds

**Output:** You'll see:

```
🔧 Rebuilding Go development tools...
Current Go version: go version go1.26.0 linux/amd64

📦 Installing golangci-lint...
✅ golangci-lint installed successfully

📦 Installing gopls...
✅ gopls installed successfully

...

✅ All tools rebuilt successfully!
```

### Step 5: Restart Your IDE

Your IDE caches the old Go language server (gopls). Reload to use the new one:

**VS Code:**

- Press `Cmd/Ctrl+Shift+P`
- Type "Developer: Reload Window"
- Press Enter

**GoLand or IntelliJ IDEA:**

- File → Invalidate Caches → Restart
- Wait for indexing to complete

### Step 6: Verify Everything Works

Run a quick test:

```bash
# This should pass without errors
go test ./backend/...
```

If tests pass, you're done! 🎉

---

## Troubleshooting

### Error: "golangci-lint: command not found"

**Problem:** Your `$PATH` doesn't include Go's binary directory.

**Fix:**

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"

# Reload your shell
source ~/.bashrc  # or source ~/.zshrc
```

Then rebuild tools:

```bash
./scripts/rebuild-go-tools.sh
```

### Error: Pre-commit hook still failing

**Problem:** Pre-commit is using a cached version of the tool.

**Fix 1: Let the hook auto-rebuild**

The pre-commit hook detects version mismatches and rebuilds automatically. Just commit again:

```bash
git commit -m "your message"
# Hook detects mismatch, rebuilds tool, and retries
```

**Fix 2: Manual rebuild**

```bash
./scripts/rebuild-go-tools.sh
git commit -m "your message"
```

### Error: "package X is not in GOROOT"

**Problem:** Your project's `go.work` or `go.mod` specifies a Go version you don't have installed.

**Check required version:**

```bash
grep '^go ' go.work
# Output: go 1.26.0
```

**Install that version:**

```bash
.github/skills/scripts/skill-runner.sh utility-update-go-version
```

### IDE showing errors but code compiles fine

**Problem:** Your IDE's language server (gopls) is out of date.

**Fix:**

```bash
# Rebuild gopls
go install golang.org/x/tools/gopls@latest

# Restart IDE
# VS Code: Cmd/Ctrl+Shift+P → "Developer: Reload Window"
```

### "undefined: someFunction" errors

**Problem:** Your tools were built with an old Go version and don't recognize new standard library functions.

**Fix:**

```bash
./scripts/rebuild-go-tools.sh
```

---

## Frequently Asked Questions

### How often do Go versions change?

Go releases **two major versions per year**:

- February (e.g., Go 1.26.0)
- August (e.g., Go 1.27.0)

Plus occasional patch releases (e.g., Go 1.26.2) for security fixes.

**Bottom line:** Expect to run `./scripts/rebuild-go-tools.sh` 2-3 times per year.

### Do I need to rebuild tools for patch releases?

**Usually no**, but it doesn't hurt. Patch releases (like 1.26.0 → 1.26.2) rarely break tool compatibility.

**Rebuild if:**

- Pre-commit hooks start failing
- IDE shows unexpected errors
- Tools report version mismatches

### Why don't CI builds have this problem?

CI environments are **ephemeral** (temporary). Every workflow run:

1. Starts with a fresh container
2. Installs Go from scratch
3. Installs tools from scratch
4. Runs tests
5. Throws everything away

**Local development** has persistent tool installations that get out of sync.

### Can I use multiple Go versions on my machine?

**Yes!** Go officially supports this via `golang.org/dl`:

```bash
# Install Go 1.25.6
go install golang.org/dl/go1.25.6@latest
go1.25.6 download

# Install Go 1.26.0
go install golang.org/dl/go1.26.0@latest
go1.26.0 download

# Use specific version
go1.25.6 version
go1.26.0 test ./...
```

But for Charon development, you only need **one version** (whatever's in `go.work`).

### What if I skip an upgrade?

**Short answer:** Your local tools will be out of sync, but CI will still work.

**What breaks:**

- Pre-commit hooks fail (but will auto-rebuild)
- IDE shows phantom errors
- Manual `go test` might fail locally
- CI is unaffected (it always uses the correct version)

**When to catch up:**

- Before opening a PR (CI checks will fail if your code uses old Go features)
- When local development becomes annoying

### Should I keep old Go versions installed?

**No need.** The upgrade script preserves old versions in `~/sdk/`, but you don't need to do anything special.

If you want to clean up:

```bash
# See installed versions
ls ~/sdk/

# Remove old versions
rm -rf ~/sdk/go1.25.5
rm -rf ~/sdk/go1.25.6
```

But they only take ~400MB each, so cleanup is optional.

### Why doesn't Renovate upgrade tools automatically?

Renovate updates **Dockerfile** and **go.work**, but it can't update tools on *your* machine.

**Think of it like this:**

- Renovate: "Hey team, we're now using Go 1.26.0"
- Your machine: "Cool, but my tools are still Go 1.25.6. Let me rebuild them."

The rebuild script bridges that gap.

### What's the difference between `go.work`, `go.mod`, and my system Go?

**`go.work`** — Workspace file (multi-module projects like Charon)

- Specifies minimum Go version for the entire project
- Used by Renovate to track upgrades

**`go.mod`** — Module file (individual Go modules)

- Each module (backend, tools) has its own `go.mod`
- Inherits Go version from `go.work`

**System Go** (`go version`) — What's installed on your machine

- Must be >= the version in `go.work`
- Tools are compiled with whatever version this is

**Example:**

```
go.work says: "Use Go 1.26.0 or newer"
go.mod says: "I'm part of the workspace, use its Go version"
Your machine: "I have Go 1.26.0 installed"
Tools: "I was built with Go 1.25.6" ❌ MISMATCH
```

Running `./scripts/rebuild-go-tools.sh` fixes the mismatch.

---

## Advanced: Pre-commit Auto-Rebuild

Charon's pre-commit hook automatically detects and fixes tool version mismatches.

**How it works:**

1. **Check versions:**

   ```bash
   golangci-lint version → "built with go1.25.6"
   go version → "go version go1.26.0"
   ```

2. **Detect mismatch:**

   ```
   ⚠️  golangci-lint Go version mismatch:
      golangci-lint: 1.25.6
      system Go:     1.26.0
   ```

3. **Auto-rebuild:**

   ```
   🔧 Rebuilding golangci-lint with current Go version...
   ✅ golangci-lint rebuilt successfully
   ```

4. **Retry linting:**
   Hook runs again with the rebuilt tool.

**What this means for you:**

The first commit after a Go upgrade will be **slightly slower** (~30 seconds for tool rebuild). Subsequent commits are normal speed.

**Disabling auto-rebuild:**

If you want manual control, edit `scripts/pre-commit-hooks/golangci-lint-fast.sh` and remove the rebuild logic. (Not recommended.)

---

## Related Documentation

- **[Go Version Management Strategy](../plans/go_version_management_strategy.md)** — Research and design decisions
- **[CONTRIBUTING.md](../../CONTRIBUTING.md)** — Quick reference for contributors
- **[Go Official Docs](https://go.dev/doc/manage-install)** — Official multi-version management guide

---

## Need Help?

**Open a [Discussion](https://github.com/Wikid82/charon/discussions)** if:

- These instructions didn't work for you
- You're seeing errors not covered in troubleshooting
- You have suggestions for improving this guide

**Open an [Issue](https://github.com/Wikid82/charon/issues)** if:

- The rebuild script crashes
- Pre-commit auto-rebuild isn't working
- CI is failing for Go version reasons

---

**Remember:** Go upgrades happen 2-3 times per year. When they do, just run `./scripts/rebuild-go-tools.sh` and you're good to go! 🚀
