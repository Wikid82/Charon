# Manual Testing Plan: Sidebar Scrolling & Fixed Header UI/UX

**Feature**: Sidebar Navigation Scrolling and Fixed Header Bar
**Branch**: `feature/beta-release`
**Created**: December 21, 2025
**Status**: Ready for Manual Testing

---

## Overview

This manual test plan focuses on validating the UI/UX improvements for:

1. **Scrollable Sidebar Navigation**: Ensures logout button remains accessible when submenus expand
2. **Fixed Header Bar**: Keeps header visible when scrolling page content

---

## Test Environment Setup

### Prerequisites

- [ ] Latest code from `feature/beta-release` branch pulled
- [ ] Frontend dependencies installed: `cd frontend && npm install`
- [ ] Development server running: `npm run dev`
- [ ] Browser DevTools open for console error monitoring

### Test Browsers

- [ ] Chrome/Edge (Chromium-based)
- [ ] Firefox
- [ ] Safari (if available)

### Test Modes

- [ ] Light theme
- [ ] Dark theme
- [ ] Desktop viewport (≥1024px width)
- [ ] Mobile viewport (<1024px width)

---

## Test Suite 1: Sidebar Navigation Scrolling

### Test Case 1.1: Expanded Sidebar with All Submenus Open

**Steps**:

1. Open Charon in browser at desktop resolution (≥1024px)
2. Ensure sidebar is expanded (click hamburger if collapsed)
3. Click "Settings" menu item to expand its submenu
4. Click "Tasks" menu item to expand its submenu
5. Expand any nested submenus within Tasks
6. Click "Security" menu item to expand its submenu
7. Scroll within the sidebar navigation area

**Expected Results**:

- [ ] Sidebar navigation area shows a subtle scrollbar when content overflows
- [ ] Scrollbar is styled with custom colors matching the theme
- [ ] Logout button remains visible at the bottom of the sidebar
- [ ] Version info remains visible above the logout button
- [ ] Scrollbar thumb color is semi-transparent gray in light mode
- [ ] Scrollbar thumb color is lighter in dark mode
- [ ] Smooth scrolling behavior (no jank or stutter)

**Bug Indicators**:

- ❌ Logout button pushed off-screen
- ❌ Harsh default scrollbar styling
- ❌ No scrollbar when content overflows
- ❌ Layout jumps or visual glitches

---

### Test Case 1.2: Collapsed Sidebar State

**Steps**:

1. Click the hamburger menu icon to collapse the sidebar
2. Observe the compact icon-only sidebar

**Expected Results**:

- [ ] Collapsed sidebar shows only icons
- [ ] Logout icon remains visible at bottom
- [ ] No scrollbar needed (all items fit in viewport height)
- [ ] Hover tooltips work for each icon
- [ ] Smooth transition animation when collapsing

**Bug Indicators**:

- ❌ Logout icon not visible
- ❌ Jerky collapse animation
- ❌ Icons overlapping or misaligned

---

### Test Case 1.3: Sidebar Scrollbar Interactivity

**Steps**:

1. Expand sidebar with multiple submenus open (repeat Test Case 1.1 steps 2-6)
2. Hover over the scrollbar
3. Click and drag the scrollbar thumb
4. Use mouse wheel to scroll
5. Use keyboard arrow keys to navigate menu items

**Expected Results**:

- [ ] Scrollbar thumb becomes slightly more opaque on hover
- [ ] Dragging scrollbar thumb scrolls content smoothly
- [ ] Mouse wheel scrolling works within sidebar
- [ ] Keyboard navigation (Tab, Arrow keys) works
- [ ] Active menu item scrolls into view when selected via keyboard

**Bug Indicators**:

- ❌ Scrollbar not interactive
- ❌ Keyboard navigation broken
- ❌ Scrolling feels laggy or stutters

---

### Test Case 1.4: Mobile Sidebar Behavior

**Steps**:

1. Resize browser to mobile viewport (<1024px) or use DevTools device emulation
2. Click hamburger menu to open mobile sidebar overlay
3. Expand multiple submenus
4. Scroll within the sidebar

**Expected Results**:

- [ ] Sidebar appears as overlay with backdrop
- [ ] Navigation area is scrollable if content overflows
- [ ] Logout button remains at bottom
- [ ] Same scrollbar styling as desktop
- [ ] Closing sidebar (click backdrop or X) works smoothly

**Bug Indicators**:

- ❌ Mobile sidebar not scrollable
- ❌ Logout button hidden on mobile
- ❌ Backdrop not dismissing sidebar

---

## Test Suite 2: Fixed Header Bar

### Test Case 2.1: Header Visibility During Content Scroll

**Steps**:

1. Navigate to a page with long content (e.g., Proxy Hosts with many entries)
2. Scroll down the page content at least 500px
3. Continue scrolling to bottom of page
4. Scroll back to top

**Expected Results**:

- [ ] Desktop header bar remains fixed at top of viewport
- [ ] Header does not scroll with content
- [ ] Header background and border remain visible
- [ ] All header elements remain interactive (notifications, theme toggle, etc.)
- [ ] No layout shift or jank when scrolling
- [ ] Content scrolls smoothly beneath the header

**Bug Indicators**:

- ❌ Header scrolls off-screen with content
- ❌ Header jumps or stutters
- ❌ Buttons in header become unresponsive
- ❌ Layout shifts causing horizontal scrollbar

---

### Test Case 2.2: Header Element Interactivity

**Steps**:

1. With page scrolled down (header should be at top of viewport)
2. Click the sidebar collapse/expand button in header
3. Click the notifications icon
4. Click the theme toggle button
5. Open the user menu dropdown (if present)

**Expected Results**:

- [ ] Sidebar collapse button works correctly
- [ ] Notifications dropdown opens anchored to header
- [ ] Theme toggle switches between light/dark mode
- [ ] Dropdowns appear above content (correct z-index)
- [ ] All click targets remain accurate (no misalignment)

**Bug Indicators**:

- ❌ Dropdowns appear behind header or content
- ❌ Buttons not responding to clicks
- ❌ Dropdowns positioned incorrectly

---

### Test Case 2.3: Mobile Header Behavior

**Steps**:

1. Resize to mobile viewport (<1024px)
2. Scroll page content down
3. Observe mobile header behavior

**Expected Results**:

- [ ] Mobile header remains fixed at top (existing behavior preserved)
- [ ] No regressions in mobile header functionality
- [ ] Sidebar toggle button works
- [ ] Content scrolls beneath mobile header

**Bug Indicators**:

- ❌ Mobile header scrolls away
- ❌ Mobile header overlaps with content
- ❌ Hamburger menu not working

---

### Test Case 2.4: Header Z-Index Hierarchy

**Steps**:

1. Desktop viewport (≥1024px)
2. Open the notifications dropdown from header
3. Observe dropdown positioning relative to header
4. Open sidebar (expand if collapsed)
5. Observe sidebar relative to header

**Expected Results**:

- [ ] Sidebar (z-30) appears above header (z-10) ✅
- [ ] Dropdowns in header appear correctly (not behind content)
- [ ] No visual overlapping issues
- [ ] Proper layering: Content < Header < Dropdowns < Sidebar < Modals

**Bug Indicators**:

- ❌ Dropdown hidden behind header
- ❌ Sidebar hidden behind header
- ❌ Content appearing above header

---

## Test Suite 3: Responsive Design & Theme Switching

### Test Case 3.1: Viewport Resize Behavior

**Steps**:

1. Start at desktop viewport (≥1024px)
2. Expand sidebar with submenus
3. Slowly resize browser width from 1400px → 1000px → 768px → 375px
4. Observe layout transitions at breakpoints

**Expected Results**:

- [ ] Smooth transition at 1024px breakpoint (desktop ↔ mobile)
- [ ] Sidebar transitions from expanded to overlay mode
- [ ] Header transitions from desktop to mobile style
- [ ] No horizontal scrollbars at any viewport size
- [ ] Content remains readable and accessible
- [ ] Scrolling continues to work in both modes

**Bug Indicators**:

- ❌ Layout breaks at specific widths
- ❌ Horizontal scrollbar appears
- ❌ Elements overlap or get cut off
- ❌ Sudden jumps instead of smooth transitions

---

### Test Case 3.2: Dark/Light Theme Toggle with Scroll State

**Steps**:

1. Expand sidebar with multiple submenus
2. Scroll sidebar to middle position (logout button out of view above)
3. Toggle between light and dark themes
4. Scroll page content down
5. Toggle theme again

**Expected Results**:

- [ ] Sidebar scroll position preserved after theme toggle
- [ ] Scrollbar styling updates to match new theme
- [ ] Header background color updates correctly
- [ ] Content scroll position preserved after theme toggle
- [ ] No flashing or visual glitches during theme transition

**Bug Indicators**:

- ❌ Scroll position resets to top
- ❌ Scrollbar styling not updating
- ❌ Layout shifts during theme change
- ❌ Flash of unstyled content

---

### Test Case 3.3: Browser Zoom Levels

**Steps**:

1. Set browser zoom to 50% (Ctrl/Cmd + Mouse wheel or View menu)
2. Verify sidebar scrolling and header behavior
3. Set browser zoom to 100% (default)
4. Verify functionality
5. Set browser zoom to 200%
6. Verify functionality

**Expected Results**:

- [ ] Sidebar scrolling works at all zoom levels
- [ ] Header remains fixed at all zoom levels
- [ ] No horizontal scrollbars introduced by zoom
- [ ] Text remains readable
- [ ] Layout remains functional

**Bug Indicators**:

- ❌ Horizontal scrollbars at high zoom
- ❌ Elements overlap at extreme zoom levels
- ❌ Scrolling broken at specific zoom
- ❌ Text or icons cut off

---

## Test Suite 4: Cross-Browser Compatibility

### Test Case 4.1: Chrome/Edge (Chromium)

**Steps**:

1. Run all test suites 1-3 in Chrome or Edge
2. Open DevTools Console and check for errors
3. Monitor Performance tab for any issues

**Expected Results**:

- [ ] All features work as expected
- [ ] No console errors related to layout or scrolling
- [ ] Smooth 60fps scrolling in Performance tab
- [ ] Custom scrollbar styling applied correctly

---

### Test Case 4.2: Firefox

**Steps**:

1. Open Charon in Firefox
2. Run all test suites 1-3
3. Verify Firefox-specific scrollbar styling (`scrollbar-width: thin`)

**Expected Results**:

- [ ] All features work as expected
- [ ] Firefox thin scrollbar styling applied
- [ ] Scrollbar color matches theme (via `scrollbar-color` property)
- [ ] No layout differences compared to Chrome

**Bug Indicators**:

- ❌ Thick default scrollbar in Firefox
- ❌ Layout differences from Chrome

---

### Test Case 4.3: Safari (if available)

**Steps**:

1. Open Charon in Safari (macOS)
2. Run all test suites 1-3
3. Verify `position: sticky` works correctly

**Expected Results**:

- [ ] Header `position: sticky` works (Safari 13+ supports this)
- [ ] All features work as expected
- [ ] WebKit scrollbar styling applied
- [ ] Smooth scrolling on trackpad

**Bug Indicators**:

- ❌ Header not sticking in Safari
- ❌ Scrollbar styling not applied
- ❌ Stuttery scrolling

---

## Test Suite 5: Accessibility & Keyboard Navigation

### Test Case 5.1: Keyboard Navigation Through Sidebar

**Steps**:

1. Click in browser address bar, then press Tab to enter page
2. Use Tab key to navigate through sidebar menu items
3. Expand submenus using Enter or Space keys
4. Continue tabbing through all menu items
5. Tab to logout button

**Expected Results**:

- [ ] Focus indicator visible on each menu item
- [ ] Focused items scroll into view automatically
- [ ] Can reach and activate logout button via keyboard
- [ ] No keyboard traps (can Tab out of sidebar)
- [ ] Focus order is logical (top to bottom)

**Bug Indicators**:

- ❌ Focused items not scrolling into view
- ❌ Cannot reach logout button via keyboard
- ❌ Focus indicator not visible
- ❌ Keyboard trapped in sidebar

---

### Test Case 5.2: Screen Reader Testing (Optional)

**Steps**:

1. Enable screen reader (NVDA, JAWS, VoiceOver)
2. Navigate through sidebar menu
3. Navigate through header elements

**Expected Results**:

- [ ] Sidebar navigation announced as "navigation" landmark
- [ ] Menu items announced with proper labels
- [ ] Current page announced correctly
- [ ] Header elements announced with proper labels
- [ ] No unexpected focus changes

---

## Test Suite 6: Performance & Edge Cases

### Test Case 6.1: Rapid Sidebar Collapse/Expand

**Steps**:

1. Rapidly click sidebar collapse button 10 times
2. Observe for memory leaks or performance degradation

**Expected Results**:

- [ ] Smooth transitions even with rapid toggling
- [ ] No memory leaks (check DevTools Memory tab)
- [ ] No console errors
- [ ] Animations complete correctly

**Bug Indicators**:

- ❌ Animations stuttering after multiple toggles
- ❌ Memory usage increasing
- ❌ Console errors appearing

---

### Test Case 6.2: Long Page Content Stress Test

**Steps**:

1. Navigate to Proxy Hosts page
2. If limited data, use browser DevTools to inject 100+ fake host entries into the list
3. Scroll from top to bottom of the page rapidly

**Expected Results**:

- [ ] Header remains fixed throughout scroll
- [ ] No layout thrashing or repaints (check DevTools Performance)
- [ ] Smooth scrolling even with large DOM
- [ ] No memory leaks

**Bug Indicators**:

- ❌ Stuttering during scroll
- ❌ Header jumping or flickering
- ❌ Performance degradation with large lists

---

### Test Case 6.3: Focus Management After Scroll

**Steps**:

1. Focus an element in header (e.g., notifications button)
2. Scroll page content down 500px
3. Click focused element
4. Expand sidebar and scroll it
5. Focus logout button
6. Verify button remains visible

**Expected Results**:

- [ ] Focused elements in header remain accessible
- [ ] Clicking focused elements works correctly
- [ ] Focused elements in sidebar scroll into view
- [ ] No focus lost during scrolling

**Bug Indicators**:

- ❌ Focused element not visible after scroll
- ❌ Click targets misaligned
- ❌ Focus lost unexpectedly

---

## Known Issues & Expected Behavior

### Not a Bug (Expected)

- **Existing Linting Warnings**: 40 pre-existing TypeScript warnings unrelated to this change
- **Nested Sticky Elements**: Child components using `position: sticky` will be relative to content scroll container, not viewport (documented limitation)
- **Safari <13**: `position: sticky` not supported in very old Safari versions (not a target)

### Future Enhancements

- Smooth scroll to active menu item on page load
- Header shadow effect when content scrolls beneath
- Collapse sidebar automatically on mobile after navigation

---

## Bug Reporting Template

If you find a bug during testing, please report it with the following details:

```markdown
**Test Case**: [e.g., Test Case 1.1]
**Browser**: [e.g., Chrome 120 on Windows 11]
**Viewport**: [e.g., Desktop 1920x1080]
**Theme**: [e.g., Dark mode]

**Steps to Reproduce**:
1. [Step 1]
2. [Step 2]
3. [Step 3]

**Expected Result**:
[What should happen]

**Actual Result**:
[What actually happened]

**Screenshot/Video**:
[Attach if possible]

**Console Errors**:
```

[Paste any console errors]

```

**Severity**: [Critical / High / Medium / Low]
```

---

## Sign-Off Checklist

After completing all test suites, verify:

- [ ] All Test Suite 1 tests passed (Sidebar Scrolling)
- [ ] All Test Suite 2 tests passed (Fixed Header)
- [ ] All Test Suite 3 tests passed (Responsive & Themes)
- [ ] All Test Suite 4 tests passed (Cross-Browser)
- [ ] All Test Suite 5 tests passed (Accessibility)
- [ ] All Test Suite 6 tests passed (Performance)
- [ ] No Critical or High severity bugs found
- [ ] Medium/Low bugs documented and triaged
- [ ] Tested in at least 2 browsers (Chrome + Firefox/Safari)
- [ ] Tested in both light and dark themes
- [ ] Tested at mobile and desktop viewports

---

**Tester Name**: ___________________________
**Date Tested**: ___________________________
**Branch**: `feature/beta-release`
**Commit SHA**: ___________________________
**Overall Result**: [ ] PASS / [ ] FAIL

---

## Additional Notes

[Add any observations, edge cases discovered, or suggestions here]
