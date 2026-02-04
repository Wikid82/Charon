---
description: 'Best practices for writing clear, consistent, and meaningful Git commit messages'
applyTo: '**'
---

## AI-Specific Requirements (Mandatory)

When generating commit messages automatically:

- ❌ DO NOT mention file names, paths, or extensions
- ❌ DO NOT mention line counts, diffs, or change statistics
  (e.g. "+10 -2", "updated file", "modified spec")
- ❌ DO NOT describe changes as "edited", "updated", or "changed files"

- ✅ DO describe the behavioral, functional, or logical change
- ✅ DO explain WHY the change was made
- ✅ DO assume the reader CANNOT see the diff

**Litmus Test**:
If someone reads only the commit message, they should understand:
- What changed
- Why it mattered
- What behavior is different now

```

# Git Commit Message Best Practices

Comprehensive guidelines for crafting high-quality commit messages that improve code review efficiency, project documentation, and team collaboration. Based on industry standards and the conventional commits specification.

## Why Good Commit Messages Matter

- **Future Reference**: Commit messages serve as project documentation
- **Code Review**: Clear messages speed up review processes
- **Debugging**: Easy to trace when and why changes were introduced
- **Collaboration**: Helps team members understand project evolution
- **Search and Filter**: Well-structured messages are easier to search
- **Automation**: Enables automated changelog generation and semantic versioning

## Commit Message Structure

A Git commit message consists of two parts:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Summary/Title (Required)

- **Character Limit**: 50 characters (hard limit: 72)
- **Format**: `<type>(<scope>): <subject>`
- **Imperative Mood**: Use "Add feature" not "Added feature" or "Adds feature"
- **No Period**: Don't end with punctuation
- **Lowercase Type**: Use lowercase for the type prefix

**Test Formula**: "If applied, this commit will [your commit message]"

✅ **Good**: `If applied, this commit will fix login redirect bug`
❌ **Bad**: `If applied, this commit will fixed login redirect bug`

### Description/Body (Optional but Recommended)

- **When to Use**: Complex changes, breaking changes, or context needed
- **Character Limit**: Wrap at 72 characters per line
- **Content**: Explain WHAT changed and WHY (not HOW - code shows that)
- **Blank Line**: Separate body from title with one blank line
- **Multiple Paragraphs**: Allowed, separated by blank lines
- **Lists**: Use bullets (`-` or `*`) or numbered lists

### Footer (Optional)

- **Breaking Changes**: `BREAKING CHANGE: description`
- **Issue References**: `Closes #123`, `Fixes #456`, `Refs #789`
- **Pull Request References**: `Related to PR #100`
- **Co-authors**: `Co-authored-by: Name <email>`

## Conventional Commit Types

Use these standardized types for consistency and automated tooling:

| Type | Description | Example | When to Use |
|------|-------------|---------|-------------|
| `feat` | New user-facing feature | `feat: add password reset email` | New functionality visible to users |
| `fix` | Bug fix in application code | `fix: correct validation logic for email` | Fixing a bug that affects users |
| `chore` | Infrastructure, tooling, dependencies | `chore: upgrade Go to 1.21` | CI/CD, build scripts, dependencies |
| `docs` | Documentation only | `docs: update installation guide` | README, API docs, comments |
| `style` | Code style/formatting (no logic change) | `style: format with prettier` | Linting, formatting, whitespace |
| `refactor` | Code restructuring (no functional change) | `refactor: extract user validation logic` | Improving code without changing behavior |
| `perf` | Performance improvement | `perf: cache database query results` | Optimizations that improve speed/memory |
| `test` | Adding or updating tests | `test: add unit tests for auth module` | Test files or test infrastructure |
| `build` | Build system or external dependencies | `build: update webpack config` | Build tools, package managers |
| `ci` | CI/CD configuration changes | `ci: add code coverage reporting` | GitHub Actions, deployment scripts |
| `revert` | Reverts a previous commit | `revert: revert commit abc123` | Undoing a previous commit |

### Scope (Optional but Recommended)

Add scope in parentheses to specify what part of the codebase changed:

```
feat(auth): add OAuth2 provider support
fix(api): handle null response from external service
docs(readme): add Docker installation instructions
chore(deps): upgrade React to 18.3.0
```

**Common Scopes**:
- Component names: `(button)`, `(modal)`, `(navbar)`
- Module names: `(auth)`, `(api)`, `(database)`
- Feature areas: `(settings)`, `(profile)`, `(checkout)`
- Layer names: `(frontend)`, `(backend)`, `(infrastructure)`

## Quick Guidelines

✅ **DO**:
- Use imperative mood: "Add", "Fix", "Update", "Remove"
- Start with lowercase type: `feat:`, `fix:`, `docs:`
- Be specific: "Fix login redirect" not "Fix bug"
- Reference issues/tickets: `Fixes #123`
- Commit frequently with focused changes
- Write for your future self and team
- Double-check spelling and grammar
- Use conventional commit types

❌ **DON'T**:
- End summary with punctuation (`.`, `!`, `?`)
- Use past tense: "Added", "Fixed", "Updated"
- Use vague messages: "Fix stuff", "Update code", "WIP"
- Capitalize randomly: "Fix Bug in Login"
- Commit everything at once: "Update multiple files"
- Use humor/emojis in professional contexts (unless team standard)
- Write commit messages when tired or rushed

## Examples

### ✅ Excellent Examples

#### Simple Feature
```
feat(auth): add two-factor authentication

Implement TOTP-based 2FA using the speakeasy library.
Users can enable 2FA in account settings.

Closes #234
```

#### Bug Fix with Context
```
fix(api): prevent race condition in user updates

Previously, concurrent updates to user profiles could
result in lost data. Added optimistic locking with
version field to detect conflicts.

The retry logic attempts up to 3 times before failing.

Fixes #567
```

#### Documentation Update
```
docs: add troubleshooting section to README

Include solutions for common installation issues:
- Node version compatibility
- Database connection errors
- Environment variable configuration
```

#### Dependency Update
```
chore(deps): upgrade express from 4.17 to 4.19

Security patch for CVE-2024-12345. No breaking changes
or API modifications required.
```

#### Breaking Change
```
feat(api): redesign user authentication endpoint

BREAKING CHANGE: The /api/login endpoint now returns
a JWT token in the response body instead of a cookie.
Clients must update to include the Authorization header
in subsequent requests.

Migration guide: docs/migration/auth-token.md
Closes #789
```

#### Refactoring
```
refactor(services): extract user service interface

Move user-related business logic from handlers to a
dedicated service layer. No functional changes.

Improves testability and separation of concerns.
```

### ❌ Bad Examples

```
❌ update files
   → Too vague - what was updated and why?

❌ Fixed the login bug.
   → Past tense, period at end, no context

❌ feat: Add new feature for users to be able to...
   → Too long for title, should be in body

❌ WIP
   → Not descriptive, doesn't explain intent

❌ Merge branch 'feature/xyz'
   → Meaningless merge commit (use squash or rebase)

❌ asdfasdf
   → Completely unhelpful

❌ Fixes issue
   → Which issue? No issue number

❌ Updated stuff in the backend
   → Vague, no technical detail
```

## Advanced Guidelines

### Atomic Commits

Each commit should represent one logical change:

✅ **Good**: Three separate commits
```
feat(auth): add login endpoint
feat(auth): add logout endpoint
test(auth): add integration tests for auth endpoints
```

❌ **Bad**: One commit with everything
```
feat: implement authentication system
(Contains login, logout, tests, and unrelated CSS changes)
```

### Commit Frequency

**Commit often to**:
- Keep messages focused and simple
- Make code review easier
- Simplify debugging with `git bisect`
- Reduce risk of lost work

**Good rhythm**:
- After completing a logical unit of work
- Before switching tasks or taking a break
- When tests pass for a feature component

### Issue/Ticket References

Include issue references in the footer:

```
feat(api): add rate limiting middleware

Implement rate limiting using express-rate-limit to
prevent API abuse. Default: 100 requests per 15 minutes.

Closes #345
Refs #346, #347
```

**Keywords for automatic closing**:
- `Closes #123`, `Fixes #123`, `Resolves #123`
- `Closes: #123` (with colon)
- Multiple: `Fixes #123, #124, #125`

### Co-authored Commits

For pair programming or collaborative work:

```
feat(ui): redesign dashboard layout

Co-authored-by: Jane Doe <jane@example.com>
Co-authored-by: John Smith <john@example.com>
```

### Reverting Commits

```
revert: revert "feat(api): add rate limiting"

This reverts commit abc123def456.

Rate limiting caused issues with legitimate high-volume
clients. Will redesign with whitelist support.

Refs #400
```

## Team-Specific Customization

### Define Team Standards

Document your team's commit message conventions:

1. **Type Usage**: Which types your team uses (subset of conventional)
2. **Scope Format**: How to name scopes (kebab-case? camelCase?)
3. **Issue Format**: Jira ticket format vs GitHub issues
4. **Special Markers**: Any team-specific prefixes or tags
5. **Breaking Changes**: How to communicate breaking changes

### Example Team Rules

```markdown
## Team Commit Standards

- Always include scope for domain code
- Use JIRA ticket format: `PROJECT-123`
- Mark breaking changes with [BREAKING] prefix in title
- Include emoji prefix: ✨ feat, 🐛 fix, 📚 docs
- All feat/fix must reference a ticket
```

## Validation and Enforcement

### Pre-commit Hooks

Use tools to enforce commit message standards:

**commitlint** (Recommended)
```bash
npm install --save-dev @commitlint/{cli,config-conventional}
```

**.commitlintrc.json**
```json
{
  "extends": ["@commitlint/config-conventional"],
  "rules": {
    "type-enum": [2, "always", [
      "feat", "fix", "docs", "style", "refactor",
      "perf", "test", "build", "ci", "chore", "revert"
    ]],
    "subject-case": [2, "always", "sentence-case"],
    "subject-max-length": [2, "always", 50],
    "body-max-line-length": [2, "always", 72]
  }
}
```

### Manual Validation Checklist

Before committing, verify:

- [ ] Type is correct and lowercase
- [ ] Subject is imperative mood
- [ ] Subject is 50 characters or less
- [ ] No period at end of subject
- [ ] Body lines wrap at 72 characters
- [ ] Body explains WHAT and WHY, not HOW
- [ ] Issue/ticket referenced if applicable
- [ ] Spelling and grammar checked
- [ ] Breaking changes documented
- [ ] Tests pass

## Tools for Better Commit Messages

### Git Commit Template

Create a commit template to remind you of the format:

**~/.gitmessage**
```
# <type>(<scope>): <subject> (max 50 chars)
# |<----  Using a Maximum Of 50 Characters  ---->|

# Explain why this change is being made
# |<----   Try To Limit Each Line to a Maximum Of 72 Characters   ---->|

# Provide links or keys to any relevant tickets, articles or other resources
# Example: Fixes #23

# --- COMMIT END ---
# Type can be:
#   feat     (new feature)
#   fix      (bug fix)
#   refactor (refactoring production code)
#   style    (formatting, missing semi colons, etc; no code change)
#   docs     (changes to documentation)
#   test     (adding or refactoring tests; no production code change)
#   chore    (updating grunt tasks etc; no production code change)
# --------------------
# Remember to:
#   - Use imperative mood in subject line
#   - Do not end the subject line with a period
#   - Capitalize the subject line
#   - Separate subject from body with a blank line
#   - Use the body to explain what and why vs. how
#   - Can use multiple lines with "-" for bullet points in body
```

**Enable it**:
```bash
git config --global commit.template ~/.gitmessage
```

### IDE Extensions

- **VS Code**: GitLens, Conventional Commits
- **JetBrains**: Git Commit Template
- **Sublime**: Git Commitizen

### Git Aliases for Quick Commits

```bash
# Add to ~/.gitconfig or ~/.git/config
[alias]
  cf = "!f() { git commit -m \"feat: $1\"; }; f"
  cx = "!f() { git commit -m \"fix: $1\"; }; f"
  cd = "!f() { git commit -m \"docs: $1\"; }; f"
  cc = "!f() { git commit -m \"chore: $1\"; }; f"
```

**Usage**:
```bash
git cf "add user authentication"  # Creates: feat: add user authentication
git cx "resolve null pointer in handler"  # Creates: fix: resolve null pointer in handler
```

## Amending and Fixing Commit Messages

### Edit Last Commit Message

```bash
git commit --amend -m "new commit message"
```

### Edit Last Commit Message in Editor

```bash
git commit --amend
```

### Edit Older Commit Messages

```bash
git rebase -i HEAD~3  # Edit last 3 commits
# Change "pick" to "reword" for commits to edit
```

⚠️ **Warning**: Never amend or rebase commits that have been pushed to shared branches!

## Language-Specific Considerations

### Go Projects
```
feat(http): add middleware for request logging
refactor(db): migrate from database/sql to sqlx
fix(parser): handle edge case in JSON unmarshaling
```

### JavaScript/TypeScript Projects
```
feat(components): add error boundary component
fix(hooks): prevent infinite loop in useEffect
chore(deps): upgrade React to 18.3.0
```

### Python Projects
```
feat(api): add FastAPI endpoint for user registration
fix(models): correct SQLAlchemy relationship mapping
test(utils): add unit tests for date parsing
```

## Common Pitfalls and Solutions

| Pitfall | Solution |
|---------|----------|
| Forgetting to commit | Set reminders, commit frequently |
| Vague messages | Include specific details about what changed |
| Too many changes in one commit | Break into atomic commits |
| Past tense usage | Use imperative mood |
| Missing issue references | Always link to tracking system |
| Not explaining "why" | Add body explaining motivation |
| Inconsistent formatting | Use commitlint or pre-commit hooks |

## Changelog Generation

Well-formatted commits enable automatic changelog generation:

**Example Tools**:
- `conventional-changelog`
- `semantic-release`
- `standard-version`

**Generated Changelog**:
```markdown
## [1.2.0] - 2024-01-15

### Features
- **auth**: add two-factor authentication (#234)
- **api**: add rate limiting middleware (#345)

### Bug Fixes
- **api**: prevent race condition in user updates (#567)
- **ui**: correct alignment in mobile view (#590)

### Documentation
- add troubleshooting section to README
- update API documentation with new endpoints
```

## Resources

- [Conventional Commits Specification](https://www.conventionalcommits.org/)
- [Angular Commit Guidelines](https://github.com/angular/angular/blob/master/CONTRIBUTING.md#commit)
- [Semantic Versioning](https://semver.org/)
- [GitKraken Commit Message Guide](https://www.gitkraken.com/learn/git/best-practices/git-commit-message)
- [Git Commit Message Style Guide](https://udacity.github.io/git-styleguide/)
- [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)

## Summary

**The 7 Rules of Great Commit Messages**:

1. Use conventional commit format: `type(scope): subject`
2. Limit subject line to 50 characters
3. Use imperative mood: "Add" not "Added"
4. Don't end subject with punctuation
5. Separate subject from body with blank line
6. Wrap body at 72 characters
7. Explain what and why, not how

**Remember**: A great commit message helps your future self and your team understand the evolution of the codebase. Write commit messages that you'd want to read when debugging at 2 AM! 🕑
