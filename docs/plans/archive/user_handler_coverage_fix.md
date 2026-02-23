# User Handler Coverage Fix Plan

## Current State

**File:** `backend/internal/api/handlers/user_handler.go`
**Current Coverage:** 66.67% patch coverage (Codecov report)
**Target Coverage:** 100% of new/changed lines, minimum 85% overall
**Missing Coverage:** 3 lines (2 missing, 1 partial)

## Coverage Analysis

Based on the coverage report analysis, the following functions have gaps:

### Functions with Incomplete Coverage

1. **PreviewInviteURL** - 0.0% coverage
   - **Lines:** 509-543
   - **Status:** Completely untested
   - **Route:** `POST /api/v1/users/preview-invite-url` (protected, admin-only)

2. **getAppName** - 0.0% coverage
   - **Lines:** 547-552
   - **Status:** Helper function, completely untested
   - **Usage:** Called internally by InviteUser

3. **generateSecureToken** - 75.0% coverage (1 line missing)
   - **Lines:** 386-390
   - **Missing:** Error path when `rand.Read()` fails
   - **Current Test:** TestGenerateSecureToken only tests success path

4. **Setup** - 76.9% coverage
   - **Lines:** 74-136
   - **Partial Coverage:** Some error paths not fully tested

5. **CreateUser** - 75.7% coverage
   - **Lines:** 295-384
   - **Partial Coverage:** Some error scenarios not covered

6. **InviteUser** - 74.5% coverage
   - **Lines:** 395-501
   - **Partial Coverage:** Some edge cases and error paths missing

7. **UpdateUser** - 75.0% coverage
   - **Lines:** 608-668
   - **Partial Coverage:** Email conflict error path may be missing

8. **UpdateProfile** - 87.1% coverage
   - **Lines:** 192-247
   - **Partial Coverage:** Database error paths not fully covered

9. **AcceptInvite** - 81.8% coverage
   - **Lines:** 814-862
   - **Partial Coverage:** Some error scenarios not tested

## Detailed Test Plan

### Priority 1: Zero Coverage Functions

#### 1.1 PreviewInviteURL Tests

**Function Purpose:** Returns a preview of what an invite URL would look like with current settings

**Test File:** `backend/internal/api/handlers/user_handler_test.go`

**Test Cases to Add:**

```go
func TestUserHandler_PreviewInviteURL_NonAdmin(t *testing.T)
```

- **Setup:** User with "user" role
- **Action:** POST /users/preview-invite-url
- **Expected:** HTTP 403 Forbidden
- **Assertion:** Error message "Admin access required"

```go
func TestUserHandler_PreviewInviteURL_InvalidJSON(t *testing.T)
```

- **Setup:** Admin user
- **Action:** POST with invalid JSON body
- **Expected:** HTTP 400 Bad Request
- **Assertion:** Binding error message

```go
func TestUserHandler_PreviewInviteURL_Success_Unconfigured(t *testing.T)
```

- **Setup:** Admin user, no app.public_url setting
- **Action:** POST with valid email
- **Expected:** HTTP 200 OK
- **Assertions:**
  - `is_configured` is false
  - `warning` is true
  - `warning_message` contains "not configured"
  - `preview_url` contains fallback URL

```go
func TestUserHandler_PreviewInviteURL_Success_Configured(t *testing.T)
```

- **Setup:** Admin user, app.public_url setting exists
- **Action:** POST with valid email
- **Expected:** HTTP 200 OK
- **Assertions:**
  - `is_configured` is true
  - `warning` is false
  - `preview_url` contains configured URL
  - `base_url` matches configured setting

**Mock Requirements:**

- Need to create Setting model with key "app.public_url"
- Test both with and without configured URL

---

#### 1.2 getAppName Tests

**Function Purpose:** Helper function to retrieve app name from settings

**Test File:** `backend/internal/api/handlers/user_handler_test.go`

**Test Cases to Add:**

```go
func TestGetAppName_Default(t *testing.T)
```

- **Setup:** Empty database
- **Action:** Call getAppName(db)
- **Expected:** Returns "Charon"

```go
func TestGetAppName_FromSettings(t *testing.T)
```

- **Setup:** Create Setting with key "app_name", value "MyCustomApp"
- **Action:** Call getAppName(db)
- **Expected:** Returns "MyCustomApp"

```go
func TestGetAppName_EmptyValue(t *testing.T)
```

- **Setup:** Create Setting with key "app_name", empty value
- **Action:** Call getAppName(db)
- **Expected:** Returns "Charon" (fallback)

**Mock Requirements:**

- Models.Setting with key "app_name"

---

### Priority 2: Partial Coverage - Error Paths

#### 2.1 generateSecureToken Error Path

**Current Coverage:** 75% (missing error branch)

**Test Case to Add:**

```go
func TestGenerateSecureToken_ReadError(t *testing.T)
```

- **Challenge:** `crypto/rand.Read()` rarely fails in normal conditions
- **Approach:** This is difficult to test without mocking the rand.Reader
- **Alternative:** Document that this error path is for catastrophic system failure
- **Decision:** Consider this acceptable uncovered code OR refactor to accept an io.Reader for dependency injection

**Recommendation:** Accept this as untestable without significant refactoring. Document with comment.

---

#### 2.2 Setup Database Error Paths

**Current Coverage:** 76.9%

**Missing Test Cases:**

```go
func TestUserHandler_Setup_TransactionFailure(t *testing.T)
```

- **Setup:** Mock DB transaction failure
- **Action:** POST /setup with valid data
- **Challenge:** SQLite doesn't easily simulate transaction failures
- **Alternative:** Test with closed DB connection or dropped table

```go
func TestUserHandler_Setup_PasswordHashError(t *testing.T)
```

- **Setup:** Valid request but password hashing fails
- **Challenge:** bcrypt.GenerateFromPassword rarely fails
- **Decision:** May be acceptable uncovered code

**Recommendation:** Focus on testable paths; document hard-to-test error scenarios.

---

#### 2.3 CreateUser Missing Scenarios

**Current Coverage:** 75.7%

**Test Cases to Add:**

```go
func TestUserHandler_CreateUser_PasswordHashError(t *testing.T)
```

- **Setup:** Valid request
- **Action:** Attempt to create user with password that causes hash failure
- **Challenge:** Hard to trigger without mocking
- **Decision:** Document as edge case

```go
func TestUserHandler_CreateUser_DatabaseCheckError(t *testing.T)
```

- **Setup:** Drop users table before email check
- **Action:** POST /users
- **Expected:** HTTP 500 "Failed to check email"

```go
func TestUserHandler_CreateUser_AssociationError(t *testing.T)
```

- **Setup:** Valid permitted_hosts with non-existent host IDs
- **Action:** POST /users with invalid host IDs
- **Expected:** Transaction should fail or hosts should be empty

---

#### 2.4 InviteUser Missing Scenarios

**Current Coverage:** 74.5%

**Test Cases to Add:**

```go
func TestUserHandler_InviteUser_TokenGenerationError(t *testing.T)
```

- **Challenge:** Hard to force crypto/rand failure
- **Decision:** Document as edge case

```go
func TestUserHandler_InviteUser_DisableUserError(t *testing.T)
```

- **Setup:** Create user, then cause Update to fail
- **Action:** POST /users/invite
- **Expected:** Transaction rollback

```go
func TestUserHandler_InviteUser_MailServiceConfigured(t *testing.T)
```

- **Setup:** Configure MailService with valid SMTP settings
- **Action:** POST /users/invite
- **Expected:** email_sent should be true (or handle SMTP error)
- **Challenge:** Requires SMTP mock or test server

---

#### 2.5 UpdateUser Email Conflict

**Current Coverage:** 75.0%

**Existing Test:** TestUserHandler_UpdateUser_Success covers basic update
**Potential Gap:** Email conflict check might not hit error path

**Test Case to Verify/Add:**

```go
func TestUserHandler_UpdateUser_EmailConflict(t *testing.T)
```

- **Setup:** Create two users
- **Action:** Try to update user1's email to user2's email
- **Expected:** HTTP 409 Conflict
- **Assertion:** Error message "Email already in use"

---

#### 2.6 UpdateProfile Database Errors

**Current Coverage:** 87.1%

**Test Cases to Add:**

```go
func TestUserHandler_UpdateProfile_EmailCheckError(t *testing.T)
```

- **Setup:** Valid user, drop table before email check
- **Action:** PUT /profile with new email
- **Expected:** HTTP 500 "Failed to check email availability"

```go
func TestUserHandler_UpdateProfile_UpdateError(t *testing.T)
```

- **Setup:** Valid user, close DB before update
- **Action:** PUT /profile
- **Expected:** HTTP 500 "Failed to update profile"

---

#### 2.7 AcceptInvite Missing Coverage

**Current Coverage:** 81.8%

**Existing Tests Cover:**

- Invalid JSON
- Invalid token
- Expired token (with status update)
- Already accepted
- Success

**Potential Gap:** SetPassword error path

**Test Case to Consider:**

```go
func TestUserHandler_AcceptInvite_PasswordHashError(t *testing.T)
```

- **Challenge:** Hard to trigger bcrypt failure
- **Decision:** Document as edge case

---

### Priority 3: Boundary Conditions and Edge Cases

#### 3.1 Email Normalization

**Test Cases to Add:**

```go
func TestUserHandler_CreateUser_EmailNormalization(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Create user with email "<User@Example.COM>"
- **Expected:** Email stored as "<user@example.com>"

```go
func TestUserHandler_InviteUser_EmailNormalization(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Invite user with mixed-case email
- **Expected:** Email stored lowercase

---

#### 3.2 Permission Mode Defaults

**Test Cases to Verify:**

```go
func TestUserHandler_CreateUser_DefaultPermissionMode(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Create user without specifying permission_mode
- **Expected:** permission_mode defaults to "allow_all"

```go
func TestUserHandler_InviteUser_DefaultPermissionMode(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Invite user without specifying permission_mode
- **Expected:** permission_mode defaults to "allow_all"

---

#### 3.3 Role Defaults

**Test Cases to Verify:**

```go
func TestUserHandler_CreateUser_DefaultRole(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Create user without specifying role
- **Expected:** role defaults to "user"

```go
func TestUserHandler_InviteUser_DefaultRole(t *testing.T)
```

- **Setup:** Admin user
- **Action:** Invite user without specifying role
- **Expected:** role defaults to "user"

---

### Priority 4: Integration Scenarios

#### 4.1 Permitted Hosts Edge Cases

**Test Cases to Add:**

```go
func TestUserHandler_CreateUser_EmptyPermittedHosts(t *testing.T)
```

- **Setup:** Admin, permission_mode "deny_all", empty permitted_hosts
- **Action:** Create user
- **Expected:** User created with deny_all mode, no permitted hosts

```go
func TestUserHandler_CreateUser_NonExistentPermittedHosts(t *testing.T)
```

- **Setup:** Admin, permission_mode "deny_all", non-existent host IDs [999, 1000]
- **Action:** Create user
- **Expected:** User created but no hosts associated (or error)

---

## Implementation Strategy

### Phase 1: Add Missing Tests (Priority 1)

1. Implement PreviewInviteURL test suite (4 tests)
2. Implement getAppName test suite (3 tests)
3. Run coverage and verify these reach 100%

**Expected Impact:** +7 test cases, ~35 lines of untested code covered

### Phase 2: Error Path Coverage (Priority 2)

1. Add database error simulation tests where feasible
2. Document hard-to-test error paths with code comments
3. Focus on testable scenarios (table drops, closed connections)

**Expected Impact:** +8-10 test cases, improved error path coverage

### Phase 3: Edge Cases and Defaults (Priority 3)

1. Add email normalization tests
2. Add default value tests
3. Verify role and permission defaults

**Expected Impact:** +6 test cases, better validation of business logic

### Phase 4: Integration Edge Cases (Priority 4)

1. Add permitted hosts edge case tests
2. Test association behavior with invalid data

**Expected Impact:** +2 test cases, comprehensive coverage

---

## Success Criteria

### Minimum Requirements (85% Coverage)

- [ ] PreviewInviteURL: 100% coverage (4 tests)
- [ ] getAppName: 100% coverage (3 tests)
- [ ] generateSecureToken: 100% or documented as untestable
- [ ] All other functions: ≥85% coverage
- [ ] Total user_handler.go coverage: ≥85%

### Stretch Goal (100% Coverage)

- [ ] All testable code paths covered
- [ ] Untestable code paths documented with `// Coverage: Untestable without mocking` comments
- [ ] All error paths tested or documented
- [ ] All edge cases and boundary conditions tested

---

## Test Execution Plan

### Step 1: Run Baseline Coverage

```bash
cd backend
go test -coverprofile=baseline_coverage.txt -run "TestUserHandler" ./internal/api/handlers
go tool cover -func=baseline_coverage.txt | grep user_handler.go
```

### Step 2: Implement Priority 1 Tests

- Add PreviewInviteURL tests
- Add getAppName tests
- Run coverage and verify improvement

### Step 3: Iterate Through Priorities

- Implement each priority group
- Run coverage after each group
- Adjust plan based on results

### Step 4: Final Coverage Report

```bash
go test -coverprofile=final_coverage.txt -run "TestUserHandler" ./internal/api/handlers
go tool cover -func=final_coverage.txt | grep user_handler.go
go tool cover -html=final_coverage.txt -o user_handler_coverage.html
```

### Step 5: Validate Against Codecov

- Push changes to branch
- Verify Codecov report shows ≥85% patch coverage
- Verify no coverage regressions

---

## Mock and Setup Requirements

### Database Models

- `models.User`
- `models.Setting`
- `models.ProxyHost`

### Test Helpers

- `setupUserHandler(t)` - Creates test DB with User and Setting tables
- `setupUserHandlerWithProxyHosts(t)` - Includes ProxyHost table
- Admin middleware mock: `c.Set("role", "admin")`
- User ID middleware mock: `c.Set("userID", uint(1))`

### Additional Mocks Needed

- SMTP server mock for email testing (optional, can verify email_sent=false)
- Settings helper for creating app.public_url and app_name settings

---

## Testing Best Practices to Follow

1. **Use Table-Driven Tests:** For similar scenarios with different inputs
2. **Descriptive Test Names:** Follow pattern `TestUserHandler_Function_Scenario`
3. **Arrange-Act-Assert:** Clear separation of setup, action, and verification
4. **Unique Database Names:** Use `t.Name()` in DB connection string
5. **Test Isolation:** Each test should be independent
6. **Assertions:** Use `testify/assert` for clear failure messages
7. **Error Messages:** Verify exact error messages, not just status codes

---

## Code Style Consistency

### Existing Patterns to Maintain

- Use `gin.SetMode(gin.TestMode)` at test start
- Use `httptest.NewRecorder()` for response capture
- Marshal request bodies with `json.Marshal()`
- Set `Content-Type: application/json` header
- Use `require.NoError()` for setup failures
- Use `assert.Equal()` for assertions

### Test Organization

- Group related tests with `t.Run()` when appropriate
- Keep tests in same file as existing tests
- Use clear comments for complex setup

---

## Acceptance Checklist

- [ ] All Priority 1 tests implemented and passing
- [ ] All Priority 2 testable scenarios implemented
- [ ] All Priority 3 edge cases tested
- [ ] Coverage report shows ≥85% for user_handler.go
- [ ] No existing tests broken
- [ ] All new tests follow existing patterns
- [ ] Code review checklist satisfied
- [ ] Codecov patch coverage ≥85%
- [ ] Documentation updated if needed

---

## Notes and Considerations

### Hard-to-Test Scenarios

1. **crypto/rand.Read() failure:** Extremely rare, requires system-level failure
2. **bcrypt password hashing failure:** Rare, usually only with invalid cost
3. **SMTP email sending:** Requires mock server or test credentials

### Recommendations

- Document untestable error paths with comments
- Focus test effort on realistic failure scenarios
- Use table drops and closed connections for DB errors
- Consider refactoring hard-to-test code if coverage is critical

### Future Improvements

- Consider dependency injection for crypto/rand and bcrypt
- Add integration tests with real SMTP mock server
- Add performance tests for password hashing
- Consider property-based testing for email normalization

---

## Related Files

- **Handler:** `backend/internal/api/handlers/user_handler.go`
- **Tests:** `backend/internal/api/handlers/user_handler_test.go`
- **Routes:** `backend/internal/api/routes/routes.go`
- **Models:** `backend/internal/models/user.go`
- **Utils:** `backend/internal/utils/url.go`

---

## Timeline Estimate

- **Phase 1 (Priority 1):** 2-3 hours
- **Phase 2 (Priority 2):** 3-4 hours
- **Phase 3 (Priority 3):** 1-2 hours
- **Phase 4 (Priority 4):** 1 hour
- **Total:** 7-10 hours

---

*Plan created: 2025-12-23*
*Target completion: Before merge to main branch*
*Owner: Development team*
