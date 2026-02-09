#!/bin/bash
# QA Test Script: Certificate Page Authentication
# Tests authentication fixes for certificate endpoints

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
COOKIE_FILE="/tmp/charon-test-cookies.txt"
# Derive repository root dynamically so script works outside specific paths
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_RESULTS="$REPO_ROOT/test-results/qa-auth-test-results.log"

# Clear previous results
> "$TEST_RESULTS"
> "$COOKIE_FILE"

echo -e "${BLUE}=== QA Test: Certificate Page Authentication ===${NC}"
echo "Testing authentication fixes for certificate endpoints"
echo "Base URL: $BASE_URL"
echo ""

# Function to log test results
log_test() {
    local status=$1
    local test_name=$2
    local details=$3

    echo "[$status] $test_name" | tee -a "$TEST_RESULTS"
    if [ -n "$details" ]; then
        echo "  Details: $details" | tee -a "$TEST_RESULTS"
    fi
}

# Function to print section header
section() {
    echo -e "\n${BLUE}=== $1 ===${NC}\n"
    echo "=== $1 ===" >> "$TEST_RESULTS"
}

# Phase 1: Certificate Page Authentication Tests
section "Phase 1: Certificate Page Authentication Tests"

# Test 1.1: Login and Cookie Verification
echo -e "${YELLOW}Test 1.1: Login and Cookie Verification${NC}"
# First, ensure test user exists (idempotent)
curl -s -X POST "$API_URL/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"email":"qa-test@example.com","password":"QATestPass123!","name":"QA Test User"}' > /dev/null 2>&1

LOGIN_RESPONSE=$(curl -s -c "$COOKIE_FILE" -X POST "$API_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"qa-test@example.com","password":"QATestPass123!"}' \
    -w "\n%{http_code}")

HTTP_CODE=$(echo "$LOGIN_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$LOGIN_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Login successful" "HTTP $HTTP_CODE"

    # Check if auth_token cookie exists
    if grep -q "auth_token" "$COOKIE_FILE"; then
        log_test "PASS" "auth_token cookie created" ""

        # Extract cookie details
        COOKIE_LINE=$(grep "auth_token" "$COOKIE_FILE")
        echo "  Cookie details: $COOKIE_LINE" | tee -a "$TEST_RESULTS"

        # Note: HttpOnly and Secure flags are not visible in curl cookie file
        # These would need to be verified in browser DevTools
        log_test "INFO" "Cookie flags (HttpOnly, Secure, SameSite)" "Verify manually in browser DevTools"
    else
        log_test "FAIL" "auth_token cookie NOT created" "Cookie file: $COOKIE_FILE"
    fi
else
    log_test "FAIL" "Login failed" "HTTP $HTTP_CODE - $RESPONSE_BODY"
    exit 1
fi

# Test 1.2: Certificate List (GET /api/v1/certificates)
echo -e "\n${YELLOW}Test 1.2: Certificate List (GET /api/v1/certificates)${NC}"
LIST_RESPONSE=$(curl -s -b "$COOKIE_FILE" "$API_URL/certificates" -w "\n%{http_code}" -v 2>&1)
HTTP_CODE=$(echo "$LIST_RESPONSE" | grep "< HTTP" | awk '{print $3}')
RESPONSE_BODY=$(echo "$LIST_RESPONSE" | grep -v "^[<>*]" | sed '/^$/d' | tail -n +2)

echo "Response: $RESPONSE_BODY" | tee -a "$TEST_RESULTS"

if echo "$LIST_RESPONSE" | grep -q "Cookie: auth_token"; then
    log_test "PASS" "Request includes auth_token cookie" ""
else
    log_test "WARN" "Could not verify Cookie header in request" "Check manually in browser Network tab"
fi

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Certificate list request successful" "HTTP $HTTP_CODE"

    # Check if response is valid JSON array
    if echo "$RESPONSE_BODY" | jq -e 'type == "array"' > /dev/null 2>&1; then
        CERT_COUNT=$(echo "$RESPONSE_BODY" | jq 'length')
        log_test "PASS" "Response is valid JSON array" "Count: $CERT_COUNT certificates"
    else
        log_test "WARN" "Response is not a JSON array" ""
    fi
elif [ "$HTTP_CODE" = "401" ]; then
    log_test "FAIL" "Authentication failed - 401 Unauthorized" "Cookie not being sent or not valid"
    echo "Response body: $RESPONSE_BODY" | tee -a "$TEST_RESULTS"
else
    log_test "FAIL" "Certificate list request failed" "HTTP $HTTP_CODE"
fi

# Test 1.3: Certificate Upload (POST /api/v1/certificates)
echo -e "\n${YELLOW}Test 1.3: Certificate Upload (POST /api/v1/certificates)${NC}"

# Create test certificate and key
TEST_CERT_DIR="/tmp/charon-test-certs"
mkdir -p "$TEST_CERT_DIR"

# Generate self-signed certificate for testing
openssl req -x509 -newkey rsa:2048 -keyout "$TEST_CERT_DIR/test.key" -out "$TEST_CERT_DIR/test.crt" \
    -days 1 -nodes -subj "/CN=qa-test.local" 2>/dev/null

if [ -f "$TEST_CERT_DIR/test.crt" ] && [ -f "$TEST_CERT_DIR/test.key" ]; then
    log_test "INFO" "Test certificate generated" "$TEST_CERT_DIR"

    # Upload certificate
    UPLOAD_RESPONSE=$(curl -s -b "$COOKIE_FILE" -X POST "$API_URL/certificates" \
        -F "name=QA-Test-Cert-$(date +%s)" \
        -F "certificate_file=@$TEST_CERT_DIR/test.crt" \
        -F "key_file=@$TEST_CERT_DIR/test.key" \
        -w "\n%{http_code}")

    HTTP_CODE=$(echo "$UPLOAD_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$UPLOAD_RESPONSE" | sed '$d')

    if [ "$HTTP_CODE" = "201" ]; then
        log_test "PASS" "Certificate upload successful" "HTTP $HTTP_CODE"

        # Extract certificate ID for later deletion
        CERT_ID=$(echo "$RESPONSE_BODY" | jq -r '.id' 2>/dev/null || echo "")
        if [ -n "$CERT_ID" ] && [ "$CERT_ID" != "null" ]; then
            log_test "INFO" "Certificate created with ID: $CERT_ID" ""
            echo "$CERT_ID" > /tmp/charon-test-cert-id.txt
        fi
    elif [ "$HTTP_CODE" = "401" ]; then
        log_test "FAIL" "Upload authentication failed - 401 Unauthorized" "Cookie not being sent"
    else
        log_test "FAIL" "Certificate upload failed" "HTTP $HTTP_CODE - $RESPONSE_BODY"
    fi
else
    log_test "FAIL" "Could not generate test certificate" ""
fi

# Test 1.4: Certificate Delete (DELETE /api/v1/certificates/:id)
echo -e "\n${YELLOW}Test 1.4: Certificate Delete (DELETE /api/v1/certificates/:id)${NC}"

if [ -f /tmp/charon-test-cert-id.txt ]; then
    CERT_ID=$(cat /tmp/charon-test-cert-id.txt)

    if [ -n "$CERT_ID" ] && [ "$CERT_ID" != "null" ]; then
        DELETE_RESPONSE=$(curl -s -b "$COOKIE_FILE" -X DELETE "$API_URL/certificates/$CERT_ID" -w "\n%{http_code}")
        HTTP_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)
        RESPONSE_BODY=$(echo "$DELETE_RESPONSE" | sed '$d')

        if [ "$HTTP_CODE" = "200" ]; then
            log_test "PASS" "Certificate delete successful" "HTTP $HTTP_CODE"
        elif [ "$HTTP_CODE" = "401" ]; then
            log_test "FAIL" "Delete authentication failed - 401 Unauthorized" "Cookie not being sent"
        elif [ "$HTTP_CODE" = "409" ]; then
            log_test "INFO" "Certificate in use (expected for active certs)" "HTTP $HTTP_CODE"
        else
            log_test "WARN" "Certificate delete failed" "HTTP $HTTP_CODE - $RESPONSE_BODY"
        fi
    else
        log_test "SKIP" "Certificate delete test" "No certificate ID available"
    fi
else
    log_test "SKIP" "Certificate delete test" "Upload test did not create a certificate"
fi

# Test 1.5: Unauthorized Access
echo -e "\n${YELLOW}Test 1.5: Unauthorized Access${NC}"

# Remove cookies and try to access
rm -f "$COOKIE_FILE"

UNAUTH_RESPONSE=$(curl -s "$API_URL/certificates" -w "\n%{http_code}")
HTTP_CODE=$(echo "$UNAUTH_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "401" ]; then
    log_test "PASS" "Unauthorized access properly rejected" "HTTP $HTTP_CODE"
else
    log_test "FAIL" "Unauthorized access NOT rejected" "HTTP $HTTP_CODE (expected 401)"
fi

# Phase 2: Regression Testing Other Endpoints
section "Phase 2: Regression Testing Other Endpoints"

# Re-login for regression tests
echo -e "${YELLOW}Re-authenticating for regression tests...${NC}"
curl -s -c "$COOKIE_FILE" -X POST "$API_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"qa-test@example.com","password":"QATestPass123!"}' > /dev/null

# Test 2.1: Proxy Hosts Page
echo -e "\n${YELLOW}Test 2.1: Proxy Hosts Page (GET /api/v1/proxy-hosts)${NC}"
HOSTS_RESPONSE=$(curl -s -b "$COOKIE_FILE" "$API_URL/proxy-hosts" -w "\n%{http_code}")
HTTP_CODE=$(echo "$HOSTS_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Proxy hosts list successful" "HTTP $HTTP_CODE"
elif [ "$HTTP_CODE" = "401" ]; then
    log_test "FAIL" "Proxy hosts authentication failed" "HTTP $HTTP_CODE"
else
    log_test "WARN" "Proxy hosts request failed" "HTTP $HTTP_CODE"
fi

# Test 2.2: Backups Page
echo -e "\n${YELLOW}Test 2.2: Backups Page (GET /api/v1/backups)${NC}"
BACKUPS_RESPONSE=$(curl -s -b "$COOKIE_FILE" "$API_URL/backups" -w "\n%{http_code}")
HTTP_CODE=$(echo "$BACKUPS_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Backups list successful" "HTTP $HTTP_CODE"
elif [ "$HTTP_CODE" = "401" ]; then
    log_test "FAIL" "Backups authentication failed" "HTTP $HTTP_CODE"
else
    log_test "WARN" "Backups request failed" "HTTP $HTTP_CODE"
fi

# Test 2.3: Settings Page
echo -e "\n${YELLOW}Test 2.3: Settings Page (GET /api/v1/settings)${NC}"
SETTINGS_RESPONSE=$(curl -s -b "$COOKIE_FILE" "$API_URL/settings" -w "\n%{http_code}")
HTTP_CODE=$(echo "$SETTINGS_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Settings list successful" "HTTP $HTTP_CODE"
elif [ "$HTTP_CODE" = "401" ]; then
    log_test "FAIL" "Settings authentication failed" "HTTP $HTTP_CODE"
else
    log_test "WARN" "Settings request failed" "HTTP $HTTP_CODE"
fi

# Test 2.4: User Management
echo -e "\n${YELLOW}Test 2.4: User Management (GET /api/v1/users)${NC}"
USERS_RESPONSE=$(curl -s -b "$COOKIE_FILE" "$API_URL/users" -w "\n%{http_code}")
HTTP_CODE=$(echo "$USERS_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" = "200" ]; then
    log_test "PASS" "Users list successful" "HTTP $HTTP_CODE"
elif [ "$HTTP_CODE" = "401" ]; then
    log_test "FAIL" "Users authentication failed" "HTTP $HTTP_CODE"
else
    log_test "WARN" "Users request failed" "HTTP $HTTP_CODE"
fi

# Summary
section "Test Summary"

echo -e "\n${BLUE}=== Test Results Summary ===${NC}\n"

TOTAL_TESTS=$(grep -c "^\[" "$TEST_RESULTS" || echo "0")
PASSED=$(grep -c "^\[PASS\]" "$TEST_RESULTS" || echo "0")
FAILED=$(grep -c "^\[FAIL\]" "$TEST_RESULTS" || echo "0")
WARNINGS=$(grep -c "^\[WARN\]" "$TEST_RESULTS" || echo "0")
SKIPPED=$(grep -c "^\[SKIP\]" "$TEST_RESULTS" || echo "0")

echo "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
echo "Skipped: $SKIPPED"

echo ""
echo "Full test results saved to: $TEST_RESULTS"
echo ""

# Exit with error if any tests failed
if [ "$FAILED" -gt 0 ]; then
    echo -e "${RED}Some tests FAILED. Review the results above.${NC}"
    exit 1
else
    echo -e "${GREEN}All critical tests PASSED!${NC}"
    exit 0
fi
