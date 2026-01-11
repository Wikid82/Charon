# Current Specification

**Status**: 🔧 IN PROGRESS - Playwright MCP Server Initialization Fix
**Last Updated**: 2026-01-11
**Previous Work**: Staticcheck Pre-Commit Integration (COMPLETE - Archived)

---

## Active Project: Playwright MCP Server Initialization Failure

**Priority:** 🔴 HIGH
**Reported:** VS Code MCP Server Error Logs (Exit Code 1)
**Critical Requirement:** Configure VS Code MCP to properly start Playwright server without exit errors

### Problem Statement

The Playwright MCP server is failing to initialize with exit code 1. Error logs show:
```
2026-01-11 00:35:54.254 [info] Connection state: Error Process exited with code 1
```

The server outputs the Playwright help menu instead of starting properly, indicating it's not receiving proper configuration or arguments when launched by VS Code's MCP system.

### Root Cause

Investigation revealed the VS Code MCP configuration file is **empty** (0 bytes):
- **Location**: `/root/.config/Code - Insiders/User/mcp.json`
- **Size**: 0 bytes (created Dec 12 14:48)
- **Impact**: Without valid JSON configuration, VS Code cannot properly invoke the Playwright MCP server
- **Current Behavior**: Server receives invalid/missing arguments → shows help → exits with code 1

### Solution Approach

Create a properly formatted MCP configuration file that tells VS Code how to start the Playwright MCP server with correct arguments, working directory, and environment variables

### Solution Approach

Create a properly formatted MCP configuration file that tells VS Code how to start the Playwright MCP server with correct arguments, working directory, and environment variables.

### Implementation Plan

#### Phase 1: Create MCP Configuration File

**File**: `/root/.config/Code - Insiders/User/mcp.json` (currently empty)

**Task 1.1**: Write valid JSON configuration
```json
{
  "mcpServers": {
    "playwright-test": {
      "command": "npx",
      "args": [
        "playwright",
        "run-test-mcp-server",
        "--config",
        "playwright.config.js",
        "--headless"
      ],
      "env": {
        "NODE_ENV": "test",
        "CI": "false"
      }
    },
    "playwright-browser": {
      "command": "npx",
      "args": [
        "playwright",
        "run-mcp-server",
        "--browser",
        "chromium",
        "--headless"
      ],
      "env": {
        "NODE_ENV": "development"
      }
    }
  }
}
```

**Task 1.2**: Validate JSON syntax
- Command: `cat /root/.config/Code\ -\ Insiders/User/mcp.json | jq .`
- Expected: Valid JSON parsing (no errors)

**Task 1.3**: Verify file permissions
- Command: `ls -lah /root/.config/Code\ -\ Insiders/User/mcp.json`
- Expected: `-rw-r--r--` permissions, size > 0 bytes

#### Phase 2: Verify Playwright Dependencies

**Task 2.1**: Confirm Playwright installation
- Command: `npx playwright --version`
- Expected: `Version 1.57.0`
- Current Status: ✅ VERIFIED

**Task 2.2**: Install browser binaries (if needed)
- Command: `cd /projects/Charon && npx playwright install chromium`
- Expected: "chromium downloaded successfully" or "chromium is already installed"

**Task 2.3**: Verify Playwright config exists
- File: `/projects/Charon/playwright.config.js`
- Current Status: ✅ EXISTS (valid configuration)

#### Phase 3: Test MCP Server Startup (Manual)

**Task 3.1**: Test server command directly
- Command: `cd /projects/Charon && npx playwright run-test-mcp-server --config playwright.config.js --headless`
- Expected: Server starts and waits (no immediate exit)
- Expected: No help menu displayed
- Terminate: Press Ctrl+C to stop

**Task 3.2**: Verify process persistence
- Command: `ps aux | grep playwright`
- Expected: Process running while server is active
- Expected: No immediate exit after startup

#### Phase 4: Reload VS Code

**Task 4.1**: Reload window
- Action: Command Palette → "Developer: Reload Window"
- Rationale: VS Code reads MCP configuration at startup

**Alternative**: Restart VS Code completely

#### Phase 5: Verify MCP Server Connection

**Task 5.1**: Check VS Code Output panel
- View → Output → Select "MCP Servers" or "Playwright"
- Expected: Connection success message
- Expected: No "exit code 1" errors

**Task 5.2**: Verify server status
- Check: MCP server connection state in VS Code status bar
- Expected: "Connected" (not "Error")

**Task 5.3**: Test Playwright MCP tools
- Open: Copilot Chat
- Try: Playwright-related commands or tools
- Expected: Tools accessible and functional

### Success Criteria (Definition of Done)

1. [ ] `/root/.config/Code - Insiders/User/mcp.json` contains valid JSON (not empty)
2. [ ] `mcp.json` has `mcpServers` key with at least one Playwright server definition
3. [ ] File size > 0 bytes (verified with `ls -lah`)
4. [ ] JSON is valid (verified with `jq`)
5. [ ] Playwright version is 1.57.0 (verified with `npx playwright --version`)
6. [ ] Chromium browser binary is installed
7. [ ] Manual server start works: `npx playwright run-test-mcp-server --config playwright.config.js`
8. [ ] Server persists when started (doesn't exit immediately)
9. [ ] VS Code window has been reloaded after creating `mcp.json`
10. [ ] No "exit code 1" errors in VS Code MCP server logs
11. [ ] MCP server connection state is "Connected" (not "Error")
12. [ ] Playwright MCP tools are accessible in Copilot Chat

### Configuration Details

#### MCP Server Types

Two server configurations are recommended:

1. **playwright-test** (Primary - For Testing)
   - Command: `npx playwright run-test-mcp-server`
   - Uses: Project's `playwright.config.js`
   - Mode: Headless
   - Purpose: Test runner interactions, E2E test execution

2. **playwright-browser** (Secondary - For Browser Automation)
   - Command: `npx playwright run-mcp-server`
   - Browser: Chromium
   - Mode: Headless
   - Purpose: Direct browser automation tasks

#### Environment Variables

- **NODE_ENV**: Set to "test" for test server, "development" for browser server
- **CI**: Set to "false" to ensure interactive features work

#### Command Arguments

- `--config playwright.config.js`: Points to project's Playwright configuration
- `--headless`: Runs browser without GUI (required for server environments)
- `--browser chromium`: Specifies browser type for browser MCP server

### Verification Commands

```bash
# Check file exists and size
ls -lah /root/.config/Code\ -\ Insiders/User/mcp.json

# Validate JSON syntax
cat /root/.config/Code\ -\ Insiders/User/mcp.json | jq .

# Verify Playwright version
npx playwright --version

# Test manual server startup
cd /projects/Charon && npx playwright run-test-mcp-server --config playwright.config.js --headless

# Check running processes
ps aux | grep playwright
```

### Common Pitfalls to Avoid

1. **Invalid JSON**: No trailing commas, proper quote escaping
2. **Wrong Command Path**: Use `npx` not absolute paths
3. **Missing Config Reference**: Always specify `--config playwright.config.js`
4. **Forget to Reload**: VS Code must reload after config changes
5. **Browser Not Installed**: Run `npx playwright install chromium`

### Troubleshooting Guide

#### Issue: "Command not found"
**Solution**: `cd /projects/Charon && npm install`

#### Issue: "Browser not installed"
**Solution**: `npx playwright install --with-deps chromium`

#### Issue: Server still exits with code 1
**Diagnosis**:
```bash
# Check JSON validity
cat /root/.config/Code\ -\ Insiders/User/mcp.json | jq .
# If error: Fix JSON syntax errors
```

#### Issue: VS Code doesn't recognize config
**Solution**:
1. Verify file location is exact: `/root/.config/Code - Insiders/User/mcp.json`
2. Reload VS Code: Command Palette → "Developer: Reload Window"
3. Check Output panel for MCP logs

### Expected Outcomes

After successful implementation:

- ✅ Playwright MCP Server Status: **Running**
- ✅ Connection State: **Connected** (not Error)
- ✅ Exit Code: N/A (server persists)
- ✅ Available Tools: Playwright test execution, browser automation
- ✅ Copilot Integration: Playwright commands work in chat

### Performance Benchmarks

- **File Size**: ~400 bytes (minimal JSON config)
- **Startup Time**: < 5 seconds for server to be ready
- **Memory Usage**: ~50-100 MB per MCP server instance
- **Configuration Reload**: < 2 seconds after VS Code reload

### Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Invalid JSON syntax | HIGH | Use `jq` to validate before reload |
| Browser binaries missing | MEDIUM | Run `npx playwright install` first |
| VS Code doesn't reload config | LOW | Explicitly reload window via Command Palette |
| Wrong file path | HIGH | Double-check path matches VS Code Insiders location |
| Playwright not installed | MEDIUM | Verify with `npx playwright --version` |

### Implementation Timeline

1. **Create mcp.json**: 2 minutes
2. **Verify Playwright installation**: 2 minutes
3. **Test server startup**: 3 minutes
4. **Reload VS Code**: 1 minute
5. **Verify connection**: 2 minutes

**Total Estimated Time**: 10 minutes

### References

- **Playwright Documentation**: https://playwright.dev/docs/intro
- **Playwright MCP Commands**: /projects/Charon/node_modules/playwright/lib/program.js (lines 149-159)
- **VS Code MCP Configuration**: https://code.visualstudio.com/docs/copilot/customization
- **Project Playwright Config**: /projects/Charon/playwright.config.js
- **MCP Server Discovery**: Investigated node_modules/playwright/lib/mcp/ directory
- **Agent Instructions Reference**: .github/instructions/agents.instructions.md (lines 544-624)

### Next Steps

1. **Immediate**: Create `mcp.json` with recommended configuration
2. **Verify**: Test manual server startup to ensure it works
3. **Reload**: Restart VS Code to apply changes
4. **Validate**: Check MCP server connection status in Output panel
5. **Test**: Use Playwright MCP tools in Copilot Chat to confirm integration

---

## Alternative Configurations

### Minimal Configuration (Single Server)

If only one server is needed:

```json
{
  "mcpServers": {
    "playwright": {
      "command": "npx",
      "args": ["playwright", "run-test-mcp-server"]
    }
  }
}
```

### Advanced Configuration (Custom Port)

For specific port binding:

```json
{
  "mcpServers": {
    "playwright": {
      "command": "npx",
      "args": [
        "playwright",
        "run-test-mcp-server",
        "--config",
        "playwright.config.js",
        "--port",
        "9323",
        "--host",
        "localhost"
      ]
    }
  }
}
```

---

**Plan Complete - Ready for Implementation**

**Note**: This specification follows [Spec-Driven Workflow v1](.github/instructions/spec-driven-workflow-v1.instructions.md) format.
