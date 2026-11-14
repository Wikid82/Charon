window.BENCHMARK_DATA = {
  "lastUpdate": 1794669171495,
  "repoUrl": "https://github.com/Wikid82/Charon",
  "entries": {
    "Go Benchmark": [
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5786ad8af5a26f788e13f5ab32ef6edeb10aceaa",
          "message": "chore: promote nightly to main (2026-11-13) (#1939)",
          "timestamp": "2026-11-13T05:08:50-05:00",
          "tree_id": "b144ccbff54a621b7fa9d70af1e428649070e19b",
          "url": "https://github.com/Wikid82/Charon/commit/5786ad8af5a26f788e13f5ab32ef6edeb10aceaa"
        },
        "date": 1794564782915,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 133016,
            "unit": "ns/op\t   45138 B/op\t     634 allocs/op",
            "extra": "8047 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 133016,
            "unit": "ns/op",
            "extra": "8047 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45138,
            "unit": "B/op",
            "extra": "8047 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "8047 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 83337,
            "unit": "ns/op\t   39174 B/op\t     480 allocs/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 83337,
            "unit": "ns/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39174,
            "unit": "B/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 221629,
            "unit": "ns/op\t   86614 B/op\t    1261 allocs/op",
            "extra": "5822 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 221629,
            "unit": "ns/op",
            "extra": "5822 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86614,
            "unit": "B/op",
            "extra": "5822 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "5822 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 44045,
            "unit": "ns/op\t   21148 B/op\t     268 allocs/op",
            "extra": "25654 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 44045,
            "unit": "ns/op",
            "extra": "25654 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21148,
            "unit": "B/op",
            "extra": "25654 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "25654 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 35998,
            "unit": "ns/op\t   15399 B/op\t     170 allocs/op",
            "extra": "32572 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 35998,
            "unit": "ns/op",
            "extra": "32572 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15399,
            "unit": "B/op",
            "extra": "32572 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "32572 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 149320,
            "unit": "ns/op\t   86309 B/op\t    1266 allocs/op",
            "extra": "7198 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 149320,
            "unit": "ns/op",
            "extra": "7198 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86309,
            "unit": "B/op",
            "extra": "7198 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1266,
            "unit": "allocs/op",
            "extra": "7198 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 132359,
            "unit": "ns/op\t   45145 B/op\t     638 allocs/op",
            "extra": "8690 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 132359,
            "unit": "ns/op",
            "extra": "8690 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45145,
            "unit": "B/op",
            "extra": "8690 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "8690 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 715.5,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1621356 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 715.5,
            "unit": "ns/op",
            "extra": "1621356 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1621356 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1621356 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 588.8,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2034636 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 588.8,
            "unit": "ns/op",
            "extra": "2034636 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2034636 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2034636 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.039,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "150055441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.039,
            "unit": "ns/op",
            "extra": "150055441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "150055441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "150055441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 232314,
            "unit": "ns/op\t   17898 B/op\t     126 allocs/op",
            "extra": "4804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 232314,
            "unit": "ns/op",
            "extra": "4804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17898,
            "unit": "B/op",
            "extra": "4804 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "4804 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 39.79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29183876 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 39.79,
            "unit": "ns/op",
            "extra": "29183876 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29183876 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29183876 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 82.35,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14837520 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 82.35,
            "unit": "ns/op",
            "extra": "14837520 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "14837520 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "14837520 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 82.61,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15199010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 82.61,
            "unit": "ns/op",
            "extra": "15199010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15199010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15199010 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 170.5,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "7567336 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 170.5,
            "unit": "ns/op",
            "extra": "7567336 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "7567336 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "7567336 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 98.54,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "11891959 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 98.54,
            "unit": "ns/op",
            "extra": "11891959 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "11891959 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "11891959 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 366.8,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3220898 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 366.8,
            "unit": "ns/op",
            "extra": "3220898 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3220898 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3220898 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 21490,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "54760 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 21490,
            "unit": "ns/op",
            "extra": "54760 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "54760 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "54760 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 431.4,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2736349 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 431.4,
            "unit": "ns/op",
            "extra": "2736349 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2736349 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2736349 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001963,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001963,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 11.12,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.12,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 10.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.72,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 10.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.68,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 41.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24557347 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 41.8,
            "unit": "ns/op",
            "extra": "24557347 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "24557347 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "24557347 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "6ba6176bd966e3ad2e94d12384df1160a8841668",
          "message": "chore(main): release 0.49.1 (#1940)",
          "timestamp": "2026-11-13T05:45:39-05:00",
          "tree_id": "c36a0c6f4966d71828fffcf78db3c574f85963c6",
          "url": "https://github.com/Wikid82/Charon/commit/6ba6176bd966e3ad2e94d12384df1160a8841668"
        },
        "date": 1794566989484,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 94151,
            "unit": "ns/op\t   45126 B/op\t     634 allocs/op",
            "extra": "13712 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 94151,
            "unit": "ns/op",
            "extra": "13712 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45126,
            "unit": "B/op",
            "extra": "13712 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "13712 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 61379,
            "unit": "ns/op\t   39174 B/op\t     480 allocs/op",
            "extra": "16840 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 61379,
            "unit": "ns/op",
            "extra": "16840 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39174,
            "unit": "B/op",
            "extra": "16840 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "16840 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 155872,
            "unit": "ns/op\t   86737 B/op\t    1261 allocs/op",
            "extra": "7490 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 155872,
            "unit": "ns/op",
            "extra": "7490 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86737,
            "unit": "B/op",
            "extra": "7490 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "7490 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 29786,
            "unit": "ns/op\t   21150 B/op\t     268 allocs/op",
            "extra": "40987 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 29786,
            "unit": "ns/op",
            "extra": "40987 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21150,
            "unit": "B/op",
            "extra": "40987 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "40987 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 26124,
            "unit": "ns/op\t   15397 B/op\t     170 allocs/op",
            "extra": "47744 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 26124,
            "unit": "ns/op",
            "extra": "47744 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15397,
            "unit": "B/op",
            "extra": "47744 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "47744 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 43285,
            "unit": "ns/op\t   47289 B/op\t     586 allocs/op",
            "extra": "27916 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 43285,
            "unit": "ns/op",
            "extra": "27916 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47289,
            "unit": "B/op",
            "extra": "27916 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 586,
            "unit": "allocs/op",
            "extra": "27916 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 116142,
            "unit": "ns/op\t   86167 B/op\t    1264 allocs/op",
            "extra": "9638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 116142,
            "unit": "ns/op",
            "extra": "9638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86167,
            "unit": "B/op",
            "extra": "9638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "9638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 88111,
            "unit": "ns/op\t   45137 B/op\t     638 allocs/op",
            "extra": "14438 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 88111,
            "unit": "ns/op",
            "extra": "14438 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45137,
            "unit": "B/op",
            "extra": "14438 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "14438 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 588.9,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "2043300 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 588.9,
            "unit": "ns/op",
            "extra": "2043300 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "2043300 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "2043300 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 474,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2542614 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 474,
            "unit": "ns/op",
            "extra": "2542614 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2542614 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2542614 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 4.799,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "250746885 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 4.799,
            "unit": "ns/op",
            "extra": "250746885 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "250746885 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "250746885 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 181275,
            "unit": "ns/op\t   17876 B/op\t     126 allocs/op",
            "extra": "6169 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 181275,
            "unit": "ns/op",
            "extra": "6169 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17876,
            "unit": "B/op",
            "extra": "6169 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6169 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 28.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40865218 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 28.98,
            "unit": "ns/op",
            "extra": "40865218 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "40865218 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "40865218 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 59.95,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20708808 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 59.95,
            "unit": "ns/op",
            "extra": "20708808 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "20708808 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "20708808 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 60.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20471834 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 60.2,
            "unit": "ns/op",
            "extra": "20471834 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "20471834 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "20471834 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 110.5,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "11462437 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 110.5,
            "unit": "ns/op",
            "extra": "11462437 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "11462437 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "11462437 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 78.38,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "15388628 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 78.38,
            "unit": "ns/op",
            "extra": "15388628 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "15388628 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "15388628 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 259.5,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "4580317 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 259.5,
            "unit": "ns/op",
            "extra": "4580317 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "4580317 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "4580317 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 15957,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "75045 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 15957,
            "unit": "ns/op",
            "extra": "75045 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "75045 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "75045 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 354,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "3906692 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 354,
            "unit": "ns/op",
            "extra": "3906692 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "3906692 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "3906692 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.000135,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.000135,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223131564 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.98,
            "unit": "ns/op",
            "extra": "223131564 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "223131564 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "223131564 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.911,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "244652886 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.911,
            "unit": "ns/op",
            "extra": "244652886 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "244652886 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "244652886 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.839,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "245862690 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.839,
            "unit": "ns/op",
            "extra": "245862690 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "245862690 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "245862690 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 33.11,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37112637 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 33.11,
            "unit": "ns/op",
            "extra": "37112637 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "37112637 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "37112637 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "15edc7f9c6241d76bdc3be8c829618ebcffce505",
          "message": "chore: promote nightly to main (2026-11-13) (#1947)",
          "timestamp": "2026-11-13T15:05:53-05:00",
          "tree_id": "0dd2681ca26d251231d7d9a02d1a2722918515a8",
          "url": "https://github.com/Wikid82/Charon/commit/15edc7f9c6241d76bdc3be8c829618ebcffce505"
        },
        "date": 1794600620726,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 171925,
            "unit": "ns/op\t   45139 B/op\t     634 allocs/op",
            "extra": "6163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 171925,
            "unit": "ns/op",
            "extra": "6163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45139,
            "unit": "B/op",
            "extra": "6163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "6163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 109460,
            "unit": "ns/op\t   39176 B/op\t     480 allocs/op",
            "extra": "9849 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 109460,
            "unit": "ns/op",
            "extra": "9849 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39176,
            "unit": "B/op",
            "extra": "9849 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "9849 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 317603,
            "unit": "ns/op\t   86618 B/op\t    1261 allocs/op",
            "extra": "4100 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 317603,
            "unit": "ns/op",
            "extra": "4100 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86618,
            "unit": "B/op",
            "extra": "4100 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "4100 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 53224,
            "unit": "ns/op\t   21149 B/op\t     268 allocs/op",
            "extra": "18837 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 53224,
            "unit": "ns/op",
            "extra": "18837 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21149,
            "unit": "B/op",
            "extra": "18837 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "18837 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 47474,
            "unit": "ns/op\t   15399 B/op\t     170 allocs/op",
            "extra": "25084 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 47474,
            "unit": "ns/op",
            "extra": "25084 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15399,
            "unit": "B/op",
            "extra": "25084 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "25084 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 85085,
            "unit": "ns/op\t   47420 B/op\t     589 allocs/op",
            "extra": "14347 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 85085,
            "unit": "ns/op",
            "extra": "14347 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47420,
            "unit": "B/op",
            "extra": "14347 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 589,
            "unit": "allocs/op",
            "extra": "14347 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 193986,
            "unit": "ns/op\t   86346 B/op\t    1266 allocs/op",
            "extra": "5461 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 193986,
            "unit": "ns/op",
            "extra": "5461 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86346,
            "unit": "B/op",
            "extra": "5461 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1266,
            "unit": "allocs/op",
            "extra": "5461 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 174681,
            "unit": "ns/op\t   45146 B/op\t     638 allocs/op",
            "extra": "6313 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 174681,
            "unit": "ns/op",
            "extra": "6313 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45146,
            "unit": "B/op",
            "extra": "6313 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "6313 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 895.2,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1314680 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 895.2,
            "unit": "ns/op",
            "extra": "1314680 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1314680 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1314680 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 755,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1553290 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 755,
            "unit": "ns/op",
            "extra": "1553290 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1553290 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1553290 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 10.32,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 10.32,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 300509,
            "unit": "ns/op\t   17887 B/op\t     126 allocs/op",
            "extra": "3697 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 300509,
            "unit": "ns/op",
            "extra": "3697 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17887,
            "unit": "B/op",
            "extra": "3697 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "3697 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 51.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23315533 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 51.48,
            "unit": "ns/op",
            "extra": "23315533 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "23315533 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "23315533 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 106.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11109516 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 106.3,
            "unit": "ns/op",
            "extra": "11109516 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11109516 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11109516 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 102.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11653140 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 102.8,
            "unit": "ns/op",
            "extra": "11653140 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11653140 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11653140 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 188.1,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "6101488 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 188.1,
            "unit": "ns/op",
            "extra": "6101488 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "6101488 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "6101488 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 127.3,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "9328388 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 127.3,
            "unit": "ns/op",
            "extra": "9328388 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "9328388 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "9328388 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 465.6,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2601830 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 465.6,
            "unit": "ns/op",
            "extra": "2601830 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2601830 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2601830 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 28309,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "41659 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 28309,
            "unit": "ns/op",
            "extra": "41659 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "41659 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "41659 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 568,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2002272 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 568,
            "unit": "ns/op",
            "extra": "2002272 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2002272 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2002272 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002108,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.76,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "82866512 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.76,
            "unit": "ns/op",
            "extra": "82866512 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "82866512 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "82866512 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.91,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "87196032 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.91,
            "unit": "ns/op",
            "extra": "87196032 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "87196032 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "87196032 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.77,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "87030535 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.77,
            "unit": "ns/op",
            "extra": "87030535 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "87030535 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "87030535 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 54.12,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23510154 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 54.12,
            "unit": "ns/op",
            "extra": "23510154 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "23510154 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "23510154 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "349da759e86fec2e37f341427a8acc44995e1ba7",
          "message": "chore(main): release 0.49.2 (#1948)",
          "timestamp": "2026-11-13T15:38:11-05:00",
          "tree_id": "a4104eb21f2875020154846049cd202c11f72b56",
          "url": "https://github.com/Wikid82/Charon/commit/349da759e86fec2e37f341427a8acc44995e1ba7"
        },
        "date": 1794602581710,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 135317,
            "unit": "ns/op\t   45134 B/op\t     634 allocs/op",
            "extra": "7794 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 135317,
            "unit": "ns/op",
            "extra": "7794 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45134,
            "unit": "B/op",
            "extra": "7794 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "7794 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 93452,
            "unit": "ns/op\t   39172 B/op\t     480 allocs/op",
            "extra": "11991 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 93452,
            "unit": "ns/op",
            "extra": "11991 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39172,
            "unit": "B/op",
            "extra": "11991 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "11991 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 205769,
            "unit": "ns/op\t   86726 B/op\t    1261 allocs/op",
            "extra": "5595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 205769,
            "unit": "ns/op",
            "extra": "5595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86726,
            "unit": "B/op",
            "extra": "5595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "5595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 40323,
            "unit": "ns/op\t   21154 B/op\t     268 allocs/op",
            "extra": "27964 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 40323,
            "unit": "ns/op",
            "extra": "27964 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21154,
            "unit": "B/op",
            "extra": "27964 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "27964 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 37599,
            "unit": "ns/op\t   15400 B/op\t     170 allocs/op",
            "extra": "32035 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 37599,
            "unit": "ns/op",
            "extra": "32035 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15400,
            "unit": "B/op",
            "extra": "32035 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "32035 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 65671,
            "unit": "ns/op\t   47326 B/op\t     587 allocs/op",
            "extra": "18256 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 65671,
            "unit": "ns/op",
            "extra": "18256 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47326,
            "unit": "B/op",
            "extra": "18256 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 587,
            "unit": "allocs/op",
            "extra": "18256 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 152659,
            "unit": "ns/op\t   86301 B/op\t    1265 allocs/op",
            "extra": "6874 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 152659,
            "unit": "ns/op",
            "extra": "6874 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86301,
            "unit": "B/op",
            "extra": "6874 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1265,
            "unit": "allocs/op",
            "extra": "6874 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 136546,
            "unit": "ns/op\t   45138 B/op\t     638 allocs/op",
            "extra": "8308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 136546,
            "unit": "ns/op",
            "extra": "8308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45138,
            "unit": "B/op",
            "extra": "8308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "8308 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 690.2,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1733631 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 690.2,
            "unit": "ns/op",
            "extra": "1733631 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1733631 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1733631 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 592.9,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2011930 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 592.9,
            "unit": "ns/op",
            "extra": "2011930 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2011930 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2011930 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 7.987,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "149731844 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 7.987,
            "unit": "ns/op",
            "extra": "149731844 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "149731844 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "149731844 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 235868,
            "unit": "ns/op\t   17870 B/op\t     126 allocs/op",
            "extra": "4824 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 235868,
            "unit": "ns/op",
            "extra": "4824 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17870,
            "unit": "B/op",
            "extra": "4824 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "4824 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 40.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29980160 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 40.68,
            "unit": "ns/op",
            "extra": "29980160 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29980160 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29980160 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 81.12,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14808918 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 81.12,
            "unit": "ns/op",
            "extra": "14808918 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "14808918 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "14808918 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15132391 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 79,
            "unit": "ns/op",
            "extra": "15132391 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15132391 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15132391 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 144.6,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "8207791 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 144.6,
            "unit": "ns/op",
            "extra": "8207791 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "8207791 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "8207791 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 98.95,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "10476810 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 98.95,
            "unit": "ns/op",
            "extra": "10476810 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "10476810 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "10476810 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 357.4,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3277238 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 357.4,
            "unit": "ns/op",
            "extra": "3277238 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3277238 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3277238 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 21947,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "53316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 21947,
            "unit": "ns/op",
            "extra": "53316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "53316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "53316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 432.8,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2756196 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 432.8,
            "unit": "ns/op",
            "extra": "2756196 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2756196 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2756196 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001626,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001626,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 11.18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.18,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 10.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.72,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 10.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.67,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 42.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29745973 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 42.19,
            "unit": "ns/op",
            "extra": "29745973 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "29745973 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "29745973 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "995479f5e281d97e8ad29b36efc0ddc315acfb8c",
          "message": "chore: promote nightly to main (2026-11-14) (#1960)",
          "timestamp": "2026-11-14T09:25:57-05:00",
          "tree_id": "3155e3386b3c2bb5e185bdba90a07df0029a97ed",
          "url": "https://github.com/Wikid82/Charon/commit/995479f5e281d97e8ad29b36efc0ddc315acfb8c"
        },
        "date": 1794666665844,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 148597,
            "unit": "ns/op\t   45135 B/op\t     634 allocs/op",
            "extra": "7545 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 148597,
            "unit": "ns/op",
            "extra": "7545 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45135,
            "unit": "B/op",
            "extra": "7545 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "7545 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 103354,
            "unit": "ns/op\t   39176 B/op\t     480 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 103354,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39176,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 258075,
            "unit": "ns/op\t   86813 B/op\t    1261 allocs/op",
            "extra": "4335 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 258075,
            "unit": "ns/op",
            "extra": "4335 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86813,
            "unit": "B/op",
            "extra": "4335 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "4335 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 49667,
            "unit": "ns/op\t   21158 B/op\t     268 allocs/op",
            "extra": "24151 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 49667,
            "unit": "ns/op",
            "extra": "24151 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21158,
            "unit": "B/op",
            "extra": "24151 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "24151 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 49115,
            "unit": "ns/op\t   15396 B/op\t     170 allocs/op",
            "extra": "26805 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 49115,
            "unit": "ns/op",
            "extra": "26805 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15396,
            "unit": "B/op",
            "extra": "26805 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "26805 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 195173,
            "unit": "ns/op\t   86158 B/op\t    1264 allocs/op",
            "extra": "5378 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 195173,
            "unit": "ns/op",
            "extra": "5378 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86158,
            "unit": "B/op",
            "extra": "5378 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "5378 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 149930,
            "unit": "ns/op\t   45146 B/op\t     638 allocs/op",
            "extra": "7586 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 149930,
            "unit": "ns/op",
            "extra": "7586 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45146,
            "unit": "B/op",
            "extra": "7586 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "7586 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 912.3,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1324087 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 912.3,
            "unit": "ns/op",
            "extra": "1324087 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1324087 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1324087 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 811.8,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1545056 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 811.8,
            "unit": "ns/op",
            "extra": "1545056 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1545056 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1545056 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 10.16,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 10.16,
            "unit": "ns/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 220552,
            "unit": "ns/op\t   17863 B/op\t     126 allocs/op",
            "extra": "5293 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 220552,
            "unit": "ns/op",
            "extra": "5293 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17863,
            "unit": "B/op",
            "extra": "5293 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "5293 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 56.36,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21247286 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 56.36,
            "unit": "ns/op",
            "extra": "21247286 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "21247286 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "21247286 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 110.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11032897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 110.2,
            "unit": "ns/op",
            "extra": "11032897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11032897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11032897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 110.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10843822 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 110.2,
            "unit": "ns/op",
            "extra": "10843822 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10843822 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10843822 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 192.2,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "6204082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 192.2,
            "unit": "ns/op",
            "extra": "6204082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "6204082 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "6204082 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 141.4,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "8788365 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 141.4,
            "unit": "ns/op",
            "extra": "8788365 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "8788365 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "8788365 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 564.6,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "1978522 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 564.6,
            "unit": "ns/op",
            "extra": "1978522 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "1978522 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "1978522 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 25847,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "46521 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 25847,
            "unit": "ns/op",
            "extra": "46521 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "46521 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "46521 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 535,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2239839 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 535,
            "unit": "ns/op",
            "extra": "2239839 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2239839 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2239839 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002103,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.88,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "84589731 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.88,
            "unit": "ns/op",
            "extra": "84589731 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "84589731 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "84589731 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.38,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "91055593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.38,
            "unit": "ns/op",
            "extra": "91055593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "91055593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "91055593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.03,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "91768572 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.03,
            "unit": "ns/op",
            "extra": "91768572 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "91768572 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "91768572 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 56.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21264546 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 56.52,
            "unit": "ns/op",
            "extra": "21264546 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "21264546 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "21264546 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Jeremy",
            "username": "Wikid82"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5babdb2299354f0c2bb1862ae5ffdf7d504f8c49",
          "message": "chore(main): release 0.50.0 (#1962)",
          "timestamp": "2026-11-14T10:08:08-05:00",
          "tree_id": "e104c430da55018f3fb2ac54943edbcc4f0da86c",
          "url": "https://github.com/Wikid82/Charon/commit/5babdb2299354f0c2bb1862ae5ffdf7d504f8c49"
        },
        "date": 1794669170496,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 109787,
            "unit": "ns/op\t   45133 B/op\t     634 allocs/op",
            "extra": "9774 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 109787,
            "unit": "ns/op",
            "extra": "9774 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45133,
            "unit": "B/op",
            "extra": "9774 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "9774 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 76335,
            "unit": "ns/op\t   39177 B/op\t     480 allocs/op",
            "extra": "15693 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 76335,
            "unit": "ns/op",
            "extra": "15693 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39177,
            "unit": "B/op",
            "extra": "15693 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "15693 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 188839,
            "unit": "ns/op\t   86757 B/op\t    1261 allocs/op",
            "extra": "6206 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 188839,
            "unit": "ns/op",
            "extra": "6206 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86757,
            "unit": "B/op",
            "extra": "6206 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "6206 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 36070,
            "unit": "ns/op\t   21153 B/op\t     268 allocs/op",
            "extra": "33163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 36070,
            "unit": "ns/op",
            "extra": "33163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21153,
            "unit": "B/op",
            "extra": "33163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "33163 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 32940,
            "unit": "ns/op\t   15399 B/op\t     170 allocs/op",
            "extra": "36306 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 32940,
            "unit": "ns/op",
            "extra": "36306 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15399,
            "unit": "B/op",
            "extra": "36306 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "36306 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 122638,
            "unit": "ns/op\t   86137 B/op\t    1264 allocs/op",
            "extra": "8421 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 122638,
            "unit": "ns/op",
            "extra": "8421 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86137,
            "unit": "B/op",
            "extra": "8421 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "8421 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 110255,
            "unit": "ns/op\t   45139 B/op\t     638 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 110255,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45139,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 698,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1713219 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 698,
            "unit": "ns/op",
            "extra": "1713219 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1713219 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1713219 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 580.2,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2091115 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 580.2,
            "unit": "ns/op",
            "extra": "2091115 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2091115 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2091115 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.115,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "148911192 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.115,
            "unit": "ns/op",
            "extra": "148911192 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "148911192 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "148911192 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 157631,
            "unit": "ns/op\t   17881 B/op\t     126 allocs/op",
            "extra": "6985 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 157631,
            "unit": "ns/op",
            "extra": "6985 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17881,
            "unit": "B/op",
            "extra": "6985 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6985 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 38.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31424965 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 38.21,
            "unit": "ns/op",
            "extra": "31424965 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "31424965 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "31424965 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 84.87,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14090331 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 84.87,
            "unit": "ns/op",
            "extra": "14090331 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "14090331 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "14090331 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 84.28,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14239275 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 84.28,
            "unit": "ns/op",
            "extra": "14239275 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "14239275 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "14239275 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 156.5,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "7902126 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 156.5,
            "unit": "ns/op",
            "extra": "7902126 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "7902126 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "7902126 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 94.17,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "12719953 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 94.17,
            "unit": "ns/op",
            "extra": "12719953 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "12719953 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "12719953 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 388.4,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3187690 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 388.4,
            "unit": "ns/op",
            "extra": "3187690 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3187690 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3187690 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 20688,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "58311 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 20688,
            "unit": "ns/op",
            "extra": "58311 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "58311 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "58311 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 395,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "3010123 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 395,
            "unit": "ns/op",
            "extra": "3010123 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "3010123 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "3010123 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001478,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001478,
            "unit": "ns/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util)",
            "value": 15.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "78528675 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.05,
            "unit": "ns/op",
            "extra": "78528675 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78528675 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78528675 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "78558070 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.72,
            "unit": "ns/op",
            "extra": "78558070 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78558070 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78558070 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "91856133 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.45,
            "unit": "ns/op",
            "extra": "91856133 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "91856133 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "91856133 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 39.46,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31156494 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 39.46,
            "unit": "ns/op",
            "extra": "31156494 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "31156494 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "31156494 times\n4 procs"
          }
        ]
      }
    ]
  }
}