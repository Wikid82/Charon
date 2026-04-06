---
title: "Manual Test Plan - Issue #26: CrowdSec Dashboard Integration"
status: Open
priority: High
assignee: QA
labels: testing, backend, frontend, security
---

# Test Objective

Confirm that the CrowdSec Dashboard tab displays accurate security metrics, charts render correctly, time range filtering works, alerts paginate properly, and export produces valid output.

# Scope

- In scope: Dashboard tab navigation, summary cards, chart rendering, time range selector, active decisions table, alerts feed, CSV/JSON export, keyboard navigation, screen reader compatibility.
- Out of scope: CrowdSec engine start/stop/configuration, bouncer registration, existing security toggle behavior.

# Prerequisites

- Charon instance running with CrowdSec enabled and at least a few recorded decisions.
- Admin account credentials.
- Browser DevTools available for network inspection.
- Screen reader available for accessibility testing (e.g., NVDA, VoiceOver).

# Manual Scenarios

## 1) Dashboard Tab Navigation

- [ ] Navigate to `/security/crowdsec`.
- [ ] **Expected**: Two tabs are visible — "Configuration" and "Dashboard."
- [ ] Click the "Dashboard" tab.
- [ ] **Expected**: The dashboard loads with summary cards, charts, and the decisions table.
- [ ] Click the "Configuration" tab.
- [ ] **Expected**: The existing CrowdSec configuration interface appears unchanged.
- [ ] Click back to "Dashboard."
- [ ] **Expected**: Dashboard state is preserved (same time range, same data).

## 2) Summary Cards Accuracy

- [ ] Open the Dashboard tab with the default 24h time range.
- [ ] **Expected**: Four summary cards are displayed — Total Bans, Active Bans, Unique IPs, Top Scenario.
- [ ] Compare the "Active Bans" count against `cscli decisions list` output from the container.
- [ ] **Expected**: The counts match (within the 30-second cache window).
- [ ] Check that the trend indicator (percentage change) is visible on the Total Bans card.

## 3) Chart Rendering

- [ ] Confirm the ban timeline chart (area chart) renders with data points.
- [ ] **Expected**: The X-axis shows time labels and the Y-axis shows ban counts.
- [ ] Confirm the top attacking IPs chart (horizontal bar chart) renders.
- [ ] **Expected**: Up to 10 IP addresses are listed with proportional bars.
- [ ] Confirm the scenario breakdown chart (pie/donut chart) renders.
- [ ] **Expected**: Slices represent different CrowdSec scenarios with a legend.
- [ ] Hover over data points in each chart.
- [ ] **Expected**: Tooltips appear with relevant values.

## 4) Time Range Switching

- [ ] Select the "1h" time range.
- [ ] **Expected**: All cards and charts update to reflect the last 1 hour of data.
- [ ] Select "7d."
- [ ] **Expected**: Data expands to show the last 7 days.
- [ ] Select "30d."
- [ ] **Expected**: Data expands to show the last 30 days. Charts may show wider time buckets.
- [ ] Rapidly toggle between "1h" and "30d" several times.
- [ ] **Expected**: No stale data, no visual glitches, and no console errors. The most recently selected range is always displayed.

## 5) Active Decisions Table

- [ ] Scroll to the active decisions table on the Dashboard.
- [ ] **Expected**: Table columns include IP, Scenario, Duration, Type (ban/captcha), Origin, and Time Remaining.
- [ ] Verify that the "Time Remaining" column shows a countdown or human-readable time.
- [ ] If there are more than 10 active decisions, confirm pagination or scrolling works.
- [ ] If there are zero active decisions, confirm a placeholder message is shown (e.g., "No active decisions").

## 6) Alerts Feed

- [ ] Scroll to the alerts section of the Dashboard.
- [ ] **Expected**: A list of recent CrowdSec alerts is displayed with timestamps and scenario names.
- [ ] If there are enough alerts, confirm that pagination controls are present and functional.
- [ ] Click "Next" on the pagination.
- [ ] **Expected**: The next page of alerts loads without duplicates.
- [ ] Click "Previous."
- [ ] **Expected**: Returns to the first page with the original data.

## 7) CSV Export

- [ ] Click the "Export" button on the Dashboard.
- [ ] Select "CSV" as the format.
- [ ] **Expected**: A `.csv` file downloads to your machine.
- [ ] Open the file in a text editor or spreadsheet application.
- [ ] **Expected**: Columns match the decisions table (IP, Scenario, Duration, Type, etc.) and rows contain valid data.

## 8) JSON Export

- [ ] Click the "Export" button on the Dashboard.
- [ ] Select "JSON" as the format.
- [ ] **Expected**: A `.json` file downloads to your machine.
- [ ] Open the file in a text editor.
- [ ] **Expected**: Valid JSON array of decision objects. Fields match the decisions table.

## 9) Keyboard Navigation

- [ ] Use `Tab` to navigate from the tab bar to the summary cards, then to the charts, then to the table.
- [ ] **Expected**: Focus moves in a logical order. A visible focus indicator is shown on each interactive element.
- [ ] Use `Enter` or `Space` to activate the time range selector buttons.
- [ ] **Expected**: The selected time range changes and data updates.
- [ ] Use `Tab` to reach the "Export" button, then press `Enter`.
- [ ] **Expected**: The export dialog or menu opens.

## 10) Screen Reader Compatibility

- [ ] Enable a screen reader (NVDA, VoiceOver, or similar).
- [ ] Navigate to the Dashboard tab.
- [ ] **Expected**: The tab bar is announced correctly with "Configuration" and "Dashboard" tab names.
- [ ] Navigate to the summary cards.
- [ ] **Expected**: Each card's label and value is announced (e.g., "Total Bans: 1247").
- [ ] Navigate to the charts.
- [ ] **Expected**: Charts have accessible labels or descriptions (e.g., "Ban Timeline Chart").
- [ ] Navigate to the decisions table.
- [ ] **Expected**: Table headers and cell values are announced correctly.

# Edge Cases

## 11) Empty CrowdSec Data

- [ ] Disable CrowdSec or test on an instance with zero recorded decisions.
- [ ] Open the Dashboard tab.
- [ ] **Expected**: Summary cards show `0` values. Charts show an empty state or placeholder. The decisions table shows "No active decisions." No errors in the console.

## 12) Large Number of Decisions

- [ ] Test on an instance with 1,000+ recorded decisions (or simulate with test data).
- [ ] Open the Dashboard tab with the "30d" time range.
- [ ] **Expected**: Dashboard loads within 2 seconds. Charts render without performance issues. Pagination handles the large dataset.

## 13) CrowdSec LAPI Unavailable

- [ ] Stop the CrowdSec container while Charon is running.
- [ ] Open the Dashboard tab.
- [ ] **Expected**: Historical data from the database still renders. Active decisions and alerts show an error or "unavailable" state. No unhandled errors in the UI.

## 14) Rapid Time Range Switching Under Load

- [ ] On an instance with significant data, rapidly click through all five time ranges in quick succession.
- [ ] **Expected**: Only the final selection's data is displayed. No race conditions, stale data, or flickering.

# Expected Results

- Dashboard tab loads and displays all components (cards, charts, table, alerts).
- Summary card numbers match CrowdSec LAPI and database records within the cache window.
- Charts render with correct data for the selected time range.
- Export produces valid CSV and JSON files with matching data.
- Keyboard and screen reader users can navigate and interact with all dashboard elements.
- Edge cases (empty data, LAPI down, large datasets) are handled gracefully.

# Regression Checks

- [ ] Confirm the existing CrowdSec Configuration tab is unchanged in behavior and layout.
- [ ] Confirm CrowdSec start/stop/restart functionality is unaffected.
- [ ] Confirm existing security toggles (ACL, WAF, Rate Limiting) are unaffected.
- [ ] Confirm no new console errors appear on pages outside the Dashboard.
