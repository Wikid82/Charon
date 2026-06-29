---
title: Themes & Personalization
description: Choose from built-in themes, build your own with a color picker, and upload a custom logo
---

# Themes & Personalization

Make Charon look exactly the way you want. Pick from five ready-made themes, build a completely custom one with a color picker, or upload your own logo. Your preferences are saved instantly and applied the moment the page loads — no flicker, no flash.

## Choosing a Theme

Open **Settings → Appearance** to see the theme gallery. Each theme is shown as a visual preview card so you know exactly what you're getting before you pick it.

### Built-In Themes

| Theme | Best For |
|-------|---------|
| **Dark** (default) | Low-light environments, long sessions |
| **Light** | Bright rooms, printing |
| **High Contrast Dark** | Maximum readability on dark backgrounds |
| **High Contrast Light** | Maximum readability on light backgrounds |
| **Solarized** | A popular low-eyestrain palette |

Hover over any card to see a live preview applied to the whole interface before you commit to it.

### Follow System

Turn on **Follow System** and Charon will automatically switch between Light and Dark to match your operating system's setting. When you change your OS theme, Charon changes with it — no manual toggle needed.

## Custom Colors

Not happy with the built-in options? Click **Custom** in the gallery to open the color picker. You can set any color you like for every part of the interface:

- Background and surface colors
- Text and heading colors
- Accent and action colors
- Border and divider colors
- Status indicator colors (success, warning, error)

Changes apply instantly as you pick colors, so you can see exactly what everything will look like.

## Saving & Sharing Themes

### Export

Built a custom theme you love? Click **Export Theme** to download it as a small `.json` file. Keep it as a backup or share it with other Charon users.

### Import

Got a theme file from someone else? Click **Import Theme** and select the file. Charon validates it before applying — no broken themes, no security risks.

## Logo Customization

Replace the Charon logo in the sidebar with your own image. Go to **Settings → Appearance → Logo** and either:

- **Upload a file** — PNG, JPG, or WebP, up to 2 MB
- **Paste a URL** — Point to any image hosted online

Click **Reset** at any time to go back to the default Charon logo.

> **Note:** Logo changes require admin access. Non-admin users will see the logo but cannot change it.

## How It Works (No Flicker)

Charon applies your saved theme before the page finishes loading. This means there is no flash of the wrong colors when the page first appears — the right theme is there from the very first pixel.

## Accessibility

All built-in themes meet **WCAG 2.1 AA** contrast requirements. The High Contrast themes exceed AA and approach AAA for users who need extra readability. Charon also respects `prefers-reduced-motion` — animations are minimized if your system has that setting enabled.

## Keyboard Navigation

| Key | Action |
|-----|--------|
| `Tab` | Move between theme cards |
| `Enter` / `Space` | Select a theme |
| `Escape` | Close color picker / preview overlay |
| `Arrow keys` | Navigate within the gallery |

## Related

- [Notifications](notifications.md) — Visual notification system
- [REST API](api.md) — Programmatic access
- [Back to Features](../features.md)
