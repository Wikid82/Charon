---
title: Point & Click Management
description: Manage your reverse proxy through an intuitive web interface
category: core
---

# Point & Click Management

Say goodbye to editing configuration files and memorizing commands. Charon gives you a beautiful web interface where you simply type your domain name, select your backend service, and click save.

## Overview

Traditional reverse proxy configuration requires editing text files, understanding complex syntax, and reloading services. Charon replaces this workflow with an intuitive web interface that makes proxy management accessible to everyone.

### Key Capabilities

- **Form-Based Configuration**: Fill in fields instead of writing syntax
- **Instant Validation**: Catch errors before they break your setup
- **Live Preview**: See configuration changes before applying
- **One-Click Actions**: Enable, disable, or delete hosts instantly

## Why Use This

### No Config Files Needed

- Never edit Caddyfile, nginx.conf, or Apache configs manually
- Changes apply immediately without service restarts
- Syntax errors become impossible—the UI validates everything

### Reduced Learning Curve

- New team members are productive in minutes
- No need to memorize directives or options
- Tooltips explain each setting's purpose

### Audit Trail

- See who changed what and when
- Roll back to previous configurations
- Track configuration drift over time

## Features

### Form-Based Host Creation

Creating a new proxy host takes seconds:

1. Click **Add Host**
2. Enter domain name (e.g., `app.example.com`)
3. Enter backend address (e.g., `http://192.168.1.100:3000`)
4. Toggle SSL certificate option
5. Click **Save**

### Bulk Operations

Manage multiple hosts efficiently:

- **Bulk Enable/Disable**: Select hosts and toggle status
- **Bulk Delete**: Remove multiple hosts at once
- **Bulk Export**: Download configurations for backup
- **Clone Host**: Duplicate configuration to new domain

### Search and Filter

Find hosts quickly in large deployments:

- Search by domain name
- Filter by status (enabled, disabled, error)
- Filter by certificate status
- Sort by name, creation date, or last modified

## Mobile-Friendly Design

Charon's responsive interface works on any device:

- **Phone**: Manage proxies from anywhere
- **Tablet**: Full functionality with touch-friendly controls
- **Desktop**: Complete dashboard with side-by-side panels

### Dark Mode Interface

Reduce eye strain during late-night maintenance:

- Automatic detection of system preference
- Manual toggle in settings
- High contrast for accessibility
- Consistent styling across all components

## Configuration

### Accessing the UI

1. Open your browser to Charon's address (default: `http://localhost:81`)
2. Log in with your credentials
3. Dashboard displays all configured hosts

### Quick Actions

| Action | How To |
|--------|--------|
| Add new host | Click **+ Add Host** button |
| Edit host | Click host row or edit icon |
| Enable/Disable | Toggle switch in host row |
| Delete host | Click delete icon, confirm |
| View logs | Click host → **Logs** tab |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl/Cmd + N` | New host |
| `Ctrl/Cmd + S` | Save current form |
| `Ctrl/Cmd + F` | Focus search |
| `Escape` | Close modal/cancel |

## Dashboard Overview

The main dashboard provides at-a-glance status:

- **Total Hosts**: Number of configured proxies
- **Active/Inactive**: Hosts currently serving traffic
- **Certificate Status**: SSL expiration warnings
- **Recent Activity**: Latest configuration changes

## Related

- [Docker Integration](docker-integration.md) - Auto-discover containers
- [Caddyfile Import](caddyfile-import.md) - Migrate existing configs
- [Back to Features](../features.md)
