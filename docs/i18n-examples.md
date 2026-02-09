---
title: i18n Implementation Examples
description: Developer guide for implementing internationalization in Charon React components using react-i18next.
---

## i18n Implementation Examples

This document shows examples of how to use translations in Charon components.

### Basic Usage

### Using the `useTranslation` Hook

```typescript
import { useTranslation } from 'react-i18next'

function MyComponent() {
  const { t } = useTranslation()

  return (
    <div>
      <h1>{t('navigation.dashboard')}</h1>
      <button>{t('common.save')}</button>
      <button>{t('common.cancel')}</button>
    </div>
  )
}
```

### With Interpolation

```typescript
import { useTranslation } from 'react-i18next'

function ProxyHostsCount({ count }: { count: number }) {
  const { t } = useTranslation()

  return <p>{t('dashboard.activeHosts', { count })}</p>
  // Renders: "5 active" (English) or "5 activo" (Spanish)
}
```

## Common Patterns

### Page Titles and Descriptions

```typescript
import { useTranslation } from 'react-i18next'
import { PageShell } from '../components/layout/PageShell'

export default function Dashboard() {
  const { t } = useTranslation()

  return (
    <PageShell
      title={t('dashboard.title')}
      description={t('dashboard.description')}
    >
      {/* Page content */}
    </PageShell>
  )
}
```

### Button Labels

```typescript
import { useTranslation } from 'react-i18next'
import { Button } from '../components/ui/Button'

function SaveButton() {
  const { t } = useTranslation()

  return (
    <Button onClick={handleSave}>
      {t('common.save')}
    </Button>
  )
}
```

### Form Labels

```typescript
import { useTranslation } from 'react-i18next'
import { Label } from '../components/ui/Label'
import { Input } from '../components/ui/Input'

function EmailField() {
  const { t } = useTranslation()

  return (
    <div>
      <Label htmlFor="email">{t('auth.email')}</Label>
      <Input
        id="email"
        type="email"
        placeholder={t('auth.email')}
      />
    </div>
  )
}
```

### Error Messages

```typescript
import { useTranslation } from 'react-i18next'

function validateForm(data: FormData) {
  const { t } = useTranslation()
  const errors: Record<string, string> = {}

  if (!data.email) {
    errors.email = t('errors.required')
  } else if (!isValidEmail(data.email)) {
    errors.email = t('errors.invalidEmail')
  }

  if (!data.password || data.password.length < 8) {
    errors.password = t('errors.passwordTooShort')
  }

  return errors
}
```

### Toast Notifications

```typescript
import { useTranslation } from 'react-i18next'
import { toast } from '../utils/toast'

function handleSave() {
  const { t } = useTranslation()

  try {
    await saveData()
    toast.success(t('notifications.saveSuccess'))
  } catch (error) {
    toast.error(t('notifications.saveFailed'))
  }
}
```

### Navigation Menu

```typescript
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

function Navigation() {
  const { t } = useTranslation()

  const navItems = [
    { path: '/', label: t('navigation.dashboard') },
    { path: '/proxy-hosts', label: t('navigation.proxyHosts') },
    { path: '/certificates', label: t('navigation.certificates') },
    { path: '/settings', label: t('navigation.settings') },
  ]

  return (
    <nav>
      {navItems.map(item => (
        <Link key={item.path} to={item.path}>
          {item.label}
        </Link>
      ))}
    </nav>
  )
}
```

## Advanced Patterns

### Pluralization

```typescript
import { useTranslation } from 'react-i18next'

function ItemCount({ count }: { count: number }) {
  const { t } = useTranslation()

  // Translation file should have:
  // "items": "{{count}} item",
  // "items_other": "{{count}} items"

  return <p>{t('items', { count })}</p>
}
```

### Dynamic Keys

```typescript
import { useTranslation } from 'react-i18next'

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()

  // Dynamically build the translation key
  return <span>{t(`certificates.${status}`)}</span>
  // Translates to: "Valid", "Pending", "Expired", etc.
}
```

### Context-Specific Translations

```typescript
import { useTranslation } from 'react-i18next'

function DeleteConfirmation({ itemType }: { itemType: 'host' | 'certificate' }) {
  const { t } = useTranslation()

  return (
    <div>
      <p>{t(`${itemType}.deleteConfirmation`)}</p>
      <Button variant="danger">{t('common.delete')}</Button>
      <Button variant="outline">{t('common.cancel')}</Button>
    </div>
  )
}
```

## Testing Components with i18n

When testing components that use i18n, mock the `useTranslation` hook:

```typescript
import { vi } from 'vitest'
import { render } from '@testing-library/react'

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key, // Return the key as-is for testing
    i18n: {
      changeLanguage: vi.fn(),
      language: 'en',
    },
  }),
}))

describe('MyComponent', () => {
  it('renders translated content', () => {
    const { getByText } = render(<MyComponent />)
    expect(getByText('navigation.dashboard')).toBeInTheDocument()
  })
})
```

## Best Practices

1. **Always use translation keys** - Never hardcode strings in components
2. **Use descriptive keys** - Keys should indicate what the text is for
3. **Group related translations** - Use namespaces (common, navigation, etc.)
4. **Keep translations short** - Long strings may not fit in the UI
5. **Test all languages** - Verify translations work in different languages
6. **Provide context** - Use comments in translation files to explain usage

## Migration Checklist

When converting an existing component to use i18n:

- [ ] Import `useTranslation` hook
- [ ] Add `const { t } = useTranslation()` at component top
- [ ] Replace all hardcoded strings with `t('key')`
- [ ] Add missing translation keys to all language files
- [ ] Test the component in different languages
- [ ] Update component tests to mock i18n
