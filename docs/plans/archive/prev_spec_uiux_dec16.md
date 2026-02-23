# Charon UI/UX Improvement Plan

**Issue:** GitHub #409 - UI Enhancement & Design System
**Date:** December 16, 2025
**Status:** ✅ Completed
**Completion Date:** December 16, 2025
**Stack:** React 19 + Vite + TypeScript + TanStack Query + Tailwind CSS v4

---

## Executive Summary

The current Charon UI is functional but lacks design consistency, visual polish, and systematic component architecture. This plan addresses Issue #409's recommendations to transform the interface from "bland" to professional-grade through:

1. **Design Token System** - Consistent colors, spacing, typography
2. **Component Library** - Reusable, accessible UI primitives
3. **Layout Improvements** - Better dashboards, tables, empty states
4. **Page Polish** - Systematic improvement of all pages

---

## 1. Current State Analysis

### 1.1 Tailwind Configuration (tailwind.config.js)

**Current:**

```javascript
colors: {
  'light-bg': '#f0f4f8',
  'dark-bg': '#0f172a',
  'dark-sidebar': '#020617',
  'dark-card': '#1e293b',
  'blue-active': '#1d4ed8',
  'blue-hover': '#2563eb',
}
```

**Problems:**

- ❌ Only 6 ad-hoc color tokens
- ❌ No semantic naming (surface, border, text layers)
- ❌ No state colors (success, warning, error, info)
- ❌ No brand color scale
- ❌ No spacing scale beyond Tailwind defaults
- ❌ No typography configuration

### 1.2 CSS Variables (index.css)

**Current:**

```css
:root {
  font-family: Inter, system-ui, Avenir, Helvetica, Arial, sans-serif;
  color: rgba(255, 255, 255, 0.87);
  background-color: #0f172a;
}
```

**Problems:**

- ❌ Hardcoded colors, not CSS variables
- ❌ No dark/light mode toggle system
- ❌ No type scale
- ❌ Custom animations exist but no transition standards

### 1.3 Existing Component Library (frontend/src/components/ui/)

| Component | Status | Issues |
|-----------|--------|--------|
| `Button.tsx` | ✅ Good foundation | Missing outline variant, icon support |
| `Card.tsx` | ✅ Good foundation | Missing hover states, compact variant |
| `Input.tsx` | ✅ Good foundation | No textarea, select variants |
| `Switch.tsx` | ⚠️ Functional | Hard-coded colors, no size variants |

**Missing Components:**

- Badge/Tag
- Alert/Callout
- Dialog/Modal (exists ad-hoc in pages)
- Dropdown/Select
- Tabs
- Tooltip
- Table (data table with sorting)
- Skeleton loaders
- Progress indicators

### 1.4 Page-Level UI Patterns

| Page | Patterns | Issues |
|------|----------|--------|
| Dashboard | KPI cards, links | Cards lack visual hierarchy, no trend indicators |
| ProxyHosts | Data table, modals | Inline modals, inconsistent styling, no sticky headers |
| Security | Layer cards, toggles | Good theming, but cards cramped |
| Settings | Tab navigation, forms | Basic tabs, form styling inconsistent |
| AccessLists | Table with selection | Good patterns, inline confirm dialogs |

### 1.5 Inconsistencies Found

1. **Modal Patterns**: Some use `fixed inset-0`, some use custom positioning
2. **Button Styling**: Mix of `bg-blue-active` and `bg-blue-600`
3. **Card Borders**: Some use `border-gray-800`, others `border-gray-700`
4. **Text Colors**: Inconsistent use of gray scale (gray-400/500 for secondary)
5. **Spacing**: No consistent page gutters or section spacing
6. **Focus States**: `focus:ring-2` used but not consistently
7. **Loading States**: Custom Charon/Cerberus loaders exist but not used everywhere

---

## 2. Design Token System

### 2.1 CSS Variables (index.css)

```css
@layer base {
  :root {
    /* ========================================
     * BRAND COLORS
     * ======================================== */
    --color-brand-50: 239 246 255;   /* #eff6ff */
    --color-brand-100: 219 234 254;  /* #dbeafe */
    --color-brand-200: 191 219 254;  /* #bfdbfe */
    --color-brand-300: 147 197 253;  /* #93c5fd */
    --color-brand-400: 96 165 250;   /* #60a5fa */
    --color-brand-500: 59 130 246;   /* #3b82f6 - Primary */
    --color-brand-600: 37 99 235;    /* #2563eb */
    --color-brand-700: 29 78 216;    /* #1d4ed8 */
    --color-brand-800: 30 64 175;    /* #1e40af */
    --color-brand-900: 30 58 138;    /* #1e3a8a */
    --color-brand-950: 23 37 84;     /* #172554 */

    /* ========================================
     * SEMANTIC COLORS - Light Mode
     * ======================================== */
    /* Surfaces */
    --color-bg-base: 248 250 252;        /* slate-50 */
    --color-bg-subtle: 241 245 249;      /* slate-100 */
    --color-bg-muted: 226 232 240;       /* slate-200 */
    --color-bg-elevated: 255 255 255;    /* white */
    --color-bg-overlay: 15 23 42;        /* slate-900 */

    /* Borders */
    --color-border-default: 226 232 240; /* slate-200 */
    --color-border-muted: 241 245 249;   /* slate-100 */
    --color-border-strong: 203 213 225;  /* slate-300 */

    /* Text */
    --color-text-primary: 15 23 42;      /* slate-900 */
    --color-text-secondary: 71 85 105;   /* slate-600 */
    --color-text-muted: 148 163 184;     /* slate-400 */
    --color-text-inverted: 255 255 255;  /* white */

    /* States */
    --color-success: 34 197 94;          /* green-500 */
    --color-success-muted: 220 252 231;  /* green-100 */
    --color-warning: 234 179 8;          /* yellow-500 */
    --color-warning-muted: 254 249 195;  /* yellow-100 */
    --color-error: 239 68 68;            /* red-500 */
    --color-error-muted: 254 226 226;    /* red-100 */
    --color-info: 59 130 246;            /* blue-500 */
    --color-info-muted: 219 234 254;     /* blue-100 */

    /* ========================================
     * TYPOGRAPHY
     * ======================================== */
    --font-sans: 'Inter', system-ui, -apple-system, sans-serif;
    --font-mono: 'JetBrains Mono', 'Fira Code', monospace;

    /* Type Scale (rem) */
    --text-xs: 0.75rem;     /* 12px */
    --text-sm: 0.875rem;    /* 14px */
    --text-base: 1rem;      /* 16px */
    --text-lg: 1.125rem;    /* 18px */
    --text-xl: 1.25rem;     /* 20px */
    --text-2xl: 1.5rem;     /* 24px */
    --text-3xl: 1.875rem;   /* 30px */
    --text-4xl: 2.25rem;    /* 36px */

    /* Line Heights */
    --leading-tight: 1.25;
    --leading-normal: 1.5;
    --leading-relaxed: 1.75;

    /* Font Weights */
    --font-normal: 400;
    --font-medium: 500;
    --font-semibold: 600;
    --font-bold: 700;

    /* ========================================
     * SPACING & LAYOUT
     * ======================================== */
    --space-0: 0;
    --space-1: 0.25rem;   /* 4px */
    --space-2: 0.5rem;    /* 8px */
    --space-3: 0.75rem;   /* 12px */
    --space-4: 1rem;      /* 16px */
    --space-5: 1.25rem;   /* 20px */
    --space-6: 1.5rem;    /* 24px */
    --space-8: 2rem;      /* 32px */
    --space-10: 2.5rem;   /* 40px */
    --space-12: 3rem;     /* 48px */
    --space-16: 4rem;     /* 64px */

    /* Container */
    --container-sm: 640px;
    --container-md: 768px;
    --container-lg: 1024px;
    --container-xl: 1280px;
    --container-2xl: 1536px;

    /* Page Gutters */
    --page-gutter: var(--space-6);
    --page-gutter-lg: var(--space-8);

    /* ========================================
     * EFFECTS
     * ======================================== */
    /* Border Radius */
    --radius-sm: 0.25rem;   /* 4px */
    --radius-md: 0.375rem;  /* 6px */
    --radius-lg: 0.5rem;    /* 8px */
    --radius-xl: 0.75rem;   /* 12px */
    --radius-2xl: 1rem;     /* 16px */
    --radius-full: 9999px;

    /* Shadows */
    --shadow-sm: 0 1px 2px 0 rgb(0 0 0 / 0.05);
    --shadow-md: 0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1);
    --shadow-lg: 0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1);
    --shadow-xl: 0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1);

    /* Transitions */
    --transition-fast: 150ms;
    --transition-normal: 200ms;
    --transition-slow: 300ms;
    --ease-default: cubic-bezier(0.4, 0, 0.2, 1);
    --ease-in: cubic-bezier(0.4, 0, 1, 1);
    --ease-out: cubic-bezier(0, 0, 0.2, 1);

    /* Focus Ring */
    --ring-width: 2px;
    --ring-offset: 2px;
    --ring-color: var(--color-brand-500);
  }

  /* ========================================
   * DARK MODE OVERRIDES
   * ======================================== */
  .dark {
    /* Surfaces */
    --color-bg-base: 15 23 42;           /* slate-900 */
    --color-bg-subtle: 30 41 59;         /* slate-800 */
    --color-bg-muted: 51 65 85;          /* slate-700 */
    --color-bg-elevated: 30 41 59;       /* slate-800 */
    --color-bg-overlay: 2 6 23;          /* slate-950 */

    /* Borders */
    --color-border-default: 51 65 85;    /* slate-700 */
    --color-border-muted: 30 41 59;      /* slate-800 */
    --color-border-strong: 71 85 105;    /* slate-600 */

    /* Text */
    --color-text-primary: 248 250 252;   /* slate-50 */
    --color-text-secondary: 203 213 225; /* slate-300 */
    --color-text-muted: 148 163 184;     /* slate-400 */
    --color-text-inverted: 15 23 42;     /* slate-900 */

    /* States - Muted versions for dark mode */
    --color-success-muted: 20 83 45;     /* green-900 */
    --color-warning-muted: 113 63 18;    /* yellow-900 */
    --color-error-muted: 127 29 29;      /* red-900 */
    --color-info-muted: 30 58 138;       /* blue-900 */
  }
}
```

### 2.2 Tailwind Configuration (tailwind.config.js)

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Brand
        brand: {
          50: 'rgb(var(--color-brand-50) / <alpha-value>)',
          100: 'rgb(var(--color-brand-100) / <alpha-value>)',
          200: 'rgb(var(--color-brand-200) / <alpha-value>)',
          300: 'rgb(var(--color-brand-300) / <alpha-value>)',
          400: 'rgb(var(--color-brand-400) / <alpha-value>)',
          500: 'rgb(var(--color-brand-500) / <alpha-value>)',
          600: 'rgb(var(--color-brand-600) / <alpha-value>)',
          700: 'rgb(var(--color-brand-700) / <alpha-value>)',
          800: 'rgb(var(--color-brand-800) / <alpha-value>)',
          900: 'rgb(var(--color-brand-900) / <alpha-value>)',
          950: 'rgb(var(--color-brand-950) / <alpha-value>)',
        },
        // Semantic Surfaces
        surface: {
          base: 'rgb(var(--color-bg-base) / <alpha-value>)',
          subtle: 'rgb(var(--color-bg-subtle) / <alpha-value>)',
          muted: 'rgb(var(--color-bg-muted) / <alpha-value>)',
          elevated: 'rgb(var(--color-bg-elevated) / <alpha-value>)',
          overlay: 'rgb(var(--color-bg-overlay) / <alpha-value>)',
        },
        // Semantic Borders
        border: {
          DEFAULT: 'rgb(var(--color-border-default) / <alpha-value>)',
          muted: 'rgb(var(--color-border-muted) / <alpha-value>)',
          strong: 'rgb(var(--color-border-strong) / <alpha-value>)',
        },
        // Semantic Text
        content: {
          primary: 'rgb(var(--color-text-primary) / <alpha-value>)',
          secondary: 'rgb(var(--color-text-secondary) / <alpha-value>)',
          muted: 'rgb(var(--color-text-muted) / <alpha-value>)',
          inverted: 'rgb(var(--color-text-inverted) / <alpha-value>)',
        },
        // Status Colors
        success: {
          DEFAULT: 'rgb(var(--color-success) / <alpha-value>)',
          muted: 'rgb(var(--color-success-muted) / <alpha-value>)',
        },
        warning: {
          DEFAULT: 'rgb(var(--color-warning) / <alpha-value>)',
          muted: 'rgb(var(--color-warning-muted) / <alpha-value>)',
        },
        error: {
          DEFAULT: 'rgb(var(--color-error) / <alpha-value>)',
          muted: 'rgb(var(--color-error-muted) / <alpha-value>)',
        },
        info: {
          DEFAULT: 'rgb(var(--color-info) / <alpha-value>)',
          muted: 'rgb(var(--color-info-muted) / <alpha-value>)',
        },
        // Legacy support (deprecate over time)
        'dark-bg': '#0f172a',
        'dark-sidebar': '#020617',
        'dark-card': '#1e293b',
        'blue-active': '#1d4ed8',
        'blue-hover': '#2563eb',
      },
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
      },
      fontSize: {
        xs: ['var(--text-xs)', { lineHeight: 'var(--leading-normal)' }],
        sm: ['var(--text-sm)', { lineHeight: 'var(--leading-normal)' }],
        base: ['var(--text-base)', { lineHeight: 'var(--leading-normal)' }],
        lg: ['var(--text-lg)', { lineHeight: 'var(--leading-normal)' }],
        xl: ['var(--text-xl)', { lineHeight: 'var(--leading-tight)' }],
        '2xl': ['var(--text-2xl)', { lineHeight: 'var(--leading-tight)' }],
        '3xl': ['var(--text-3xl)', { lineHeight: 'var(--leading-tight)' }],
        '4xl': ['var(--text-4xl)', { lineHeight: 'var(--leading-tight)' }],
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        DEFAULT: 'var(--radius-md)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        '2xl': 'var(--radius-2xl)',
      },
      boxShadow: {
        sm: 'var(--shadow-sm)',
        DEFAULT: 'var(--shadow-md)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        xl: 'var(--shadow-xl)',
      },
      transitionDuration: {
        fast: 'var(--transition-fast)',
        normal: 'var(--transition-normal)',
        slow: 'var(--transition-slow)',
      },
      spacing: {
        'page': 'var(--page-gutter)',
        'page-lg': 'var(--page-gutter-lg)',
      },
    },
  },
  plugins: [],
}
```

---

## 3. Component Library Specifications

### 3.1 Directory Structure

```
frontend/src/components/ui/
├── index.ts                 # Barrel exports
├── Button.tsx              # ✅ Exists - enhance
├── Card.tsx                # ✅ Exists - enhance
├── Input.tsx               # ✅ Exists - enhance
├── Switch.tsx              # ✅ Exists - enhance
├── Badge.tsx               # 🆕 New
├── Alert.tsx               # 🆕 New
├── Dialog.tsx              # 🆕 New
├── Select.tsx              # 🆕 New
├── Tabs.tsx                # 🆕 New
├── Tooltip.tsx             # 🆕 New
├── DataTable.tsx           # 🆕 New
├── Skeleton.tsx            # 🆕 New
├── Progress.tsx            # 🆕 New
├── Checkbox.tsx            # 🆕 New
├── Label.tsx               # 🆕 New
├── Textarea.tsx            # 🆕 New
└── __tests__/              # Component tests
```

### 3.2 Component Specifications

#### Badge Component

```tsx
// frontend/src/components/ui/Badge.tsx
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../utils/cn'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-surface-muted text-content-primary',
        primary: 'bg-brand-500/10 text-brand-500',
        success: 'bg-success-muted text-success',
        warning: 'bg-warning-muted text-warning',
        error: 'bg-error-muted text-error',
        outline: 'border border-border text-content-secondary',
      },
      size: {
        sm: 'px-2 py-0.5 text-xs',
        md: 'px-2.5 py-0.5 text-xs',
        lg: 'px-3 py-1 text-sm',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  }
)

interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {
  icon?: React.ReactNode
}

export function Badge({ className, variant, size, icon, children, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ variant, size }), className)} {...props}>
      {icon && <span className="mr-1">{icon}</span>}
      {children}
    </span>
  )
}
```

#### Alert Component

```tsx
// frontend/src/components/ui/Alert.tsx
import { cva, type VariantProps } from 'class-variance-authority'
import { AlertCircle, CheckCircle, Info, AlertTriangle, X } from 'lucide-react'
import { cn } from '../../utils/cn'

const alertVariants = cva(
  'relative w-full rounded-lg border p-4 [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4 [&>svg+div]:translate-y-[-3px] [&:has(svg)]:pl-11',
  {
    variants: {
      variant: {
        default: 'bg-surface-subtle border-border text-content-primary',
        info: 'bg-info-muted border-info/20 text-info [&>svg]:text-info',
        success: 'bg-success-muted border-success/20 text-success [&>svg]:text-success',
        warning: 'bg-warning-muted border-warning/20 text-warning [&>svg]:text-warning',
        error: 'bg-error-muted border-error/20 text-error [&>svg]:text-error',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

const iconMap = {
  default: Info,
  info: Info,
  success: CheckCircle,
  warning: AlertTriangle,
  error: AlertCircle,
}

interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {
  title?: string
  onDismiss?: () => void
}

export function Alert({
  className,
  variant = 'default',
  title,
  children,
  onDismiss,
  ...props
}: AlertProps) {
  const Icon = iconMap[variant || 'default']

  return (
    <div role="alert" className={cn(alertVariants({ variant }), className)} {...props}>
      <Icon className="h-4 w-4" />
      <div className="flex-1">
        {title && <h5 className="mb-1 font-medium leading-none tracking-tight">{title}</h5>}
        <div className="text-sm [&_p]:leading-relaxed">{children}</div>
      </div>
      {onDismiss && (
        <button
          onClick={onDismiss}
          className="absolute right-2 top-2 rounded-md p-1 opacity-70 hover:opacity-100 transition-opacity"
        >
          <X className="h-4 w-4" />
        </button>
      )}
    </div>
  )
}
```

#### Dialog Component

```tsx
// frontend/src/components/ui/Dialog.tsx
import { Fragment, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { cn } from '../../utils/cn'

interface DialogProps {
  open: boolean
  onClose: () => void
  children: ReactNode
  className?: string
}

export function Dialog({ open, onClose, children, className }: DialogProps) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-surface-overlay/80 backdrop-blur-sm transition-opacity"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Dialog container */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div
          className={cn(
            'relative w-full max-w-lg transform overflow-hidden rounded-xl',
            'bg-surface-elevated border border-border shadow-xl',
            'transition-all duration-normal',
            'animate-in fade-in-0 zoom-in-95',
            className
          )}
          role="dialog"
          aria-modal="true"
        >
          {children}
        </div>
      </div>
    </div>
  )
}

export function DialogHeader({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('flex items-center justify-between p-6 border-b border-border', className)}>
      {children}
    </div>
  )
}

export function DialogTitle({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <h2 className={cn('text-lg font-semibold text-content-primary', className)}>
      {children}
    </h2>
  )
}

export function DialogClose({ onClose }: { onClose: () => void }) {
  return (
    <button
      onClick={onClose}
      className="rounded-md p-1 text-content-muted hover:text-content-primary hover:bg-surface-muted transition-colors"
    >
      <X className="h-5 w-5" />
    </button>
  )
}

export function DialogContent({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('p-6', className)}>{children}</div>
}

export function DialogFooter({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('flex items-center justify-end gap-3 p-6 border-t border-border bg-surface-subtle', className)}>
      {children}
    </div>
  )
}
```

#### Enhanced Button Component

```tsx
// frontend/src/components/ui/Button.tsx (enhanced)
import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { Loader2 } from 'lucide-react'
import { cn } from '../../utils/cn'

const buttonVariants = cva(
  [
    'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg',
    'text-sm font-medium transition-all duration-fast',
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
    'disabled:pointer-events-none disabled:opacity-50',
    'active:scale-[0.98]',
  ],
  {
    variants: {
      variant: {
        primary: 'bg-brand-600 text-white hover:bg-brand-700 shadow-sm',
        secondary: 'bg-surface-muted text-content-primary hover:bg-surface-subtle border border-border',
        danger: 'bg-error text-white hover:bg-red-600 shadow-sm',
        ghost: 'text-content-secondary hover:text-content-primary hover:bg-surface-muted',
        outline: 'border border-border text-content-primary hover:bg-surface-muted',
        link: 'text-brand-500 hover:text-brand-600 underline-offset-4 hover:underline',
      },
      size: {
        sm: 'h-8 px-3 text-xs',
        md: 'h-10 px-4 text-sm',
        lg: 'h-12 px-6 text-base',
        icon: 'h-10 w-10',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'md',
    },
  }
)

interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  isLoading?: boolean
  leftIcon?: ReactNode
  rightIcon?: ReactNode
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, isLoading, leftIcon, rightIcon, children, disabled, ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(buttonVariants({ variant, size }), className)}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : leftIcon ? (
          <span className="shrink-0">{leftIcon}</span>
        ) : null}
        {children}
        {rightIcon && !isLoading && <span className="shrink-0">{rightIcon}</span>}
      </button>
    )
  }
)

Button.displayName = 'Button'
```

#### Skeleton Component

```tsx
// frontend/src/components/ui/Skeleton.tsx
import { cn } from '../../utils/cn'

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'circular' | 'text'
}

export function Skeleton({ className, variant = 'default', ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        'animate-pulse bg-surface-muted',
        {
          'rounded-md': variant === 'default',
          'rounded-full': variant === 'circular',
          'rounded h-4 w-full': variant === 'text',
        },
        className
      )}
      {...props}
    />
  )
}

// Pre-built skeleton patterns
export function SkeletonCard() {
  return (
    <div className="rounded-lg border border-border p-6 space-y-4">
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="h-8 w-1/2" />
      <div className="space-y-2">
        <Skeleton variant="text" />
        <Skeleton variant="text" className="w-5/6" />
      </div>
    </div>
  )
}

export function SkeletonTable({ rows = 5 }: { rows?: number }) {
  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <div className="border-b border-border p-4 bg-surface-subtle">
        <div className="flex gap-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-4 flex-1" />
          ))}
        </div>
      </div>
      <div className="divide-y divide-border">
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="p-4 flex gap-4">
            {[1, 2, 3, 4].map((j) => (
              <Skeleton key={j} className="h-4 flex-1" />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
```

### 3.3 Dependencies to Add

```json
{
  "dependencies": {
    "class-variance-authority": "^0.7.0",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-tooltip": "^1.0.7",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-checkbox": "^1.0.4",
    "@radix-ui/react-progress": "^1.0.3"
  }
}
```

---

## 4. Layout Improvements

### 4.1 Page Shell Component

```tsx
// frontend/src/components/layout/PageShell.tsx
import { type ReactNode } from 'react'
import { cn } from '../../utils/cn'

interface PageShellProps {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}

export function PageShell({ title, description, actions, children, className }: PageShellProps) {
  return (
    <div className={cn('space-y-6', className)}>
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-content-primary">{title}</h1>
          {description && (
            <p className="mt-1 text-sm text-content-secondary">{description}</p>
          )}
        </div>
        {actions && <div className="flex items-center gap-3">{actions}</div>}
      </header>
      {children}
    </div>
  )
}
```

### 4.2 Stats Card Component

```tsx
// frontend/src/components/ui/StatsCard.tsx
import { type ReactNode } from 'react'
import { cn } from '../../utils/cn'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'

interface StatsCardProps {
  title: string
  value: string | number
  change?: {
    value: number
    trend: 'up' | 'down' | 'neutral'
    label?: string
  }
  icon?: ReactNode
  href?: string
  className?: string
}

export function StatsCard({ title, value, change, icon, href, className }: StatsCardProps) {
  const Wrapper = href ? 'a' : 'div'
  const wrapperProps = href ? { href } : {}

  const TrendIcon = change?.trend === 'up' ? TrendingUp : change?.trend === 'down' ? TrendingDown : Minus
  const trendColor = change?.trend === 'up' ? 'text-success' : change?.trend === 'down' ? 'text-error' : 'text-content-muted'

  return (
    <Wrapper
      {...wrapperProps}
      className={cn(
        'block rounded-xl border border-border bg-surface-elevated p-6',
        'transition-all duration-fast',
        href && 'hover:shadow-md hover:border-brand-500/50 cursor-pointer',
        className
      )}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-medium text-content-secondary">{title}</p>
          <p className="mt-2 text-3xl font-bold text-content-primary">{value}</p>
          {change && (
            <div className={cn('mt-2 flex items-center gap-1 text-sm', trendColor)}>
              <TrendIcon className="h-4 w-4" />
              <span>{change.value}%</span>
              {change.label && <span className="text-content-muted">{change.label}</span>}
            </div>
          )}
        </div>
        {icon && (
          <div className="rounded-lg bg-brand-500/10 p-3 text-brand-500">
            {icon}
          </div>
        )}
      </div>
    </Wrapper>
  )
}
```

### 4.3 Empty State Component (Enhanced)

```tsx
// frontend/src/components/ui/EmptyState.tsx
import { type ReactNode } from 'react'
import { cn } from '../../utils/cn'
import { Button } from './Button'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description: string
  action?: {
    label: string
    onClick: () => void
    variant?: 'primary' | 'secondary'
  }
  secondaryAction?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  secondaryAction,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center py-16 px-6 text-center',
        'rounded-xl border border-dashed border-border bg-surface-subtle/50',
        className
      )}
    >
      {icon && (
        <div className="mb-4 rounded-full bg-surface-muted p-4 text-content-muted">
          {icon}
        </div>
      )}
      <h3 className="text-lg font-semibold text-content-primary">{title}</h3>
      <p className="mt-2 max-w-sm text-sm text-content-secondary">{description}</p>
      {(action || secondaryAction) && (
        <div className="mt-6 flex items-center gap-3">
          {action && (
            <Button variant={action.variant || 'primary'} onClick={action.onClick}>
              {action.label}
            </Button>
          )}
          {secondaryAction && (
            <Button variant="ghost" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
```

### 4.4 Data Table Component

```tsx
// frontend/src/components/ui/DataTable.tsx
import { type ReactNode, useState } from 'react'
import { cn } from '../../utils/cn'
import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { Checkbox } from './Checkbox'

interface Column<T> {
  key: string
  header: string
  cell: (row: T) => ReactNode
  sortable?: boolean
  width?: string
}

interface DataTableProps<T> {
  data: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string
  selectable?: boolean
  selectedKeys?: Set<string>
  onSelectionChange?: (keys: Set<string>) => void
  onRowClick?: (row: T) => void
  emptyState?: ReactNode
  isLoading?: boolean
  stickyHeader?: boolean
  className?: string
}

export function DataTable<T>({
  data,
  columns,
  rowKey,
  selectable,
  selectedKeys = new Set(),
  onSelectionChange,
  onRowClick,
  emptyState,
  isLoading,
  stickyHeader = true,
  className,
}: DataTableProps<T>) {
  const [sortConfig, setSortConfig] = useState<{ key: string; direction: 'asc' | 'desc' } | null>(null)

  const handleSort = (key: string) => {
    setSortConfig((prev) => {
      if (prev?.key === key) {
        return prev.direction === 'asc' ? { key, direction: 'desc' } : null
      }
      return { key, direction: 'asc' }
    })
  }

  const handleSelectAll = () => {
    if (!onSelectionChange) return
    if (selectedKeys.size === data.length) {
      onSelectionChange(new Set())
    } else {
      onSelectionChange(new Set(data.map(rowKey)))
    }
  }

  const handleSelectRow = (key: string) => {
    if (!onSelectionChange) return
    const newKeys = new Set(selectedKeys)
    if (newKeys.has(key)) {
      newKeys.delete(key)
    } else {
      newKeys.add(key)
    }
    onSelectionChange(newKeys)
  }

  const allSelected = data.length > 0 && selectedKeys.size === data.length
  const someSelected = selectedKeys.size > 0 && selectedKeys.size < data.length

  return (
    <div className={cn('rounded-xl border border-border overflow-hidden', className)}>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead className={cn(
            'bg-surface-subtle border-b border-border',
            stickyHeader && 'sticky top-0 z-10'
          )}>
            <tr>
              {selectable && (
                <th className="w-12 px-4 py-3">
                  <Checkbox
                    checked={allSelected}
                    indeterminate={someSelected}
                    onChange={handleSelectAll}
                  />
                </th>
              )}
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cn(
                    'px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-content-secondary',
                    col.sortable && 'cursor-pointer select-none hover:text-content-primary'
                  )}
                  style={{ width: col.width }}
                  onClick={() => col.sortable && handleSort(col.key)}
                >
                  <div className="flex items-center gap-1">
                    {col.header}
                    {col.sortable && (
                      <span className="text-content-muted">
                        {sortConfig?.key === col.key ? (
                          sortConfig.direction === 'asc' ? (
                            <ChevronUp className="h-4 w-4" />
                          ) : (
                            <ChevronDown className="h-4 w-4" />
                          )
                        ) : (
                          <ChevronsUpDown className="h-4 w-4 opacity-50" />
                        )}
                      </span>
                    )}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border bg-surface-elevated">
            {data.length === 0 && !isLoading ? (
              <tr>
                <td colSpan={columns.length + (selectable ? 1 : 0)} className="px-6 py-12">
                  {emptyState || (
                    <div className="text-center text-content-muted">No data available</div>
                  )}
                </td>
              </tr>
            ) : (
              data.map((row) => {
                const key = rowKey(row)
                const isSelected = selectedKeys.has(key)

                return (
                  <tr
                    key={key}
                    className={cn(
                      'transition-colors',
                      isSelected && 'bg-brand-500/5',
                      onRowClick && 'cursor-pointer hover:bg-surface-muted',
                      !onRowClick && 'hover:bg-surface-subtle'
                    )}
                    onClick={() => onRowClick?.(row)}
                  >
                    {selectable && (
                      <td className="w-12 px-4 py-4" onClick={(e) => e.stopPropagation()}>
                        <Checkbox
                          checked={isSelected}
                          onChange={() => handleSelectRow(key)}
                        />
                      </td>
                    )}
                    {columns.map((col) => (
                      <td key={col.key} className="px-6 py-4 text-sm text-content-primary">
                        {col.cell(row)}
                      </td>
                    ))}
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
```

---

## 5. Implementation Phases

### Phase 1: Design Tokens Foundation (Week 1)

**Files to Modify:**

- [frontend/src/index.css](frontend/src/index.css) - Add CSS variables
- [frontend/tailwind.config.js](frontend/tailwind.config.js) - Add semantic color mapping

**Files to Create:**

- None (modify existing)

**Tasks:**

1. Add CSS custom properties to `:root` and `.dark` in index.css
2. Update tailwind.config.js with new color tokens
3. Test light/dark mode switching
4. Verify no visual regressions

**Testing:**

- Visual regression test for Dashboard, Security, ProxyHosts
- Dark/light mode toggle verification
- Build succeeds without errors

---

### Phase 2: Core Component Library (Weeks 2-3)

**Files to Create:**

- [frontend/src/components/ui/Badge.tsx](frontend/src/components/ui/Badge.tsx)
- [frontend/src/components/ui/Alert.tsx](frontend/src/components/ui/Alert.tsx)
- [frontend/src/components/ui/Dialog.tsx](frontend/src/components/ui/Dialog.tsx)
- [frontend/src/components/ui/Select.tsx](frontend/src/components/ui/Select.tsx)
- [frontend/src/components/ui/Tabs.tsx](frontend/src/components/ui/Tabs.tsx)
- [frontend/src/components/ui/Tooltip.tsx](frontend/src/components/ui/Tooltip.tsx)
- [frontend/src/components/ui/Skeleton.tsx](frontend/src/components/ui/Skeleton.tsx)
- [frontend/src/components/ui/Progress.tsx](frontend/src/components/ui/Progress.tsx)
- [frontend/src/components/ui/Checkbox.tsx](frontend/src/components/ui/Checkbox.tsx)
- [frontend/src/components/ui/Label.tsx](frontend/src/components/ui/Label.tsx)
- [frontend/src/components/ui/Textarea.tsx](frontend/src/components/ui/Textarea.tsx)
- [frontend/src/components/ui/index.ts](frontend/src/components/ui/index.ts) - Barrel exports

**Files to Modify:**

- [frontend/src/components/ui/Button.tsx](frontend/src/components/ui/Button.tsx) - Enhance with variants
- [frontend/src/components/ui/Card.tsx](frontend/src/components/ui/Card.tsx) - Add hover, variants
- [frontend/src/components/ui/Input.tsx](frontend/src/components/ui/Input.tsx) - Enhance styling
- [frontend/src/components/ui/Switch.tsx](frontend/src/components/ui/Switch.tsx) - Use tokens

**Dependencies to Add:**

```bash
npm install class-variance-authority @radix-ui/react-dialog @radix-ui/react-tooltip @radix-ui/react-tabs @radix-ui/react-select @radix-ui/react-checkbox @radix-ui/react-progress
```

**Testing:**

- Unit tests for each new component
- Storybook-style visual verification (manual)
- Accessibility audit (keyboard nav, screen reader)

---

### Phase 3: Layout Components (Week 4)

**Files to Create:**

- [frontend/src/components/layout/PageShell.tsx](frontend/src/components/layout/PageShell.tsx)
- [frontend/src/components/ui/StatsCard.tsx](frontend/src/components/ui/StatsCard.tsx)
- [frontend/src/components/ui/EmptyState.tsx](frontend/src/components/ui/EmptyState.tsx) (enhance existing)
- [frontend/src/components/ui/DataTable.tsx](frontend/src/components/ui/DataTable.tsx)

**Files to Modify:**

- [frontend/src/components/Layout.tsx](frontend/src/components/Layout.tsx) - Apply token system

**Testing:**

- Responsive layout tests
- Mobile sidebar behavior
- Table scrolling with sticky headers

---

### Phase 4: Page-by-Page Polish (Weeks 5-7)

#### 4.1 Dashboard (Week 5)

**Files to Modify:**

- [frontend/src/pages/Dashboard.tsx](frontend/src/pages/Dashboard.tsx)

**Changes:**

- Replace link cards with `StatsCard` component
- Add trend indicators
- Improve UptimeWidget styling
- Add skeleton loading states
- Consistent page padding

#### 4.2 ProxyHosts (Week 5)

**Files to Modify:**

- [frontend/src/pages/ProxyHosts.tsx](frontend/src/pages/ProxyHosts.tsx)
- [frontend/src/components/ProxyHostForm.tsx](frontend/src/components/ProxyHostForm.tsx)

**Changes:**

- Replace inline table with `DataTable` component
- Replace inline modals with `Dialog` component
- Use `Badge` for SSL/WS/ACL indicators
- Use `Alert` for error states
- Add `EmptyState` for no hosts

#### 4.3 Security Dashboard (Week 6)

**Files to Modify:**

- [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx)

**Changes:**

- Use enhanced `Card` with hover states
- Use `Badge` for status indicators
- Improve layer card spacing
- Consistent button variants

#### 4.4 Settings (Week 6)

**Files to Modify:**

- [frontend/src/pages/Settings.tsx](frontend/src/pages/Settings.tsx)
- [frontend/src/pages/SystemSettings.tsx](frontend/src/pages/SystemSettings.tsx)
- [frontend/src/pages/SMTPSettings.tsx](frontend/src/pages/SMTPSettings.tsx)
- [frontend/src/pages/Account.tsx](frontend/src/pages/Account.tsx)

**Changes:**

- Replace tab links with `Tabs` component
- Improve form field styling with `Label`
- Use `Alert` for validation errors
- Consistent page shell

#### 4.5 AccessLists (Week 7)

**Files to Modify:**

- [frontend/src/pages/AccessLists.tsx](frontend/src/pages/AccessLists.tsx)
- [frontend/src/components/AccessListForm.tsx](frontend/src/components/AccessListForm.tsx)

**Changes:**

- Replace inline table with `DataTable`
- Replace confirm dialogs with `Dialog`
- Use `Alert` for CGNAT warning
- Use `Badge` for ACL types

#### 4.6 Other Pages (Week 7)

**Files to Review/Modify:**

- [frontend/src/pages/Certificates.tsx](frontend/src/pages/Certificates.tsx)
- [frontend/src/pages/RemoteServers.tsx](frontend/src/pages/RemoteServers.tsx)
- [frontend/src/pages/Logs.tsx](frontend/src/pages/Logs.tsx)
- [frontend/src/pages/Backups.tsx](frontend/src/pages/Backups.tsx)

**Changes:**

- Apply consistent `PageShell` wrapper
- Use new component library throughout
- Add loading skeletons
- Improve empty states

---

## 6. Page-by-Page Improvement Checklist

### Dashboard

- [ ] Replace link cards with `StatsCard`
- [ ] Add trend indicators (up/down arrows)
- [ ] Skeleton loading states
- [ ] Consistent spacing (page gutter)
- [ ] Improve CertificateStatusCard styling

### ProxyHosts

- [ ] `DataTable` with sticky header
- [ ] `Dialog` for add/edit forms
- [ ] `Badge` for SSL/WS/ACL status
- [ ] `EmptyState` when no hosts
- [ ] Bulk action bar styling
- [ ] Loading skeleton

### Security

- [ ] Improved layer cards with consistent padding
- [ ] `Badge` for status indicators
- [ ] Better disabled state styling
- [ ] `Alert` for Cerberus disabled message
- [ ] Consistent button variants

### Settings

- [ ] `Tabs` component for navigation
- [ ] Form field consistency
- [ ] `Alert` for validation
- [ ] Success toast styling

### AccessLists

- [ ] `DataTable` with selection
- [ ] `Dialog` for confirmations
- [ ] `Alert` for CGNAT warning
- [ ] `Badge` for ACL types
- [ ] `EmptyState` when none exist

### Certificates

- [ ] `DataTable` for certificate list
- [ ] `Badge` for status (valid/expiring/expired)
- [ ] `Dialog` for upload form
- [ ] Improved certificate details view

### Logs

- [ ] Improved filter styling
- [ ] `Badge` for log levels
- [ ] Better table density
- [ ] Skeleton during load

### Backups

- [ ] `DataTable` for backup list
- [ ] `Dialog` for restore confirmation
- [ ] `Badge` for backup type
- [ ] `EmptyState` when none exist

---

## 7. Testing Requirements

### Unit Tests

Each new component needs:

- Render test (renders without crashing)
- Variant tests (all variants render correctly)
- Interaction tests (onClick, onChange work)
- Accessibility tests (aria labels, keyboard nav)

### Integration Tests

- Dark/light mode toggle persists
- Page navigation maintains theme
- Forms submit correctly with new components
- Modals open/close properly

### Visual Regression

- Screenshot comparison for:
  - Dashboard (light + dark)
  - ProxyHosts table (empty + populated)
  - Security dashboard (enabled + disabled)
  - Settings tabs

### Accessibility

- WCAG 2.1 AA compliance
- Keyboard navigation throughout
- Focus visible on all interactive elements
- Screen reader compatibility

---

## 8. Migration Strategy

### Backward Compatibility

1. Keep legacy color tokens (`dark-bg`, `dark-card`, etc.) during transition
2. Gradually replace hardcoded colors with semantic tokens
3. Use `cn()` utility for all className merging
4. Create new components alongside existing, migrate pages incrementally

### Rollout Order

1. **Token system** - No visual change, foundation only
2. **New components** - Available but not used
3. **Dashboard** - High visibility, validates approach
4. **ProxyHosts** - Most complex, proves scalability
5. **Remaining pages** - Systematic cleanup

### Deprecation Path

After all pages migrated:

1. Remove legacy color tokens from tailwind.config.js
2. Remove inline modal patterns
3. Remove ad-hoc button styling
4. Clean up unused CSS

---

## 9. Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Unique color values in CSS | 50+ hardcoded | <20 via tokens |
| Component reuse | ~20% | >80% |
| Inline styles | Prevalent | Eliminated |
| Accessibility score (Lighthouse) | Unknown | 90+ |
| Dark/light mode support | Partial | Complete |
| Loading states coverage | ~30% | 100% |
| Empty states coverage | ~50% | 100% |

---

## 10. Open Questions / Decisions Needed

1. **Font loading strategy**: Should we self-host Inter/JetBrains Mono or use CDN?
2. **Animation library**: Use Framer Motion for complex animations or keep CSS-only?
3. **Form library integration**: Deeper react-hook-form integration with new Input components?
4. **Icon library**: Stick with lucide-react or consider alternatives?
5. **Radix UI scope**: All primitives or selective use for accessibility-critical components?

---

## Appendix A: File Change Summary

### New Files (23)

```
frontend/src/components/ui/Badge.tsx
frontend/src/components/ui/Alert.tsx
frontend/src/components/ui/Dialog.tsx
frontend/src/components/ui/Select.tsx
frontend/src/components/ui/Tabs.tsx
frontend/src/components/ui/Tooltip.tsx
frontend/src/components/ui/Skeleton.tsx
frontend/src/components/ui/Progress.tsx
frontend/src/components/ui/Checkbox.tsx
frontend/src/components/ui/Label.tsx
frontend/src/components/ui/Textarea.tsx
frontend/src/components/ui/StatsCard.tsx
frontend/src/components/ui/EmptyState.tsx
frontend/src/components/ui/DataTable.tsx
frontend/src/components/ui/index.ts
frontend/src/components/layout/PageShell.tsx
frontend/src/components/ui/__tests__/Badge.test.tsx
frontend/src/components/ui/__tests__/Alert.test.tsx
frontend/src/components/ui/__tests__/Dialog.test.tsx
frontend/src/components/ui/__tests__/Skeleton.test.tsx
frontend/src/components/ui/__tests__/DataTable.test.tsx
frontend/src/components/ui/__tests__/EmptyState.test.tsx
frontend/src/components/ui/__tests__/StatsCard.test.tsx
```

### Modified Files (20+)

```
frontend/src/index.css
frontend/tailwind.config.js
frontend/package.json
frontend/src/components/ui/Button.tsx
frontend/src/components/ui/Card.tsx
frontend/src/components/ui/Input.tsx
frontend/src/components/ui/Switch.tsx
frontend/src/components/Layout.tsx
frontend/src/pages/Dashboard.tsx
frontend/src/pages/ProxyHosts.tsx
frontend/src/pages/Security.tsx
frontend/src/pages/Settings.tsx
frontend/src/pages/AccessLists.tsx
frontend/src/pages/Certificates.tsx
frontend/src/pages/RemoteServers.tsx
frontend/src/pages/Logs.tsx
frontend/src/pages/Backups.tsx
frontend/src/pages/SystemSettings.tsx
frontend/src/pages/SMTPSettings.tsx
frontend/src/pages/Account.tsx
```

---

*Plan created: December 16, 2025*
*Estimated completion: 7 weeks*
*Issue reference: GitHub #409*
