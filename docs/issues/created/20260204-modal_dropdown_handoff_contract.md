# Modal Dropdown Fix - Local Environment Handoff Contract

**Date**: 2026-02-04
**Status**: Implementation Complete - Testing Required
**Environment**: Codespace → Local Development Environment

---

## IMPLEMENTATION COMPLETED ✅

### Frontend Changes Made
All 7 P0 critical modal components have been updated with the 3-layer modal architecture:

1. ✅ **ProxyHostForm.tsx** - ACL selector, Security Headers dropdowns fixed
2. ✅ **UsersPage.tsx** - InviteUserModal role/permission dropdowns fixed
3. ✅ **UsersPage.tsx** - EditPermissionsModal dropdowns fixed
4. ✅ **Uptime.tsx** - CreateMonitorModal & EditMonitorModal type dropdowns fixed
5. ✅ **RemoteServerForm.tsx** - Provider dropdown fixed
6. ✅ **CrowdSecConfig.tsx** - BanIPModal duration dropdown fixed

### Technical Changes Applied
- **3-Layer Modal Pattern**: Separated overlay (z-40) / container (z-50) / content (pointer-events-auto)
- **DOM Restructuring**: Split single overlay div into proper layered architecture
- **Event Handling**: Preserved modal close behavior (backdrop click, ESC key)
- **CSS Classes**: Added `pointer-events-none/auto` for proper interaction handling

---

## LOCAL ENVIRONMENT TESTING REQUIRED 🧪

### Prerequisites for Testing
```bash
# Required for E2E testing
docker --version          # Must be available
docker-compose --version  # Must be available
node --version            # v18+ required
npm --version             # Latest stable
```

### Step 1: Environment Setup
```bash
# 1. Switch to local environment
cd /path/to/charon

# 2. Ensure on correct branch
git checkout feature/beta-release
git pull origin feature/beta-release

# 3. Install dependencies
npm install
cd frontend && npm install && cd ..

# 4. Build frontend
cd frontend && npm run build && cd ..
```

### Step 2: Start E2E Environment
```bash
# CRITICAL: Rebuild E2E container with new code
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# OR manual rebuild if skill script unavailable:
docker-compose -f .docker/compose/docker-compose.yml down
docker-compose -f .docker/compose/docker-compose.yml build --no-cache
docker-compose -f .docker/compose/docker-compose.yml up -d
```

### Step 3: Manual Testing (30-45 minutes)

#### Test Each Modal Component

**A. ProxyHostForm (Priority 1)**
```bash
# Navigate to: http://localhost:8080/proxy-hosts
# 1. Click "Add Proxy Host"
# 2. Test ACL dropdown - should open and allow selection
# 3. Test Security Headers dropdown - should open and allow selection
# 4. Fill form and submit - should work normally
# 5. Edit existing proxy host - repeat dropdown tests
```

**B. User Management Modals**
```bash
# Navigate to: http://localhost:8080/users
# 1. Click "Invite User"
# 2. Test Role dropdown (User/Admin) - should work
# 3. Test Permission Mode dropdown - should work
# 4. Click existing user "Edit Permissions"
# 5. Test permission dropdowns - should work
```

**C. Uptime Monitor Modals**
```bash
# Navigate to: http://localhost:8080/uptime
# 1. Click "Create Monitor"
# 2. Test Monitor Type dropdown (HTTP/TCP) - should work
# 3. Save monitor, then click "Configure"
# 4. Test Monitor Type dropdown in edit mode - should work
```

**D. Remote Servers**
```bash
# Navigate to: http://localhost:8080/remote-servers
# 1. Click "Add Server"
# 2. Test Provider dropdown (Generic/Docker/Kubernetes) - should work
```

**E. CrowdSec IP Bans**
```bash
# Navigate to: http://localhost:8080/security/crowdsec
# 1. Click "Ban IP"
# 2. Test Duration dropdown - should work and allow selection
```

### Step 4: Automated E2E Testing
```bash
# MUST run after manual testing confirms dropdowns work

# 1. Test proxy host ACL integration (primary test case)
npx playwright test tests/integration/proxy-acl-integration.spec.ts --project=chromium

# 2. Run full E2E suite
npx playwright test --project=chromium --project=firefox --project=webkit

# 3. Check for specific dropdown-related failures
npx playwright test --grep "dropdown|select|acl|security.headers" --project=chromium
```

### Step 5: Cross-Browser Verification
```bash
# Test in each browser for compatibility
npx playwright test tests/integration/proxy-acl-integration.spec.ts --project=chromium
npx playwright test tests/integration/proxy-acl-integration.spec.ts --project=firefox
npx playwright test tests/integration/proxy-acl-integration.spec.ts --project=webkit
```

---

## SUCCESS CRITERIA ✅

### Must Pass Before Merge
- [ ] **All 7 modal dropdowns** open and allow selection
- [ ] **Modal close behavior** works (backdrop click, ESC key)
- [ ] **Form submission** works with selected dropdown values
- [ ] **E2E tests pass** - especially proxy-acl-integration.spec.ts
- [ ] **Cross-browser compatibility** (Chrome, Firefox, Safari)
- [ ] **No console errors** in browser dev tools
- [ ] **No TypeScript errors** - `npm run type-check` passes

### Verification Commands
```bash
# Frontend type check
cd frontend && npm run type-check

# Backend tests (should be unaffected)
cd backend && go test ./...

# Full test suite
npm test
```

---

## ROLLBACK PLAN 🔄

If any issues are discovered:

```bash
# Quick rollback - revert all modal changes
git log --oneline -5                    # Find modal fix commit hash
git revert <commit-hash>                # Revert the modal changes
git push origin feature/beta-release    # Push rollback

# Test rollback worked
npx playwright test tests/integration/proxy-acl-integration.spec.ts --project=chromium
```

---

## EXPECTED ISSUES & SOLUTIONS 🔧

### Issue: E2E Container Won't Start
```bash
# Solution: Clean rebuild
docker-compose down -v
docker system prune -f
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
```

### Issue: Frontend Build Fails
```bash
# Solution: Clean install
cd frontend
rm -rf node_modules package-lock.json
npm install
npm run build
```

### Issue: Tests Still Fail
```bash
# Solution: Check if environment variables are set
cat .env | grep -E "(EMERGENCY|ENCRYPTION)"
# Should show EMERGENCY_TOKEN and ENCRYPTION_KEY
```

---

## COMMIT MESSAGE TEMPLATE 📝

When testing is complete and successful:

```
fix: resolve modal dropdown z-index conflicts across application

Restructure 7 modal components to use 3-layer architecture preventing
native select dropdown menus from being blocked by modal overlays.

Components fixed:
- ProxyHostForm: ACL selector and Security Headers dropdowns
- User management: Role and permission mode selection
- Uptime monitors: Monitor type selection (HTTP/TCP)
- Remote servers: Provider selection dropdown
- CrowdSec: IP ban duration selection

The fix separates modal background overlay (z-40) from form container
(z-50) and enables pointer events only on form content, allowing
native dropdown menus to render above all modal layers.

Resolves user inability to select security policies, user roles,
monitor types, and other critical configuration options through
the UI interface.
```

---

## QA REQUIREMENTS 📋

### Definition of Done
- [ ] Manual testing completed for all 7 components
- [ ] All E2E tests passing
- [ ] Cross-browser verification complete
- [ ] No console errors or TypeScript issues
- [ ] Code review approved (if applicable)
- [ ] Commit message follows conventional format

### Documentation Updates
- [ ] Update component documentation if modal patterns changed
- [ ] Add note to design system about correct modal z-index patterns
- [ ] Consider adding ESLint rule to catch future modal z-index anti-patterns

---

**🎯 READY FOR LOCAL ENVIRONMENT TESTING**

All implementation work is complete. The modal dropdown z-index fix has been applied comprehensively across all 7 affected components. Testing in the local Docker environment will validate the fix works as designed.

**Next Actions**: Move to local environment, run the testing checklist above, and merge when all success criteria are met.
