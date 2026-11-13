window.BENCHMARK_DATA = {
  "lastUpdate": 1794566990068,
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
      }
    ]
  }
}