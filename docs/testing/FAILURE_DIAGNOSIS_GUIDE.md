# E2E Test Failure Diagnosis Guide

This guide explains how to use the comprehensive debugging infrastructure to diagnose the 11 failed tests from the latest E2E run.

## Quick Access Tools

### 1. **Playwright HTML Report** (Visual Analysis)
```bash
# When tests complete, open the report
npx playwright show-report

# Or start the server on a custom port
npx playwright show-report --port 9323
```

**What to look for:**
- Click on each failed test
- View the trace timeline (shows each action, network request, assertion)
- Check the video recording to see exactly what went wrong
- Read the assertion error message
- Check browser console logs

### 2. **Debug Logger CSV Export** (Network Analysis)
```bash
# After tests complete, check for network logs in test-results
find test-results -name "*.csv" -type f
```

**What to look for:**
- HTTP requests that failed or timed out
- Slow network operations (>1000ms)
- Authentication failures (401/403)
- API response errors

### 3. **Trace Files** (Step-by-Step Replay)
```bash
# View detailed trace for a failed test
npx playwright show-trace test-results/[test-name]/trace.zip
```

**Features:**
- Pause and step through each action
- Inspect DOM at any point
- Review network timing
- Check locator matching

### 4. **Video Recordings** (Visual Feedback Loop)
- Located in: `test-results/.playwright-artifacts-1/`
- Map filenames to test names in Playwright report
- Watch to understand timing and UI state when failure occurred

## The 11 Failures: What to Investigate

Based on the summary showing "other" category failures, these issues likely fall into:

### Category A: Timing/Flakiness Issues
- Tests intermittently fail due to timeouts
- Elements not appearing in expected timeframe
- **Diagnosis**: Check videos for loading spinners, network delays
- **Fix**: Increase timeout or add wait for specific condition

### Category B: Locator Issues
- Selectors matching wrong elements or multiple elements
- Elements appearing in different UI states
- **Diagnosis**: Check traces to see selector matching logic
- **Fix**: Make selectors more specific or use role-based locators

### Category C: State/Data Issues
- Form data not persisting
- Navigation not working correctly
- **Diagnosis**: Check network logs for API failures
- **Fix**: Add wait for API completion, verify mock data

### Category D: Accessibility/Keyboard Navigation
- Keyboard events not triggering actions
- Focus not moving as expected
- **Diagnosis**: Review traces for keyboard action handling
- **Fix**: Verify component keyboard event handlers

## Step-by-Step Failure Analysis Process

### For Each Failed Test:

1. **Get Test Name**
   - Open Playwright report
   - Find test in "Failed" section
   - Note the test file + test name

2. **View the Trace**
   ```bash
   npx playwright show-trace test-results/[test-name-hash]/trace.zip
   ```
   - Go through each step
   - Note which step failed and why
   - Check the actual error message

3. **Check Network Activity**
   - In trace, click "Network" tab
   - Look for failed requests (red entries)
   - Check response status and timing

4. **Review Video**
   - Watch the video recording
   - Observe what the user would see
   - Note UI state when failure occurred
   - Check for loading states, spinners, dialogs

5. **Analyze Debug Logs**
   - Check console output in trace
   - Look for our custom debug logger messages
   - Note timing information
   - Check for error context

### Debug Logger Output Format

Our debug logger outputs structured messages like:

```
✅ Step "Navigate to certificates page" completed [234ms]
  ├─ POST /api/certificates/list [200] 45ms
  ├─ Locator matched "getByRole('table')" [12ms]
  └─ Assert: Table visible passed [8ms]

❌ Step "Fill form with valid data" FAILED [5000ms+]
  ├─ Input focused but value not set?
  └─ Error: Assertion timeout after 5000ms
```

## Common Failure Patterns & Solutions

### Pattern 1: "Timeout waiting for locator"
**Cause**: Element not appearing within timeout
**Diagnosis**:
- Check video - is the page still loading?
- Check network tab - any pending requests?
- Check DOM snapshot - does element exist but hidden?

**Solution**:
- Add `await page.waitForLoadState('networkidle')`
- Use more robust locators (role-based instead of ID)
- Increase timeout if it's a legitimate slow operation

### Pattern 2: "Assertion failed: expect(locator).toBeDisabled()"
**Cause**: Button not in expected state
**Diagnosis**:
- Check trace - what's the button's actual state?
- Check console - any JS errors?
- Check network - is a form submission in progress?

**Solution**:
- Add explicit wait: `await expect(button).toBeDisabled({timeout: 10000})`
- Wait for preceding action: `await page.getByRole('button').click(); await page.waitForLoadState()`
- Check form library state

### Pattern 3: "Strict mode violation: multiple elements found"
**Cause**: Selector matches 2+ elements
**Diagnosis**:
- Check trace DOM snapshots - count matching elements
- Check test file - is selector too broad?

**Solution**:
- Scope to container: `page.getByRole('dialog').getByRole('button', {name: 'Save'})`
- Use .first() or .nth(0): `getByRole('button').first()`
- Make selector more specific

### Pattern 4: "Element not found by getByRole(...)"
**Cause**: Accessibility attributes missing
**Diagnosis**:
- Check DOM in trace - what tags/attributes exist?
- Is it missing role attribute?
- Is aria-label/aria-labelledby correct?

**Solution**:
- Add role attribute to element
- Add accessible name (aria-label, aria-labelledby, or text content)
- Use more forgiving selectors temporarily to confirm

### Pattern 5: "Test timed out after 30000ms"
**Cause**: Test execution exceeded timeout
**Diagnosis**:
- Check videos - where did it hang?
- Check traces - last action before timeout?
- Check network - any concurrent long-running requests?

**Solution**:
- Break test into smaller steps
- Add explicit waits between actions
- Check for infinite loops or blocking operations
- Increase test timeout if operation is legitimately slow

## Using the Debug Report for Triage

After tests complete, the custom debug reporter provides:

```
⏱️  Slow Tests (>5s):
────────────────────────────────────────────────────────────
1. should show user status badges           16.25s
2. should resend invite for pending user    12.61s
...

🔍 Failure Analysis by Type:
────────────────────────────────────────────────────────────
timeout      │ ████░░░░░░░░░░░░░░░░ 4/11 (36%)
assertion    │ ███░░░░░░░░░░░░░░░░░ 3/11 (27%)
locator      │ ██░░░░░░░░░░░░░░░░░░ 2/11 (18%)
other        │ ██░░░░░░░░░░░░░░░░░░ 2/11 (18%)
```

**Key insights:**
- **Timeout**: Look for network delays or missing waits
- **Assertion**: Check state management and form validation
- **Locator**: Focus on selector robustness
- **Other**: Check for exceptions or edge cases

## Advanced Debugging Techniques

### 1. Run Single Failed Test Locally
```bash
# Get exact test name from report, then:
npx playwright test --grep "should show user status badges"

# With full debug output:
DEBUG=charon:* npx playwright test --grep "should show user status badges" --debug
```

### 2. Inspect Network Logs CSV
```bash
# Convert CSV to readable format
column -t -s',' tests/network-logs.csv | less

# Or analyze in Excel/Google Sheets
```

### 3. Compare Videos Side-by-Side
- Download videos from test-results/.playwright-artifacts-1/
- Open in VLC with playlist
- Play at 2x speed to spot behavior differences

### 4. Check Browser Console
- In trace player, click "Console" tab
- Look for JS errors or warnings
- Check for 404/500 API responses in network tab

### 5. Reproduce Locally with Same Conditions
```bash
# Use the exact same seed (if randomization is involved)
SEED=12345 npx playwright test --grep "failing-test"

# With extended timeout for investigation
npx playwright test --grep "failing-test" --project=chromium --debug
```

## Docker-Specific Debugging

If tests pass locally but fail in CI Docker container:

### Check Container Logs
```bash
# View Docker container output
docker compose -f .docker/compose/docker-compose.test.yml logs charon

# Check for errors during startup
docker compose logs --tail=50
```

### Compare Environments
- Docker: Running on 0.0.0.0:8080
- Local: Running on localhost:8080/http://127.0.0.1:8080
- **Check**: Are there IPv4/IPv6 differences?
- **Check**: Are there DNS resolution issues?

### Port Accessibility
```bash
# From inside Docker, check if ports are accessible
docker exec charon curl -v http://localhost:8080
docker exec charon curl -v http://localhost:2019
docker exec charon curl -v http://localhost:2020
```

## Escalation Path

### When to Investigate Code
- Same tests fail consistently (not flaky)
- Error message points to specific feature
- Video shows incorrect behavior
- Network logs show API failures

**Action**: Fix the code/feature being tested

### When to Improve Test
- Tests flaky (fail 1 in 5 times)
- Timeout errors on slow operations
- Intermittent locator matching issues
- **Action**: Add waits, use more robust selectors, increase timeouts

### When to Update Test Infrastructure
- Port/networking issues
- Authentication failures
- Global setup incomplete
- **Action**: Check docker-compose, test fixtures, environment variables

## Next Steps

1. **Wait for Test Completion** (~6 minutes)
2. **Open Playwright Report** `npx playwright show-report`
3. **Identify Failure Categories** (timeout, assertion, locator, other)
4. **Run Single Test Locally** with debug output
5. **Review Traces & Videos** to understand exact failure point
6. **Apply Appropriate Fix** (code, test, or infrastructure)
7. **Re-run Tests** to validate fix

---

**Remember**: With the new debugging infrastructure, you have complete visibility into every action the browser took, every network request made, and every assertion evaluated. Use the traces to understand not just WHAT failed, but WHY it failed.
