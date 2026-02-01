#!/usr/bin/env bash
set +e  # Don't exit on error

echo "========================================="
echo "Validating Webhook and RFC2136 Tests"
echo "========================================="
echo ""

# Test Webhook provider 10 times
echo "Testing Webhook Provider (10 runs)..."
webhook_passed=0
webhook_failed=0

for i in {1..10}; do
    echo "  Run $i/10..."
    if npx playwright test tests/dns-provider-types.spec.ts \
        --grep "should show URL field when Webhook type is selected" \
        --project=firefox \
        --quiet >/dev/null 2>&1; then
        ((webhook_passed++))
        echo "    ✓ Passed"
    else
        ((webhook_failed++))
        echo "    ✗ Failed"
    fi
done

echo ""
echo "Webhook Results: $webhook_passed passed, $webhook_failed failed"
echo ""

# Test RFC2136 provider 10 times
echo "Testing RFC2136 Provider (10 runs)..."
rfc2136_passed=0
rfc2136_failed=0

for i in {1..10}; do
    echo "  Run $i/10..."
    if npx playwright test tests/dns-provider-types.spec.ts \
        --grep "should show server field when RFC2136 type is selected" \
        --project=firefox \
        --quiet >/dev/null 2>&1; then
        ((rfc2136_passed++))
        echo "    ✓ Passed"
    else
        ((rfc2136_failed++))
        echo "    ✗ Failed"
    fi
done

echo ""
echo "RFC2136 Results: $rfc2136_passed passed, $rfc2136_failed failed"
echo ""

# Summary
echo "========================================="
echo "FINAL RESULTS"
echo "========================================="
echo "Webhook:  $webhook_passed/10 passed"
echo "RFC2136:  $rfc2136_passed/10 passed"
echo "Total:    $((webhook_passed + rfc2136_passed))/20 passed"
echo ""

if [ $webhook_passed -eq 10 ] && [ $rfc2136_passed -eq 10 ]; then
    echo "✅ SUCCESS: All 20 tests passed!"
    exit 0
else
    echo "❌ FAILURE: Some tests failed"
    exit 1
fi
