---
title: Dark Mode & Modern UI
description: Toggle between light and dark themes with a clean, modern interface
---

# Dark Mode & Modern UI

Easy on the eyes, day or night. Toggle between light and dark themes to match your preference. The clean, modern interface makes managing complex setups feel simple.

## Overview

Charon's interface is built with **Tailwind CSS v4** and a modern React component library. Dark mode is the default, with automatic system preference detection and manual override support.

### Design Philosophy

- **Dark-first**: Optimized for low-light environments and reduced eye strain
- **Semantic colors**: Consistent meaning across light and dark modes
- **Accessibility-first**: WCAG 2.1 AA compliant with focus management
- **Responsive**: Works seamlessly on desktop, tablet, and mobile

## Why a Modern UI Matters

| Feature | Benefit |
|---------|---------|
| **Dark Mode** | Reduced eye strain during long sessions |
| **Semantic Tokens** | Consistent, predictable color behavior |
| **Component Library** | Professional, polished interactions |
| **Keyboard Navigation** | Full functionality without a mouse |
| **Screen Reader Support** | Accessible to all users |

## Theme System

### Color Tokens

Charon uses semantic color tokens that automatically adapt:

| Token | Light Mode | Dark Mode | Usage |
|-------|------------|-----------|-------|
| `--background` | White | Slate 950 | Page backgrounds |
| `--foreground` | Slate 900 | Slate 50 | Primary text |
| `--primary` | Blue 600 | Blue 500 | Actions, links |
| `--destructive` | Red 600 | Red 500 | Delete, errors |
| `--muted` | Slate 100 | Slate 800 | Secondary surfaces |
| `--border` | Slate 200 | Slate 700 | Dividers, outlines |

### Switching Themes

1. Click the **theme toggle** in the top navigation
2. Choose: **Light**, **Dark**, or **System**
3. Preference is saved to local storage

## Component Library

### Core Components

| Component | Purpose | Accessibility |
|-----------|---------|---------------|
| **Badge** | Status indicators, tags | Color + icon redundancy |
| **Alert** | Notifications, warnings | ARIA live regions |
| **Dialog** | Modal interactions | Focus trap, ESC to close |
| **DataTable** | Sortable data display | Keyboard navigation |
| **Tooltip** | Contextual help | Delay for screen readers |
| **DropdownMenu** | Action menus | Arrow key navigation |

### Status Indicators

Visual status uses color AND icons for accessibility:

- ✅ **Online** - Green badge with check icon
- ⚠️ **Warning** - Yellow badge with alert icon
- ❌ **Offline** - Red badge with X icon
- ⏳ **Pending** - Gray badge with clock icon

## Accessibility Features

### WCAG 2.1 Compliance

- **Color contrast**: Minimum 4.5:1 for text, 3:1 for UI elements
- **Focus indicators**: Visible focus rings on all interactive elements
- **Text scaling**: UI adapts to browser zoom up to 200%
- **Motion**: Respects `prefers-reduced-motion`

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `Tab` | Move between interactive elements |
| `Enter` / `Space` | Activate buttons, links |
| `Escape` | Close dialogs, dropdowns |
| `Arrow keys` | Navigate within menus, tables |

### Screen Reader Support

- Semantic HTML structure with landmarks
- ARIA labels on icon-only buttons
- Live regions for dynamic content updates
- Skip links for main content access

## Customization

### CSS Variables Override

Advanced users can customize the theme via CSS:

```css
/* Custom brand colors */
:root {
  --primary: 210 100% 50%;  /* Custom blue */
  --radius: 0.75rem;        /* Rounder corners */
}
```

## Related

- [Notifications](notifications.md) - Visual notification system
- [REST API](api.md) - Programmatic access
- [Back to Features](../features.md)
