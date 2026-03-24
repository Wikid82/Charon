---
title: "Manual Testing: Certificate Deletion UX Enhancement"
labels:
  - testing
  - feature
  - frontend
priority: medium
assignees: []
---

# Manual Testing: Certificate Deletion UX Enhancement

## Description

Manual test plan for expanded certificate deletion. Focuses on edge cases and race conditions that automated E2E tests cannot fully cover.

## Pre-requisites

- A running Charon instance with certificates in various states:
  - At least one expired Let's Encrypt certificate **not** attached to a proxy host
  - At least one custom (uploaded) certificate **not** attached to a proxy host
  - At least one certificate **attached** to a proxy host (in use)
  - At least one valid (non-expired) Let's Encrypt production certificate not in use
- Access to the Charon Certificates page

## Test Cases

### Happy Path

- [ ] **Delete expired LE cert not in use**: Click the delete button on an expired Let's Encrypt certificate that is not attached to any proxy host. Confirm in the dialog. Certificate disappears from the list and a success toast appears.
- [ ] **Delete custom cert not in use**: Click the delete button on an uploaded custom certificate not attached to any host. Confirm. Certificate is removed with a success toast.
- [ ] **Delete staging cert not in use**: Click the delete button on a staging certificate not attached to any host. Confirm. Certificate is removed with a success toast.

### Delete Prevention

- [ ] **In-use cert shows disabled button**: Find a certificate attached to a proxy host. Verify the delete button is visible but disabled.
- [ ] **In-use cert tooltip**: Hover over the disabled delete button. A tooltip should explain that the certificate is in use and cannot be deleted.
- [ ] **Valid LE cert hides delete button**: Find a valid (non-expired) Let's Encrypt production certificate not attached to any host. Verify no delete button is shown — Charon manages these automatically.

### Confirmation Dialog

- [ ] **Cancel does not delete**: Click the delete button on a deletable certificate. In the confirmation dialog, click Cancel. The certificate should remain in the list.
- [ ] **Escape key closes dialog**: Open the confirmation dialog. Press Escape. The dialog closes and the certificate remains.
- [ ] **Click overlay closes dialog**: Open the confirmation dialog. Click outside the dialog (on the overlay). The dialog closes and the certificate remains.
- [ ] **Confirm deletes**: Open the confirmation dialog. Click the Delete/Confirm button. The certificate is removed and a success toast appears.

### Keyboard Navigation

- [ ] **Tab through dialog**: Open the confirmation dialog. Press Tab to move focus between the Cancel and Delete buttons. Focus order should be logical (Cancel → Delete or Delete → Cancel).
- [ ] **Enter activates focused button**: Tab to the Cancel button and press Enter — dialog closes, certificate remains. Repeat with the Delete button — certificate is removed.
- [ ] **Focus trap**: With the dialog open, Tab should cycle within the dialog and not escape to the page behind it.

### Edge Cases & Race Conditions

- [ ] **Rapid double-click on delete**: Quickly double-click the delete button. Only one confirmation dialog should appear. Only one delete request should be sent.
- [ ] **Cert becomes in-use between dialog open and confirm**: Open the delete dialog for a certificate. In another tab, attach that certificate to a proxy host. Return and confirm deletion. The server should return a 409 error and the UI should show an appropriate error message — the certificate should remain.
- [ ] **Delete when backup may fail (low disk space)**: If testable, simulate low disk space. Attempt a deletion. The server creates a backup before deleting — verify the error is surfaced to the user if the backup fails.
- [ ] **Network error during delete**: Open the delete dialog and disconnect from the network (or throttle to offline in DevTools). Confirm deletion. An error message should appear and the certificate should remain.

### Visual & UX Consistency

- [ ] **Dialog styling**: The confirmation dialog should match the application theme (dark/light mode).
- [ ] **Toast messages**: Success and error toasts should appear in the expected position and auto-dismiss.
- [ ] **List updates without full reload**: After a successful deletion, the certificate list should update without requiring a page refresh.

## Related

- [Automatic HTTPS Certificates](../features/ssl-certificates.md)
