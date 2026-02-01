#!/bin/bash
set -e

echo "═══════════════════════════════════════════════"
echo "Phase 2 Validation: DNS Provider Type Tests"
echo "═══════════════════════════════════════════════"
echo ""

# Test 1: Webhook provider test
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: Webhook Provider (10 runs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

WEBHOOK_PASS=0
WEBHOOK_FAIL=0

for i in {1..10}; do
  echo "Run $i/10: Webhook provider test..."

  if npx playwright test tests/dns-provider-types.spec.ts \
    --grep "should show URL field when Webhook type is selected" \
    --project=chromium \
    --reporter=line > /dev/null 2>&1; then
    echo "✅ Run $i PASSED"
    WEBHOOK_PASS=$((WEBHOOK_PASS + 1))
  else
    echo "❌ Run $i FAILED"
    WEBHOOK_FAIL=$((WEBHOOK_FAIL + 1))
  fi
done

echo ""
echo "Webhook Test Results: $WEBHOOK_PASS passed, $WEBHOOK_FAIL failed"
echo ""

if [ $WEBHOOK_FAIL -gt 0 ]; then
  echo "❌ Webhook test validation FAILED"
  exit 1
fi

# Test 2: RFC2136 provider test
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: RFC2136 Provider (10 runs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

RFC_PASS=0
RFC_FAIL=0

for i in {1..10}; do
  echo "Run $i/10: RFC2136 provider test..."

  if npx playwright test tests/dns-provider-types.spec.ts \
    --grep "should show server field when RFC2136 type is selected" \
    --project=chromium \
    --reporter=line > /dev/null 2>&1; then
    echo "✅ Run $i PASSED"
    RFC_PASS=$((RFC_PASS + 1))
  else
    echo "❌ Run $i FAILED"
    RFC_FAIL=$((RFC_FAIL + 1))
  fi
done

echo ""
echo "RFC2136 Test Results: $RFC_PASS passed, $RFC_FAIL failed"
echo ""

if [ $RFC_FAIL -gt 0 ]; then
  echo "❌ RFC2136 test validation FAILED"
  exit 1
fi

# Summary
echo "═══════════════════════════════════════════════"
echo "✅ Phase 2 Validation Complete"
echo "═══════════════════════════════════════════════"
echo ""
echo "Summary:"
echo "  Webhook Provider:  $WEBHOOK_PASS/10 passed"
echo "  RFC2136 Provider:  $RFC_PASS/10 passed"
echo ""
echo "All tests passed 10 consecutive runs!"
