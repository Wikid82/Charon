# Contributing to CaddyProxyManager+

Thank you for your interest in contributing to CaddyProxyManager+! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)
- [Documentation](#documentation)

## Code of Conduct

This project follows a Code of Conduct that all contributors are expected to adhere to:

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on what's best for the community
- Show empathy towards other community members

## Getting Started

### Prerequisites

- **Go 1.22+** for backend development
- **Node.js 20+** and npm for frontend development
- Git for version control
- A GitHub account

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
```bash
git clone https://github.com/YOUR_USERNAME/CaddyProxyManagerPlus.git
cd CaddyProxyManagerPlus
```

3. Add the upstream remote:
```bash
git remote add upstream https://github.com/Wikid82/CaddyProxyManagerPlus.git
```

### Set Up Development Environment

**Backend:**
```bash
cd backend
go mod download
go run ./cmd/seed/main.go  # Seed test data
go run ./cmd/api/main.go   # Start backend
```

**Frontend:**
```bash
cd frontend
npm install
npm run dev  # Start frontend dev server
```

## Development Workflow

### Branching Strategy

- **main** - Production-ready code
- **development** - Main development branch (default)
- **feature/** - Feature branches (e.g., `feature/add-ssl-support`)
- **bugfix/** - Bug fix branches (e.g., `bugfix/fix-import-crash`)
- **hotfix/** - Urgent production fixes

### Creating a Feature Branch

Always branch from `development`:

```bash
git checkout development
git pull upstream development
git checkout -b feature/your-feature-name
```

### Commit Message Guidelines

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(proxy-hosts): add SSL certificate upload

- Implement certificate upload endpoint
- Add UI for certificate management
- Update database schema

Closes #123
```

```
fix(import): resolve conflict detection bug

When importing Caddyfiles with multiple domains, conflicts
were not being detected properly.

Fixes #456
```

### Keeping Your Fork Updated

```bash
git checkout development
git fetch upstream
git merge upstream/development
git push origin development
```

## Coding Standards

### Go Backend

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Write godoc comments for exported functions
- Keep functions small and focused
- Handle errors explicitly

**Example:**
```go
// GetProxyHost retrieves a proxy host by UUID.
// Returns an error if the host is not found.
func GetProxyHost(uuid string) (*models.ProxyHost, error) {
    var host models.ProxyHost
    if err := db.First(&host, "uuid = ?", uuid).Error; err != nil {
        return nil, fmt.Errorf("proxy host not found: %w", err)
    }
    return &host, nil
}
```

### TypeScript Frontend

- Use TypeScript for type safety
- Follow React best practices and hooks patterns
- Use functional components
- Destructure props at function signature
- Extract reusable logic into custom hooks

**Example:**
```typescript
interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: ProxyHostData) => Promise<void>
  onCancel: () => void
}

export function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  const [domain, setDomain] = useState(host?.domain ?? '')
  // ... component logic
}
```

### CSS/Styling

- Use TailwindCSS utility classes
- Follow the dark theme color palette
- Keep custom CSS minimal
- Use semantic color names from the theme

## Testing Guidelines

### Backend Tests

Write tests for all new functionality:

```go
func TestGetProxyHost(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    host := createTestHost(db)

    // Execute
    result, err := GetProxyHost(host.UUID)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, host.Domain, result.Domain)
}
```

**Run tests:**
```bash
go test ./... -v
go test -cover ./...
```

### Frontend Tests

Write component and hook tests using Vitest and React Testing Library:

```typescript
describe('ProxyHostForm', () => {
  it('renders create form with empty fields', async () => {
    render(
      <ProxyHostForm onSubmit={vi.fn()} onCancel={vi.fn()} />
    )

    await waitFor(() => {
      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })
  })
})
```

**Run tests:**
```bash
npm test              # Watch mode
npm run test:coverage # Coverage report
```

### Test Coverage

- Aim for 80%+ code coverage
- All new features must include tests
- Bug fixes should include regression tests

## Pull Request Process

### Before Submitting

1. **Ensure tests pass:**
```bash
# Backend
go test ./...

# Frontend
npm test -- --run
```

2. **Check code quality:**
```bash
# Go formatting
go fmt ./...

# Frontend linting
npm run lint
```

3. **Update documentation** if needed
4. **Add tests** for new functionality
5. **Rebase on latest development** branch

### Submitting a Pull Request

1. Push your branch to your fork:
```bash
git push origin feature/your-feature-name
```

2. Open a Pull Request on GitHub
3. Fill out the PR template completely
4. Link related issues using "Closes #123" or "Fixes #456"
5. Request review from maintainers

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Manual testing performed
- [ ] All tests passing

## Screenshots (if applicable)
Add screenshots of UI changes

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review performed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No new warnings generated
```

### Review Process

- Maintainers will review within 2-3 business days
- Address review feedback promptly
- Keep discussions focused and professional
- Be open to suggestions and alternative approaches

## Issue Guidelines

### Reporting Bugs

Use the bug report template and include:

- Clear, descriptive title
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, browser, Go version, etc.)
- Screenshots or error logs
- Potential solutions (if known)

### Feature Requests

Use the feature request template and include:

- Clear description of the feature
- Use case and motivation
- Potential implementation approach
- Mockups or examples (if applicable)

### Issue Labels

- `bug` - Something isn't working
- `enhancement` - New feature or request
- `documentation` - Documentation improvements
- `good first issue` - Good for newcomers
- `help wanted` - Extra attention needed
- `priority: high` - Urgent issue
- `wontfix` - Will not be fixed

## Documentation

### Code Documentation

- Add docstrings to all exported functions
- Include examples in complex functions
- Document return types and error conditions
- Keep comments up-to-date with code changes

### Project Documentation

When adding features, update:

- `README.md` - User-facing information
- `docs/api.md` - API changes
- `docs/import-guide.md` - Import feature updates
- `docs/database-schema.md` - Schema changes

## Recognition

Contributors will be recognized in:

- CONTRIBUTORS.md file
- Release notes for significant contributions
- GitHub contributors page

## Questions?

- Open a [Discussion](https://github.com/Wikid82/CaddyProxyManagerPlus/discussions) for general questions
- Join our community chat (coming soon)
- Tag maintainers in issues for urgent matters

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

---

Thank you for contributing to CaddyProxyManager+! 🎉
