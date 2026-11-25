window.BENCHMARK_DATA = {
  "lastUpdate": 1795630315173,
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
      },
      {
        "commit": {
          "author": {
            "email": "jhatfield82@gmail.com",
            "name": "Wikid82",
            "username": "Wikid82"
          },
          "committer": {
            "email": "jhatfield82@gmail.com",
            "name": "Wikid82",
            "username": "Wikid82"
          },
          "distinct": true,
          "id": "187ff611f75adb67c282efe5f29c4769e09759a4",
          "message": "chore: update dependencyDashboard configuration to use an object format",
          "timestamp": "2026-11-16T09:37:33-05:00",
          "tree_id": "e4097d82a65b192b9ead4c248c18d87964be4355",
          "url": "https://github.com/Wikid82/Charon/commit/187ff611f75adb67c282efe5f29c4769e09759a4"
        },
        "date": 1794840488765,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 233505,
            "unit": "ns/op\t   45136 B/op\t     634 allocs/op",
            "extra": "4677 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 233505,
            "unit": "ns/op",
            "extra": "4677 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45136,
            "unit": "B/op",
            "extra": "4677 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 634,
            "unit": "allocs/op",
            "extra": "4677 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 167029,
            "unit": "ns/op\t   39172 B/op\t     480 allocs/op",
            "extra": "8010 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 167029,
            "unit": "ns/op",
            "extra": "8010 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 39172,
            "unit": "B/op",
            "extra": "8010 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 480,
            "unit": "allocs/op",
            "extra": "8010 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 319602,
            "unit": "ns/op\t   86528 B/op\t    1261 allocs/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 319602,
            "unit": "ns/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86528,
            "unit": "B/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 62231,
            "unit": "ns/op\t   21155 B/op\t     268 allocs/op",
            "extra": "18865 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 62231,
            "unit": "ns/op",
            "extra": "18865 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 21155,
            "unit": "B/op",
            "extra": "18865 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 268,
            "unit": "allocs/op",
            "extra": "18865 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 65545,
            "unit": "ns/op\t   15400 B/op\t     170 allocs/op",
            "extra": "18246 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 65545,
            "unit": "ns/op",
            "extra": "18246 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 15400,
            "unit": "B/op",
            "extra": "18246 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 170,
            "unit": "allocs/op",
            "extra": "18246 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 105750,
            "unit": "ns/op\t   47721 B/op\t     597 allocs/op",
            "extra": "9732 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 105750,
            "unit": "ns/op",
            "extra": "9732 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47721,
            "unit": "B/op",
            "extra": "9732 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 597,
            "unit": "allocs/op",
            "extra": "9732 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 229757,
            "unit": "ns/op\t   86350 B/op\t    1266 allocs/op",
            "extra": "4908 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 229757,
            "unit": "ns/op",
            "extra": "4908 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86350,
            "unit": "B/op",
            "extra": "4908 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1266,
            "unit": "allocs/op",
            "extra": "4908 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 229215,
            "unit": "ns/op\t   45152 B/op\t     638 allocs/op",
            "extra": "5068 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 229215,
            "unit": "ns/op",
            "extra": "5068 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 45152,
            "unit": "B/op",
            "extra": "5068 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 638,
            "unit": "allocs/op",
            "extra": "5068 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 887.7,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1375894 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 887.7,
            "unit": "ns/op",
            "extra": "1375894 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1375894 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1375894 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 750.6,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1603393 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 750.6,
            "unit": "ns/op",
            "extra": "1603393 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1603393 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1603393 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 9.701,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "123693638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 9.701,
            "unit": "ns/op",
            "extra": "123693638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "123693638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "123693638 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 320562,
            "unit": "ns/op\t   17887 B/op\t     126 allocs/op",
            "extra": "3655 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 320562,
            "unit": "ns/op",
            "extra": "3655 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17887,
            "unit": "B/op",
            "extra": "3655 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "3655 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 53.41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22553264 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 53.41,
            "unit": "ns/op",
            "extra": "22553264 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "22553264 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "22553264 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 107.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11152815 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 107.9,
            "unit": "ns/op",
            "extra": "11152815 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11152815 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11152815 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 110.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10805728 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 110.7,
            "unit": "ns/op",
            "extra": "10805728 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10805728 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10805728 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 182.3,
            "unit": "ns/op\t     600 B/op\t       5 allocs/op",
            "extra": "6591475 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 182.3,
            "unit": "ns/op",
            "extra": "6591475 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 600,
            "unit": "B/op",
            "extra": "6591475 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "6591475 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 138.2,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "8687314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 138.2,
            "unit": "ns/op",
            "extra": "8687314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "8687314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "8687314 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 505.8,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2371888 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 505.8,
            "unit": "ns/op",
            "extra": "2371888 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2371888 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2371888 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 34763,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "33346 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 34763,
            "unit": "ns/op",
            "extra": "33346 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "33346 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "33346 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 573.6,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2084552 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 573.6,
            "unit": "ns/op",
            "extra": "2084552 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2084552 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2084552 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002472,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002472,
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
            "value": 12.87,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95564029 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.87,
            "unit": "ns/op",
            "extra": "95564029 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95564029 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95564029 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95817012 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.5,
            "unit": "ns/op",
            "extra": "95817012 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95817012 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95817012 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96500202 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.5,
            "unit": "ns/op",
            "extra": "96500202 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96500202 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96500202 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 54.37,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20983797 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 54.37,
            "unit": "ns/op",
            "extra": "20983797 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "20983797 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "20983797 times\n4 procs"
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
          "id": "080397f2918faecd135f6f6a9c0849fdfc8c6bec",
          "message": "chore: promote nightly to main (2026-11-16) (#1994)",
          "timestamp": "2026-11-16T15:30:58-05:00",
          "tree_id": "740f51ec36bbca79e24ac3b0a6c50eb8a12463c8",
          "url": "https://github.com/Wikid82/Charon/commit/080397f2918faecd135f6f6a9c0849fdfc8c6bec"
        },
        "date": 1794861301692,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 114376,
            "unit": "ns/op\t   47181 B/op\t     670 allocs/op",
            "extra": "9145 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 114376,
            "unit": "ns/op",
            "extra": "9145 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47181,
            "unit": "B/op",
            "extra": "9145 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "9145 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 82681,
            "unit": "ns/op\t   42621 B/op\t     519 allocs/op",
            "extra": "14494 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 82681,
            "unit": "ns/op",
            "extra": "14494 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42621,
            "unit": "B/op",
            "extra": "14494 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "14494 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 190123,
            "unit": "ns/op\t   86722 B/op\t    1261 allocs/op",
            "extra": "6159 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 190123,
            "unit": "ns/op",
            "extra": "6159 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86722,
            "unit": "B/op",
            "extra": "6159 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "6159 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 95532,
            "unit": "ns/op\t   47626 B/op\t     789 allocs/op",
            "extra": "12832 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 95532,
            "unit": "ns/op",
            "extra": "12832 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47626,
            "unit": "B/op",
            "extra": "12832 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "12832 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 49765,
            "unit": "ns/op\t   20605 B/op\t     236 allocs/op",
            "extra": "23437 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 49765,
            "unit": "ns/op",
            "extra": "23437 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20605,
            "unit": "B/op",
            "extra": "23437 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "23437 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 127570,
            "unit": "ns/op\t   86179 B/op\t    1264 allocs/op",
            "extra": "8716 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 127570,
            "unit": "ns/op",
            "extra": "8716 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86179,
            "unit": "B/op",
            "extra": "8716 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "8716 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 114506,
            "unit": "ns/op\t   47170 B/op\t     675 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 114506,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47170,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 706.5,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1677064 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 706.5,
            "unit": "ns/op",
            "extra": "1677064 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1677064 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1677064 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 617.1,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2029561 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 617.1,
            "unit": "ns/op",
            "extra": "2029561 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2029561 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2029561 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.055,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "148221994 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.055,
            "unit": "ns/op",
            "extra": "148221994 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "148221994 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "148221994 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 160368,
            "unit": "ns/op\t   17873 B/op\t     126 allocs/op",
            "extra": "6859 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 160368,
            "unit": "ns/op",
            "extra": "6859 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17873,
            "unit": "B/op",
            "extra": "6859 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6859 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 67.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17942874 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 67.2,
            "unit": "ns/op",
            "extra": "17942874 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17942874 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17942874 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 112.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10570041 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 112.8,
            "unit": "ns/op",
            "extra": "10570041 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10570041 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10570041 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 112.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10642435 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 112.7,
            "unit": "ns/op",
            "extra": "10642435 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10642435 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10642435 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 238.3,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5032548 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 238.3,
            "unit": "ns/op",
            "extra": "5032548 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5032548 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5032548 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 99.07,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "12609314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 99.07,
            "unit": "ns/op",
            "extra": "12609314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "12609314 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "12609314 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 377.3,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2797562 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 377.3,
            "unit": "ns/op",
            "extra": "2797562 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2797562 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2797562 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 19359,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "61627 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 19359,
            "unit": "ns/op",
            "extra": "61627 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "61627 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "61627 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 407.4,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2956622 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 407.4,
            "unit": "ns/op",
            "extra": "2956622 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2956622 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2956622 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001861,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001861,
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
            "value": 15.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "78719211 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.1,
            "unit": "ns/op",
            "extra": "78719211 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78719211 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78719211 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.55,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "84194780 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.55,
            "unit": "ns/op",
            "extra": "84194780 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "84194780 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "84194780 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.38,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "89204758 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.38,
            "unit": "ns/op",
            "extra": "89204758 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "89204758 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "89204758 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 68.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17799674 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 68.21,
            "unit": "ns/op",
            "extra": "17799674 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17799674 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17799674 times\n4 procs"
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
          "id": "d034513951edebbdb2840e1e5bbd51b3b8491799",
          "message": "chore(main): release 0.51.0 (#1995)",
          "timestamp": "2026-11-16T16:15:35-05:00",
          "tree_id": "54a6933b506bba65f6711210c4597f34fe792bd4",
          "url": "https://github.com/Wikid82/Charon/commit/d034513951edebbdb2840e1e5bbd51b3b8491799"
        },
        "date": 1794863860175,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 116188,
            "unit": "ns/op\t   47175 B/op\t     670 allocs/op",
            "extra": "9038 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 116188,
            "unit": "ns/op",
            "extra": "9038 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47175,
            "unit": "B/op",
            "extra": "9038 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "9038 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 81956,
            "unit": "ns/op\t   42623 B/op\t     519 allocs/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 81956,
            "unit": "ns/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42623,
            "unit": "B/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "14370 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 187523,
            "unit": "ns/op\t   86804 B/op\t    1261 allocs/op",
            "extra": "6057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 187523,
            "unit": "ns/op",
            "extra": "6057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86804,
            "unit": "B/op",
            "extra": "6057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "6057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 93084,
            "unit": "ns/op\t   47637 B/op\t     789 allocs/op",
            "extra": "12931 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 93084,
            "unit": "ns/op",
            "extra": "12931 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47637,
            "unit": "B/op",
            "extra": "12931 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "12931 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 46422,
            "unit": "ns/op\t   20625 B/op\t     236 allocs/op",
            "extra": "25747 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 46422,
            "unit": "ns/op",
            "extra": "25747 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20625,
            "unit": "B/op",
            "extra": "25747 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "25747 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 163251,
            "unit": "ns/op\t   86118 B/op\t    1263 allocs/op",
            "extra": "7215 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 163251,
            "unit": "ns/op",
            "extra": "7215 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86118,
            "unit": "B/op",
            "extra": "7215 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1263,
            "unit": "allocs/op",
            "extra": "7215 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 119059,
            "unit": "ns/op\t   47166 B/op\t     675 allocs/op",
            "extra": "9742 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 119059,
            "unit": "ns/op",
            "extra": "9742 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47166,
            "unit": "B/op",
            "extra": "9742 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "9742 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 690.8,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1739930 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 690.8,
            "unit": "ns/op",
            "extra": "1739930 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1739930 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1739930 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 581.6,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2076577 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 581.6,
            "unit": "ns/op",
            "extra": "2076577 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2076577 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2076577 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.079,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "148508833 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.079,
            "unit": "ns/op",
            "extra": "148508833 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "148508833 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "148508833 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 158383,
            "unit": "ns/op\t   17868 B/op\t     126 allocs/op",
            "extra": "6624 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 158383,
            "unit": "ns/op",
            "extra": "6624 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17868,
            "unit": "B/op",
            "extra": "6624 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6624 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 66.64,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17990935 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 66.64,
            "unit": "ns/op",
            "extra": "17990935 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17990935 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17990935 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 111.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10839066 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 111.4,
            "unit": "ns/op",
            "extra": "10839066 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10839066 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10839066 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 112.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10704379 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 112.4,
            "unit": "ns/op",
            "extra": "10704379 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10704379 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10704379 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 231.6,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5130532 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 231.6,
            "unit": "ns/op",
            "extra": "5130532 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5130532 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5130532 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 93.71,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "12602785 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 93.71,
            "unit": "ns/op",
            "extra": "12602785 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "12602785 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "12602785 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 370,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3260396 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 370,
            "unit": "ns/op",
            "extra": "3260396 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3260396 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3260396 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 19475,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "61057 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 19475,
            "unit": "ns/op",
            "extra": "61057 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "61057 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "61057 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 460.1,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2531138 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 460.1,
            "unit": "ns/op",
            "extra": "2531138 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2531138 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2531138 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002022,
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
            "extra": "78802868 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.05,
            "unit": "ns/op",
            "extra": "78802868 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78802868 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78802868 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.03,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "92122588 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.03,
            "unit": "ns/op",
            "extra": "92122588 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "92122588 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "92122588 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.99,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "82016853 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.99,
            "unit": "ns/op",
            "extra": "82016853 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "82016853 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "82016853 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 68.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15938034 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 68.8,
            "unit": "ns/op",
            "extra": "15938034 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15938034 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15938034 times\n4 procs"
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
          "id": "73326814666298f92322f78deaf26e3a92e44190",
          "message": "chore: promote nightly to main (2026-11-17) (#2003)",
          "timestamp": "2026-11-16T23:35:07-05:00",
          "tree_id": "1abea28dc8f623a928e50fb9b9e157208eb7205e",
          "url": "https://github.com/Wikid82/Charon/commit/73326814666298f92322f78deaf26e3a92e44190"
        },
        "date": 1794890442686,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 243910,
            "unit": "ns/op\t   47179 B/op\t     670 allocs/op",
            "extra": "4407 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 243910,
            "unit": "ns/op",
            "extra": "4407 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47179,
            "unit": "B/op",
            "extra": "4407 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "4407 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 153385,
            "unit": "ns/op\t   42627 B/op\t     519 allocs/op",
            "extra": "7357 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 153385,
            "unit": "ns/op",
            "extra": "7357 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42627,
            "unit": "B/op",
            "extra": "7357 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "7357 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 288971,
            "unit": "ns/op\t   86701 B/op\t    1261 allocs/op",
            "extra": "3956 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 288971,
            "unit": "ns/op",
            "extra": "3956 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86701,
            "unit": "B/op",
            "extra": "3956 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "3956 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 186815,
            "unit": "ns/op\t   47619 B/op\t     789 allocs/op",
            "extra": "6826 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 186815,
            "unit": "ns/op",
            "extra": "6826 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47619,
            "unit": "B/op",
            "extra": "6826 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "6826 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 105407,
            "unit": "ns/op\t   20614 B/op\t     236 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 105407,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20614,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 234978,
            "unit": "ns/op\t   86171 B/op\t    1265 allocs/op",
            "extra": "4634 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 234978,
            "unit": "ns/op",
            "extra": "4634 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86171,
            "unit": "B/op",
            "extra": "4634 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1265,
            "unit": "allocs/op",
            "extra": "4634 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 243063,
            "unit": "ns/op\t   47176 B/op\t     675 allocs/op",
            "extra": "4734 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 243063,
            "unit": "ns/op",
            "extra": "4734 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47176,
            "unit": "B/op",
            "extra": "4734 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "4734 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 876.7,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1374942 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 876.7,
            "unit": "ns/op",
            "extra": "1374942 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1374942 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1374942 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 792.9,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1610634 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 792.9,
            "unit": "ns/op",
            "extra": "1610634 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1610634 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1610634 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 9.683,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "123974301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 9.683,
            "unit": "ns/op",
            "extra": "123974301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "123974301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "123974301 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 321550,
            "unit": "ns/op\t   17889 B/op\t     126 allocs/op",
            "extra": "3562 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 321550,
            "unit": "ns/op",
            "extra": "3562 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17889,
            "unit": "B/op",
            "extra": "3562 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "3562 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 93.35,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12861936 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 93.35,
            "unit": "ns/op",
            "extra": "12861936 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "12861936 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "12861936 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 141.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8527954 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 141.3,
            "unit": "ns/op",
            "extra": "8527954 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8527954 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8527954 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 144.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8300932 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 144.7,
            "unit": "ns/op",
            "extra": "8300932 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8300932 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8300932 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 294.2,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "4116231 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 294.2,
            "unit": "ns/op",
            "extra": "4116231 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "4116231 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "4116231 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 144.5,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "8293734 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 144.5,
            "unit": "ns/op",
            "extra": "8293734 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "8293734 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "8293734 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 521.6,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2297886 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 521.6,
            "unit": "ns/op",
            "extra": "2297886 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2297886 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2297886 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 36343,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "32946 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 36343,
            "unit": "ns/op",
            "extra": "32946 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "32946 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "32946 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 573.5,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2115382 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 573.5,
            "unit": "ns/op",
            "extra": "2115382 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2115382 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2115382 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002075,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002075,
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
            "value": 12.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "94070043 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.52,
            "unit": "ns/op",
            "extra": "94070043 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "94070043 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "94070043 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.49,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95429756 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.49,
            "unit": "ns/op",
            "extra": "95429756 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95429756 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95429756 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96420531 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.48,
            "unit": "ns/op",
            "extra": "96420531 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96420531 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96420531 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 93.14,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12072074 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 93.14,
            "unit": "ns/op",
            "extra": "12072074 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "12072074 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "12072074 times\n4 procs"
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
          "id": "e33ee5f4b7b99f1a85448335d853292f32f8f3de",
          "message": "chore(main): release 0.51.1 (#2004)",
          "timestamp": "2026-11-17T00:04:34-05:00",
          "tree_id": "82a709cde7539e9e40389a88e7b5ed8409b133a3",
          "url": "https://github.com/Wikid82/Charon/commit/e33ee5f4b7b99f1a85448335d853292f32f8f3de"
        },
        "date": 1794892065270,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 135004,
            "unit": "ns/op\t   47179 B/op\t     670 allocs/op",
            "extra": "7593 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 135004,
            "unit": "ns/op",
            "extra": "7593 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47179,
            "unit": "B/op",
            "extra": "7593 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "7593 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 89566,
            "unit": "ns/op\t   42621 B/op\t     519 allocs/op",
            "extra": "13401 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 89566,
            "unit": "ns/op",
            "extra": "13401 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42621,
            "unit": "B/op",
            "extra": "13401 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "13401 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 204733,
            "unit": "ns/op\t   86677 B/op\t    1261 allocs/op",
            "extra": "5530 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 204733,
            "unit": "ns/op",
            "extra": "5530 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86677,
            "unit": "B/op",
            "extra": "5530 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "5530 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 111213,
            "unit": "ns/op\t   47628 B/op\t     789 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 111213,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47628,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 57661,
            "unit": "ns/op\t   20662 B/op\t     238 allocs/op",
            "extra": "19063 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 57661,
            "unit": "ns/op",
            "extra": "19063 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20662,
            "unit": "B/op",
            "extra": "19063 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 238,
            "unit": "allocs/op",
            "extra": "19063 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 147938,
            "unit": "ns/op\t   86156 B/op\t    1266 allocs/op",
            "extra": "7308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 147938,
            "unit": "ns/op",
            "extra": "7308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86156,
            "unit": "B/op",
            "extra": "7308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1266,
            "unit": "allocs/op",
            "extra": "7308 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 135384,
            "unit": "ns/op\t   47176 B/op\t     675 allocs/op",
            "extra": "8209 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 135384,
            "unit": "ns/op",
            "extra": "8209 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47176,
            "unit": "B/op",
            "extra": "8209 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "8209 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 711.8,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1756095 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 711.8,
            "unit": "ns/op",
            "extra": "1756095 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1756095 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1756095 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 582.5,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1855546 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 582.5,
            "unit": "ns/op",
            "extra": "1855546 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1855546 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1855546 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 7.916,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "151395794 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 7.916,
            "unit": "ns/op",
            "extra": "151395794 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "151395794 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "151395794 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 233392,
            "unit": "ns/op\t   17880 B/op\t     126 allocs/op",
            "extra": "4981 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 233392,
            "unit": "ns/op",
            "extra": "4981 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17880,
            "unit": "B/op",
            "extra": "4981 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "4981 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 65.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18459552 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 65.48,
            "unit": "ns/op",
            "extra": "18459552 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "18459552 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "18459552 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 103.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11744308 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 103.5,
            "unit": "ns/op",
            "extra": "11744308 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11744308 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11744308 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 101.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11854850 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 101.2,
            "unit": "ns/op",
            "extra": "11854850 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11854850 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11854850 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 227.3,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5304228 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 227.3,
            "unit": "ns/op",
            "extra": "5304228 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5304228 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5304228 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 99.04,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "11421627 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 99.04,
            "unit": "ns/op",
            "extra": "11421627 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "11421627 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "11421627 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 366.6,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3274960 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 366.6,
            "unit": "ns/op",
            "extra": "3274960 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3274960 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3274960 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 21916,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "55176 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 21916,
            "unit": "ns/op",
            "extra": "55176 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "55176 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "55176 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 431.2,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2681408 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 431.2,
            "unit": "ns/op",
            "extra": "2681408 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2681408 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2681408 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002166,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002166,
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
            "value": 10.79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.79,
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
            "value": 11.27,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.27,
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
            "value": 10.82,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.82,
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
            "value": 69.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18242930 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 69.15,
            "unit": "ns/op",
            "extra": "18242930 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "18242930 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "18242930 times\n4 procs"
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
          "id": "4db4f7d14ee955af7adf43531f38b58d1499cbbe",
          "message": "chore: promote nightly to main (2026-11-23) (#2015)",
          "timestamp": "2026-11-23T08:50:29-05:00",
          "tree_id": "4c7cd1b85d3cabcb8cc58892c0f3ff01a380df62",
          "url": "https://github.com/Wikid82/Charon/commit/4db4f7d14ee955af7adf43531f38b58d1499cbbe"
        },
        "date": 1795442148354,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 139589,
            "unit": "ns/op\t   47177 B/op\t     670 allocs/op",
            "extra": "8616 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 139589,
            "unit": "ns/op",
            "extra": "8616 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47177,
            "unit": "B/op",
            "extra": "8616 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "8616 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 92227,
            "unit": "ns/op\t   42624 B/op\t     519 allocs/op",
            "extra": "13117 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 92227,
            "unit": "ns/op",
            "extra": "13117 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42624,
            "unit": "B/op",
            "extra": "13117 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "13117 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 208129,
            "unit": "ns/op\t   86708 B/op\t    1261 allocs/op",
            "extra": "5583 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 208129,
            "unit": "ns/op",
            "extra": "5583 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86708,
            "unit": "B/op",
            "extra": "5583 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "5583 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 103287,
            "unit": "ns/op\t   47636 B/op\t     789 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 103287,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47636,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 55692,
            "unit": "ns/op\t   20627 B/op\t     236 allocs/op",
            "extra": "21638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 55692,
            "unit": "ns/op",
            "extra": "21638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20627,
            "unit": "B/op",
            "extra": "21638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "21638 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 186704,
            "unit": "ns/op\t   86120 B/op\t    1264 allocs/op",
            "extra": "6204 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 186704,
            "unit": "ns/op",
            "extra": "6204 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86120,
            "unit": "B/op",
            "extra": "6204 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "6204 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 137448,
            "unit": "ns/op\t   47169 B/op\t     675 allocs/op",
            "extra": "8589 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 137448,
            "unit": "ns/op",
            "extra": "8589 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47169,
            "unit": "B/op",
            "extra": "8589 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "8589 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 689.2,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1741352 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 689.2,
            "unit": "ns/op",
            "extra": "1741352 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1741352 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1741352 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 600.5,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2037862 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 600.5,
            "unit": "ns/op",
            "extra": "2037862 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2037862 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2037862 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 7.935,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "151491271 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 7.935,
            "unit": "ns/op",
            "extra": "151491271 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "151491271 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "151491271 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 230811,
            "unit": "ns/op\t   17886 B/op\t     126 allocs/op",
            "extra": "4910 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 230811,
            "unit": "ns/op",
            "extra": "4910 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17886,
            "unit": "B/op",
            "extra": "4910 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "4910 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 64.77,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18617446 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 64.77,
            "unit": "ns/op",
            "extra": "18617446 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "18617446 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "18617446 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 102.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11806291 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 102.5,
            "unit": "ns/op",
            "extra": "11806291 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11806291 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11806291 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 104.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11831054 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 104.8,
            "unit": "ns/op",
            "extra": "11831054 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11831054 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11831054 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 236.4,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5179224 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 236.4,
            "unit": "ns/op",
            "extra": "5179224 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5179224 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5179224 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 98.63,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "12074212 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 98.63,
            "unit": "ns/op",
            "extra": "12074212 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "12074212 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "12074212 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 418.8,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3265302 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 418.8,
            "unit": "ns/op",
            "extra": "3265302 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3265302 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3265302 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 23637,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "46970 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 23637,
            "unit": "ns/op",
            "extra": "46970 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "46970 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "46970 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 432.6,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2638801 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 432.6,
            "unit": "ns/op",
            "extra": "2638801 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2638801 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2638801 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001757,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001757,
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
            "value": 10.66,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.66,
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
            "value": 11.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.19,
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
            "value": 10.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 10.69,
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
            "value": 69.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17709010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 69.59,
            "unit": "ns/op",
            "extra": "17709010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17709010 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17709010 times\n4 procs"
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
          "id": "52063b3f25a234d3038e65212d34b212269d9190",
          "message": "chore(main): release 0.51.2 (#2016)",
          "timestamp": "2026-11-23T09:20:58-05:00",
          "tree_id": "c860309368e7993c61f23e5438c43babe5aa689c",
          "url": "https://github.com/Wikid82/Charon/commit/52063b3f25a234d3038e65212d34b212269d9190"
        },
        "date": 1795444103754,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 245976,
            "unit": "ns/op\t   47182 B/op\t     670 allocs/op",
            "extra": "4321 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 245976,
            "unit": "ns/op",
            "extra": "4321 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47182,
            "unit": "B/op",
            "extra": "4321 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "4321 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 160221,
            "unit": "ns/op\t   42630 B/op\t     519 allocs/op",
            "extra": "7236 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 160221,
            "unit": "ns/op",
            "extra": "7236 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42630,
            "unit": "B/op",
            "extra": "7236 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "7236 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 306920,
            "unit": "ns/op\t   86620 B/op\t    1261 allocs/op",
            "extra": "3910 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 306920,
            "unit": "ns/op",
            "extra": "3910 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86620,
            "unit": "B/op",
            "extra": "3910 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "3910 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 167811,
            "unit": "ns/op\t   47652 B/op\t     790 allocs/op",
            "extra": "6789 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 167811,
            "unit": "ns/op",
            "extra": "6789 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47652,
            "unit": "B/op",
            "extra": "6789 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 790,
            "unit": "allocs/op",
            "extra": "6789 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 101614,
            "unit": "ns/op\t   20625 B/op\t     236 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 101614,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20625,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 237677,
            "unit": "ns/op\t   86211 B/op\t    1265 allocs/op",
            "extra": "4914 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 237677,
            "unit": "ns/op",
            "extra": "4914 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86211,
            "unit": "B/op",
            "extra": "4914 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1265,
            "unit": "allocs/op",
            "extra": "4914 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 246503,
            "unit": "ns/op\t   47174 B/op\t     675 allocs/op",
            "extra": "4550 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 246503,
            "unit": "ns/op",
            "extra": "4550 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47174,
            "unit": "B/op",
            "extra": "4550 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "4550 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 904.4,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1307290 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 904.4,
            "unit": "ns/op",
            "extra": "1307290 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1307290 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1307290 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 767.7,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1541756 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 767.7,
            "unit": "ns/op",
            "extra": "1541756 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1541756 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1541756 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 9.676,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "124155720 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 9.676,
            "unit": "ns/op",
            "extra": "124155720 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "124155720 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "124155720 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 332023,
            "unit": "ns/op\t   17870 B/op\t     126 allocs/op",
            "extra": "3441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 332023,
            "unit": "ns/op",
            "extra": "3441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17870,
            "unit": "B/op",
            "extra": "3441 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "3441 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 93.89,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12851180 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 93.89,
            "unit": "ns/op",
            "extra": "12851180 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "12851180 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "12851180 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 141.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8545538 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 141.1,
            "unit": "ns/op",
            "extra": "8545538 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8545538 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8545538 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 144.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8277111 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 144.6,
            "unit": "ns/op",
            "extra": "8277111 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8277111 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8277111 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 314.4,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "4111174 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 314.4,
            "unit": "ns/op",
            "extra": "4111174 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "4111174 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "4111174 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 169.4,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "7966525 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 169.4,
            "unit": "ns/op",
            "extra": "7966525 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "7966525 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "7966525 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 610.5,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "1751016 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 610.5,
            "unit": "ns/op",
            "extra": "1751016 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "1751016 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "1751016 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 37103,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "31615 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 37103,
            "unit": "ns/op",
            "extra": "31615 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "31615 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "31615 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 584.5,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2037673 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 584.5,
            "unit": "ns/op",
            "extra": "2037673 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2037673 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2037673 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002448,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002448,
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
            "value": 12.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "86660575 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.48,
            "unit": "ns/op",
            "extra": "86660575 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "86660575 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "86660575 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95814349 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.52,
            "unit": "ns/op",
            "extra": "95814349 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "95814349 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "95814349 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96037522 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.48,
            "unit": "ns/op",
            "extra": "96037522 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96037522 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96037522 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 106.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10670972 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 106.5,
            "unit": "ns/op",
            "extra": "10670972 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10670972 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10670972 times\n4 procs"
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
          "id": "b5ba1120611e26a0eca8afc618785eb0f91e22e9",
          "message": "chore: promote nightly to main (2026-11-24) (#2027)",
          "timestamp": "2026-11-23T20:34:37-05:00",
          "tree_id": "635693bb37042eb7b97f54cacffb8809b31bf877",
          "url": "https://github.com/Wikid82/Charon/commit/b5ba1120611e26a0eca8afc618785eb0f91e22e9"
        },
        "date": 1795484330820,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 235563,
            "unit": "ns/op\t   47181 B/op\t     670 allocs/op",
            "extra": "4447 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 235563,
            "unit": "ns/op",
            "extra": "4447 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47181,
            "unit": "B/op",
            "extra": "4447 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "4447 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 179067,
            "unit": "ns/op\t   42616 B/op\t     519 allocs/op",
            "extra": "7472 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 179067,
            "unit": "ns/op",
            "extra": "7472 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42616,
            "unit": "B/op",
            "extra": "7472 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "7472 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 285108,
            "unit": "ns/op\t   86787 B/op\t    1261 allocs/op",
            "extra": "4099 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 285108,
            "unit": "ns/op",
            "extra": "4099 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86787,
            "unit": "B/op",
            "extra": "4099 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "4099 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 153837,
            "unit": "ns/op\t   47650 B/op\t     789 allocs/op",
            "extra": "7706 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 153837,
            "unit": "ns/op",
            "extra": "7706 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47650,
            "unit": "B/op",
            "extra": "7706 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "7706 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 93114,
            "unit": "ns/op\t   20627 B/op\t     236 allocs/op",
            "extra": "12721 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 93114,
            "unit": "ns/op",
            "extra": "12721 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20627,
            "unit": "B/op",
            "extra": "12721 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "12721 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 286940,
            "unit": "ns/op\t   86267 B/op\t    1264 allocs/op",
            "extra": "4108 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 286940,
            "unit": "ns/op",
            "extra": "4108 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86267,
            "unit": "B/op",
            "extra": "4108 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "4108 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 258310,
            "unit": "ns/op\t   47158 B/op\t     675 allocs/op",
            "extra": "4526 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 258310,
            "unit": "ns/op",
            "extra": "4526 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47158,
            "unit": "B/op",
            "extra": "4526 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "4526 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 868.8,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1370314 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 868.8,
            "unit": "ns/op",
            "extra": "1370314 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1370314 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1370314 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 741.9,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1584582 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 741.9,
            "unit": "ns/op",
            "extra": "1584582 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1584582 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1584582 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 9.672,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "123925477 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 9.672,
            "unit": "ns/op",
            "extra": "123925477 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "123925477 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "123925477 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 333608,
            "unit": "ns/op\t   17877 B/op\t     126 allocs/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 333608,
            "unit": "ns/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17877,
            "unit": "B/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "3566 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 93.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12821890 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 93.45,
            "unit": "ns/op",
            "extra": "12821890 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "12821890 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "12821890 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 140.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8531856 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 140.8,
            "unit": "ns/op",
            "extra": "8531856 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8531856 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8531856 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 144.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8179399 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 144.8,
            "unit": "ns/op",
            "extra": "8179399 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8179399 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8179399 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 289,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "4217949 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 289,
            "unit": "ns/op",
            "extra": "4217949 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "4217949 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "4217949 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 159.3,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "7489711 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 159.3,
            "unit": "ns/op",
            "extra": "7489711 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "7489711 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "7489711 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 533.6,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2264582 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 533.6,
            "unit": "ns/op",
            "extra": "2264582 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2264582 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2264582 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 36978,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "32997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 36978,
            "unit": "ns/op",
            "extra": "32997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "32997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "32997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 686.8,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "1644913 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 686.8,
            "unit": "ns/op",
            "extra": "1644913 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "1644913 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1644913 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002316,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002316,
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
            "value": 12.49,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "91544806 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.49,
            "unit": "ns/op",
            "extra": "91544806 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "91544806 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "91544806 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96088470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.5,
            "unit": "ns/op",
            "extra": "96088470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96088470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96088470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 12.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "96099420 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 12.48,
            "unit": "ns/op",
            "extra": "96099420 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "96099420 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "96099420 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 91.83,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13305228 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 91.83,
            "unit": "ns/op",
            "extra": "13305228 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "13305228 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "13305228 times\n4 procs"
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
          "id": "18fc3462e78100656464cca4779285cbd0539c39",
          "message": "chore(main): release 0.51.3 (#2028)",
          "timestamp": "2026-11-23T21:13:47-05:00",
          "tree_id": "cb22c97e78dbdf8e4849327517bedc9c3841fa26",
          "url": "https://github.com/Wikid82/Charon/commit/18fc3462e78100656464cca4779285cbd0539c39"
        },
        "date": 1795486760243,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 198091,
            "unit": "ns/op\t   47184 B/op\t     670 allocs/op",
            "extra": "5594 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 198091,
            "unit": "ns/op",
            "extra": "5594 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47184,
            "unit": "B/op",
            "extra": "5594 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "5594 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 138607,
            "unit": "ns/op\t   42628 B/op\t     519 allocs/op",
            "extra": "8893 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 138607,
            "unit": "ns/op",
            "extra": "8893 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42628,
            "unit": "B/op",
            "extra": "8893 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "8893 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 292012,
            "unit": "ns/op\t   86640 B/op\t    1261 allocs/op",
            "extra": "4024 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 292012,
            "unit": "ns/op",
            "extra": "4024 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86640,
            "unit": "B/op",
            "extra": "4024 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "4024 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 162210,
            "unit": "ns/op\t   47595 B/op\t     789 allocs/op",
            "extra": "6396 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 162210,
            "unit": "ns/op",
            "extra": "6396 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47595,
            "unit": "B/op",
            "extra": "6396 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "6396 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 77243,
            "unit": "ns/op\t   20636 B/op\t     236 allocs/op",
            "extra": "13988 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 77243,
            "unit": "ns/op",
            "extra": "13988 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20636,
            "unit": "B/op",
            "extra": "13988 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "13988 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 219351,
            "unit": "ns/op\t   86152 B/op\t    1265 allocs/op",
            "extra": "5044 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 219351,
            "unit": "ns/op",
            "extra": "5044 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86152,
            "unit": "B/op",
            "extra": "5044 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1265,
            "unit": "allocs/op",
            "extra": "5044 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 193248,
            "unit": "ns/op\t   47178 B/op\t     675 allocs/op",
            "extra": "6034 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 193248,
            "unit": "ns/op",
            "extra": "6034 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47178,
            "unit": "B/op",
            "extra": "6034 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "6034 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 1135,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "983103 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 1135,
            "unit": "ns/op",
            "extra": "983103 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "983103 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "983103 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 845,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1434388 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 845,
            "unit": "ns/op",
            "extra": "1434388 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1434388 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1434388 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "133752328 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.98,
            "unit": "ns/op",
            "extra": "133752328 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "133752328 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "133752328 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 234740,
            "unit": "ns/op\t   17881 B/op\t     126 allocs/op",
            "extra": "4262 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 234740,
            "unit": "ns/op",
            "extra": "4262 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17881,
            "unit": "B/op",
            "extra": "4262 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "4262 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 79.44,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15274644 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 79.44,
            "unit": "ns/op",
            "extra": "15274644 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15274644 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15274644 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 126.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9457566 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 126.5,
            "unit": "ns/op",
            "extra": "9457566 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "9457566 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "9457566 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 126.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9436998 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 126.8,
            "unit": "ns/op",
            "extra": "9436998 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "9436998 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "9436998 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 337.6,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "3912352 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 337.6,
            "unit": "ns/op",
            "extra": "3912352 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "3912352 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "3912352 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 141.1,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "8505870 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 141.1,
            "unit": "ns/op",
            "extra": "8505870 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "8505870 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "8505870 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 523.3,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "2226918 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 523.3,
            "unit": "ns/op",
            "extra": "2226918 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "2226918 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "2226918 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 31984,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "37412 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 31984,
            "unit": "ns/op",
            "extra": "37412 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "37412 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "37412 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 609.3,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "1963006 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 609.3,
            "unit": "ns/op",
            "extra": "1963006 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "1963006 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "1963006 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002547,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002547,
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
            "value": 11.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.69,
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
            "value": 11.62,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.62,
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
            "value": 11.69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 11.69,
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
            "value": 79.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14432979 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 79.24,
            "unit": "ns/op",
            "extra": "14432979 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "14432979 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "14432979 times\n4 procs"
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
          "id": "86307421218814a9e3fa460a22f62c47af858735",
          "message": "chore: promote nightly to main (2026-11-24) (#2040)",
          "timestamp": "2026-11-24T12:02:58-05:00",
          "tree_id": "6c9ab852204e2612e274843fda245b060a9cf6c3",
          "url": "https://github.com/Wikid82/Charon/commit/86307421218814a9e3fa460a22f62c47af858735"
        },
        "date": 1795540060186,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 112053,
            "unit": "ns/op\t   47179 B/op\t     670 allocs/op",
            "extra": "9595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 112053,
            "unit": "ns/op",
            "extra": "9595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47179,
            "unit": "B/op",
            "extra": "9595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "9595 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 81141,
            "unit": "ns/op\t   42621 B/op\t     519 allocs/op",
            "extra": "14976 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 81141,
            "unit": "ns/op",
            "extra": "14976 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42621,
            "unit": "B/op",
            "extra": "14976 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "14976 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 211123,
            "unit": "ns/op\t   86517 B/op\t    1261 allocs/op",
            "extra": "5528 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 211123,
            "unit": "ns/op",
            "extra": "5528 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86517,
            "unit": "B/op",
            "extra": "5528 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "5528 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 91161,
            "unit": "ns/op\t   47632 B/op\t     789 allocs/op",
            "extra": "12409 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 91161,
            "unit": "ns/op",
            "extra": "12409 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47632,
            "unit": "B/op",
            "extra": "12409 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "12409 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 46069,
            "unit": "ns/op\t   20622 B/op\t     236 allocs/op",
            "extra": "25792 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 46069,
            "unit": "ns/op",
            "extra": "25792 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20622,
            "unit": "B/op",
            "extra": "25792 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "25792 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 120758,
            "unit": "ns/op\t   86025 B/op\t    1263 allocs/op",
            "extra": "9176 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 120758,
            "unit": "ns/op",
            "extra": "9176 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86025,
            "unit": "B/op",
            "extra": "9176 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1263,
            "unit": "allocs/op",
            "extra": "9176 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 113858,
            "unit": "ns/op\t   47166 B/op\t     675 allocs/op",
            "extra": "9998 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 113858,
            "unit": "ns/op",
            "extra": "9998 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47166,
            "unit": "B/op",
            "extra": "9998 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "9998 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 689.9,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1717839 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 689.9,
            "unit": "ns/op",
            "extra": "1717839 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1717839 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1717839 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 579.3,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2076855 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 579.3,
            "unit": "ns/op",
            "extra": "2076855 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2076855 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2076855 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.054,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "149112332 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.054,
            "unit": "ns/op",
            "extra": "149112332 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "149112332 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "149112332 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 157386,
            "unit": "ns/op\t   17879 B/op\t     126 allocs/op",
            "extra": "7567 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 157386,
            "unit": "ns/op",
            "extra": "7567 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17879,
            "unit": "B/op",
            "extra": "7567 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "7567 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 66.11,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18001897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 66.11,
            "unit": "ns/op",
            "extra": "18001897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "18001897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "18001897 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 111,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10831148 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 111,
            "unit": "ns/op",
            "extra": "10831148 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10831148 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10831148 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 111.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10308694 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 111.9,
            "unit": "ns/op",
            "extra": "10308694 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10308694 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10308694 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 235.1,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5113726 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 235.1,
            "unit": "ns/op",
            "extra": "5113726 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5113726 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5113726 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 89.25,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "13362998 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 89.25,
            "unit": "ns/op",
            "extra": "13362998 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "13362998 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "13362998 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 370.8,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3231525 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 370.8,
            "unit": "ns/op",
            "extra": "3231525 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3231525 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3231525 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 19284,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "62239 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 19284,
            "unit": "ns/op",
            "extra": "62239 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "62239 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "62239 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 407.9,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2924316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 407.9,
            "unit": "ns/op",
            "extra": "2924316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2924316 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2924316 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001749,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001749,
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
            "extra": "78946593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.05,
            "unit": "ns/op",
            "extra": "78946593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78946593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78946593 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.51,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "87963480 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.51,
            "unit": "ns/op",
            "extra": "87963480 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "87963480 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "87963480 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 14.54,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "83198816 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 14.54,
            "unit": "ns/op",
            "extra": "83198816 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "83198816 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "83198816 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 68.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17543012 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 68.15,
            "unit": "ns/op",
            "extra": "17543012 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17543012 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17543012 times\n4 procs"
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
          "id": "57c78b96f0367a6f24300d300ee795a3f781d3bd",
          "message": "chore(main): release 0.51.4 (#2041)",
          "timestamp": "2026-11-24T12:44:41-05:00",
          "tree_id": "e94426b3848148f08ff58ad6938eafa705f0ebd6",
          "url": "https://github.com/Wikid82/Charon/commit/57c78b96f0367a6f24300d300ee795a3f781d3bd"
        },
        "date": 1795542500386,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 112075,
            "unit": "ns/op\t   47175 B/op\t     670 allocs/op",
            "extra": "9558 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 112075,
            "unit": "ns/op",
            "extra": "9558 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47175,
            "unit": "B/op",
            "extra": "9558 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "9558 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 81222,
            "unit": "ns/op\t   42624 B/op\t     519 allocs/op",
            "extra": "14768 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 81222,
            "unit": "ns/op",
            "extra": "14768 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42624,
            "unit": "B/op",
            "extra": "14768 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "14768 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 190256,
            "unit": "ns/op\t   86591 B/op\t    1261 allocs/op",
            "extra": "6120 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 190256,
            "unit": "ns/op",
            "extra": "6120 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86591,
            "unit": "B/op",
            "extra": "6120 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "6120 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 104118,
            "unit": "ns/op\t   47604 B/op\t     789 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 104118,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47604,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 46743,
            "unit": "ns/op\t   20618 B/op\t     236 allocs/op",
            "extra": "23980 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 46743,
            "unit": "ns/op",
            "extra": "23980 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20618,
            "unit": "B/op",
            "extra": "23980 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "23980 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 125339,
            "unit": "ns/op\t   86144 B/op\t    1263 allocs/op",
            "extra": "9088 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 125339,
            "unit": "ns/op",
            "extra": "9088 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86144,
            "unit": "B/op",
            "extra": "9088 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1263,
            "unit": "allocs/op",
            "extra": "9088 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 113411,
            "unit": "ns/op\t   47165 B/op\t     675 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 113411,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47165,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 708,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1724193 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 708,
            "unit": "ns/op",
            "extra": "1724193 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1724193 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1724193 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 591.8,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1917495 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 591.8,
            "unit": "ns/op",
            "extra": "1917495 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1917495 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1917495 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.114,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "148274830 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.114,
            "unit": "ns/op",
            "extra": "148274830 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "148274830 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "148274830 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 163337,
            "unit": "ns/op\t   17883 B/op\t     126 allocs/op",
            "extra": "6834 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 163337,
            "unit": "ns/op",
            "extra": "6834 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17883,
            "unit": "B/op",
            "extra": "6834 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6834 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 66.97,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18055206 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 66.97,
            "unit": "ns/op",
            "extra": "18055206 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "18055206 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "18055206 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 111.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10737661 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 111.8,
            "unit": "ns/op",
            "extra": "10737661 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10737661 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10737661 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 112.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10702382 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 112.7,
            "unit": "ns/op",
            "extra": "10702382 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10702382 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10702382 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 242.8,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "5096850 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 242.8,
            "unit": "ns/op",
            "extra": "5096850 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "5096850 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "5096850 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 90.27,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "13299242 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 90.27,
            "unit": "ns/op",
            "extra": "13299242 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "13299242 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "13299242 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 372.4,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3237092 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 372.4,
            "unit": "ns/op",
            "extra": "3237092 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3237092 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3237092 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 19426,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "61227 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 19426,
            "unit": "ns/op",
            "extra": "61227 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "61227 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "61227 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 412,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2928112 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 412,
            "unit": "ns/op",
            "extra": "2928112 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2928112 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2928112 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002157,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002157,
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
            "value": 15.14,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "78448994 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.14,
            "unit": "ns/op",
            "extra": "78448994 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78448994 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78448994 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.61,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "84196741 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.61,
            "unit": "ns/op",
            "extra": "84196741 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "84196741 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "84196741 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 14.65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "86940254 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 14.65,
            "unit": "ns/op",
            "extra": "86940254 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "86940254 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "86940254 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 78.46,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17345521 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 78.46,
            "unit": "ns/op",
            "extra": "17345521 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17345521 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17345521 times\n4 procs"
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
          "id": "6baab1d310ac0388a267cdd69b78be3ded1281c2",
          "message": "chore: promote nightly to main (2026-11-25) (#2052)",
          "timestamp": "2026-11-25T01:51:38-05:00",
          "tree_id": "7070ebba40851248be20ffa33e2fb8b8570b31c3",
          "url": "https://github.com/Wikid82/Charon/commit/6baab1d310ac0388a267cdd69b78be3ded1281c2"
        },
        "date": 1795589724654,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 111181,
            "unit": "ns/op\t   47176 B/op\t     670 allocs/op",
            "extra": "9639 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 111181,
            "unit": "ns/op",
            "extra": "9639 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47176,
            "unit": "B/op",
            "extra": "9639 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "9639 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 80447,
            "unit": "ns/op\t   42622 B/op\t     519 allocs/op",
            "extra": "14842 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 80447,
            "unit": "ns/op",
            "extra": "14842 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42622,
            "unit": "B/op",
            "extra": "14842 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "14842 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 188437,
            "unit": "ns/op\t   86593 B/op\t    1261 allocs/op",
            "extra": "6090 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 188437,
            "unit": "ns/op",
            "extra": "6090 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86593,
            "unit": "B/op",
            "extra": "6090 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "6090 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 100601,
            "unit": "ns/op\t   47612 B/op\t     789 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 100601,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47612,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 47648,
            "unit": "ns/op\t   20611 B/op\t     236 allocs/op",
            "extra": "23847 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 47648,
            "unit": "ns/op",
            "extra": "23847 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20611,
            "unit": "B/op",
            "extra": "23847 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "23847 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 120784,
            "unit": "ns/op\t   86067 B/op\t    1264 allocs/op",
            "extra": "8440 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 120784,
            "unit": "ns/op",
            "extra": "8440 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86067,
            "unit": "B/op",
            "extra": "8440 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "8440 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 114443,
            "unit": "ns/op\t   47171 B/op\t     675 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 114443,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47171,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 742.9,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1734150 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 742.9,
            "unit": "ns/op",
            "extra": "1734150 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1734150 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1734150 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 588.9,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2043340 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 588.9,
            "unit": "ns/op",
            "extra": "2043340 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2043340 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2043340 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 8.058,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "148754592 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 8.058,
            "unit": "ns/op",
            "extra": "148754592 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "148754592 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "148754592 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 159528,
            "unit": "ns/op\t   17875 B/op\t     126 allocs/op",
            "extra": "7068 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 159528,
            "unit": "ns/op",
            "extra": "7068 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17875,
            "unit": "B/op",
            "extra": "7068 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "7068 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 66.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17969634 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 66.19,
            "unit": "ns/op",
            "extra": "17969634 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17969634 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17969634 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 112.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10742625 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 112.1,
            "unit": "ns/op",
            "extra": "10742625 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10742625 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10742625 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 111.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10762956 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 111.9,
            "unit": "ns/op",
            "extra": "10762956 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "10762956 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "10762956 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 249.6,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "4603225 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 249.6,
            "unit": "ns/op",
            "extra": "4603225 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "4603225 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "4603225 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 89.75,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "13269349 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 89.75,
            "unit": "ns/op",
            "extra": "13269349 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "13269349 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "13269349 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 379.4,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "3163648 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 379.4,
            "unit": "ns/op",
            "extra": "3163648 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "3163648 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "3163648 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 19270,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "62126 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 19270,
            "unit": "ns/op",
            "extra": "62126 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "62126 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "62126 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 394.4,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "3044222 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 394.4,
            "unit": "ns/op",
            "extra": "3044222 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "3044222 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "3044222 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001601,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001601,
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
            "value": 15.13,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "78706170 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 15.13,
            "unit": "ns/op",
            "extra": "78706170 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "78706170 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "78706170 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.05,
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
            "value": 13.97,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "93758019 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.97,
            "unit": "ns/op",
            "extra": "93758019 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "93758019 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "93758019 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 70.28,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15866853 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 70.28,
            "unit": "ns/op",
            "extra": "15866853 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15866853 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15866853 times\n4 procs"
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
          "id": "81a20ca7bf61c64de9830a9dd7c8a8e646ebd239",
          "message": "chore(main): release 0.51.5 (#2053)",
          "timestamp": "2026-11-25T02:20:21-05:00",
          "tree_id": "2fb7039fd3d08128be67793721c2234eff5b0d96",
          "url": "https://github.com/Wikid82/Charon/commit/81a20ca7bf61c64de9830a9dd7c8a8e646ebd239"
        },
        "date": 1795591550936,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 150139,
            "unit": "ns/op\t   47183 B/op\t     670 allocs/op",
            "extra": "7168 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 150139,
            "unit": "ns/op",
            "extra": "7168 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47183,
            "unit": "B/op",
            "extra": "7168 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "7168 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 109019,
            "unit": "ns/op\t   42625 B/op\t     519 allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 109019,
            "unit": "ns/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42625,
            "unit": "B/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "10000 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 256798,
            "unit": "ns/op\t   86618 B/op\t    1261 allocs/op",
            "extra": "4563 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 256798,
            "unit": "ns/op",
            "extra": "4563 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86618,
            "unit": "B/op",
            "extra": "4563 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "4563 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 125429,
            "unit": "ns/op\t   47635 B/op\t     789 allocs/op",
            "extra": "8974 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 125429,
            "unit": "ns/op",
            "extra": "8974 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47635,
            "unit": "B/op",
            "extra": "8974 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "8974 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 64102,
            "unit": "ns/op\t   20621 B/op\t     236 allocs/op",
            "extra": "19234 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 64102,
            "unit": "ns/op",
            "extra": "19234 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20621,
            "unit": "B/op",
            "extra": "19234 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "19234 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 202020,
            "unit": "ns/op\t   86156 B/op\t    1264 allocs/op",
            "extra": "5966 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 202020,
            "unit": "ns/op",
            "extra": "5966 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86156,
            "unit": "B/op",
            "extra": "5966 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "5966 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 153380,
            "unit": "ns/op\t   47177 B/op\t     675 allocs/op",
            "extra": "7092 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 153380,
            "unit": "ns/op",
            "extra": "7092 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47177,
            "unit": "B/op",
            "extra": "7092 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "7092 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 922.1,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1305601 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 922.1,
            "unit": "ns/op",
            "extra": "1305601 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1305601 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1305601 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 821.6,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "1535902 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 821.6,
            "unit": "ns/op",
            "extra": "1535902 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "1535902 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "1535902 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 10.43,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 10.43,
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
            "value": 221827,
            "unit": "ns/op\t   17874 B/op\t     126 allocs/op",
            "extra": "5361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 221827,
            "unit": "ns/op",
            "extra": "5361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17874,
            "unit": "B/op",
            "extra": "5361 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "5361 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 95.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12746083 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 95.98,
            "unit": "ns/op",
            "extra": "12746083 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "12746083 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "12746083 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 143.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8385028 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 143.1,
            "unit": "ns/op",
            "extra": "8385028 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8385028 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8385028 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 155.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8070067 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 155.8,
            "unit": "ns/op",
            "extra": "8070067 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "8070067 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "8070067 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 294.4,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "4030006 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 294.4,
            "unit": "ns/op",
            "extra": "4030006 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "4030006 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "4030006 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 140.6,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "8387596 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 140.6,
            "unit": "ns/op",
            "extra": "8387596 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "8387596 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "8387596 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 589,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "1903964 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 589,
            "unit": "ns/op",
            "extra": "1903964 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "1903964 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "1903964 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 25966,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "45079 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 25966,
            "unit": "ns/op",
            "extra": "45079 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "45079 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "45079 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 535.3,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "2233527 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 535.3,
            "unit": "ns/op",
            "extra": "2233527 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "2233527 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "2233527 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0002846,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0002846,
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
            "value": 13.94,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "87005470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.94,
            "unit": "ns/op",
            "extra": "87005470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "87005470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "87005470 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "88295665 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.71,
            "unit": "ns/op",
            "extra": "88295665 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "88295665 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "88295665 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 13.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "88845643 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 13.59,
            "unit": "ns/op",
            "extra": "88845643 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "88845643 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "88845643 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 102.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11847439 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 102.6,
            "unit": "ns/op",
            "extra": "11847439 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "11847439 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "11847439 times\n4 procs"
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
          "id": "518ea4b45a99ed599c01f11cbb6b6e8cc8ff99fa",
          "message": "chore: promote nightly to main (2026-11-25) (#2060)",
          "timestamp": "2026-11-25T12:00:45-05:00",
          "tree_id": "3f30a29730250401cd25e1fcecc3819fc0aab6db",
          "url": "https://github.com/Wikid82/Charon/commit/518ea4b45a99ed599c01f11cbb6b6e8cc8ff99fa"
        },
        "date": 1795626314626,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 84155,
            "unit": "ns/op\t   47175 B/op\t     670 allocs/op",
            "extra": "14275 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 84155,
            "unit": "ns/op",
            "extra": "14275 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47175,
            "unit": "B/op",
            "extra": "14275 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "14275 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 61858,
            "unit": "ns/op\t   42622 B/op\t     519 allocs/op",
            "extra": "19178 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 61858,
            "unit": "ns/op",
            "extra": "19178 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42622,
            "unit": "B/op",
            "extra": "19178 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "19178 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 148245,
            "unit": "ns/op\t   86638 B/op\t    1261 allocs/op",
            "extra": "7785 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 148245,
            "unit": "ns/op",
            "extra": "7785 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86638,
            "unit": "B/op",
            "extra": "7785 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "7785 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 72000,
            "unit": "ns/op\t   47623 B/op\t     789 allocs/op",
            "extra": "16507 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 72000,
            "unit": "ns/op",
            "extra": "16507 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47623,
            "unit": "B/op",
            "extra": "16507 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "16507 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 34875,
            "unit": "ns/op\t   20626 B/op\t     236 allocs/op",
            "extra": "35330 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 34875,
            "unit": "ns/op",
            "extra": "35330 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20626,
            "unit": "B/op",
            "extra": "35330 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 236,
            "unit": "allocs/op",
            "extra": "35330 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 127206,
            "unit": "ns/op\t   86068 B/op\t    1263 allocs/op",
            "extra": "9462 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 127206,
            "unit": "ns/op",
            "extra": "9462 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86068,
            "unit": "B/op",
            "extra": "9462 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1263,
            "unit": "allocs/op",
            "extra": "9462 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 86114,
            "unit": "ns/op\t   47173 B/op\t     675 allocs/op",
            "extra": "12414 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 86114,
            "unit": "ns/op",
            "extra": "12414 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47173,
            "unit": "B/op",
            "extra": "12414 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "12414 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 551.6,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "2160372 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 551.6,
            "unit": "ns/op",
            "extra": "2160372 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "2160372 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "2160372 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 439.9,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2726382 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 439.9,
            "unit": "ns/op",
            "extra": "2726382 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2726382 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2726382 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 4.374,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "274802325 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 4.374,
            "unit": "ns/op",
            "extra": "274802325 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "274802325 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "274802325 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 175243,
            "unit": "ns/op\t   17880 B/op\t     126 allocs/op",
            "extra": "6470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 175243,
            "unit": "ns/op",
            "extra": "6470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17880,
            "unit": "B/op",
            "extra": "6470 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "6470 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 44.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27944665 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 44.05,
            "unit": "ns/op",
            "extra": "27944665 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "27944665 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "27944665 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 68.82,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17357260 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 68.82,
            "unit": "ns/op",
            "extra": "17357260 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "17357260 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "17357260 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 72.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16538469 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 72.05,
            "unit": "ns/op",
            "extra": "16538469 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "16538469 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "16538469 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 165.1,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "6958677 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 165.1,
            "unit": "ns/op",
            "extra": "6958677 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "6958677 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "6958677 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 73.11,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "15496669 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 73.11,
            "unit": "ns/op",
            "extra": "15496669 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "15496669 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "15496669 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 246.3,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "4896387 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 246.3,
            "unit": "ns/op",
            "extra": "4896387 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "4896387 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "4896387 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 15411,
            "unit": "ns/op\t    8235 B/op\t     165 allocs/op",
            "extra": "75927 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 15411,
            "unit": "ns/op",
            "extra": "75927 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8235,
            "unit": "B/op",
            "extra": "75927 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "75927 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 362.9,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "3901218 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 362.9,
            "unit": "ns/op",
            "extra": "3901218 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "3901218 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "3901218 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.0001736,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.0001736,
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
            "value": 4.683,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "266293880 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.683,
            "unit": "ns/op",
            "extra": "266293880 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "266293880 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "266293880 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.636,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "268550612 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.636,
            "unit": "ns/op",
            "extra": "268550612 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "268550612 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "268550612 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.569,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "254814282 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.569,
            "unit": "ns/op",
            "extra": "254814282 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "254814282 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "254814282 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 51.64,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25042867 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 51.64,
            "unit": "ns/op",
            "extra": "25042867 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "25042867 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "25042867 times\n4 procs"
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
          "id": "0f450ce71c7f301a6d4307b6b20898377b5e9bda",
          "message": "chore(main): release 0.51.6 (#2061)",
          "timestamp": "2026-11-25T13:07:42-05:00",
          "tree_id": "48ed4c269c6a9abf3f4bf695128c324b91f8b223",
          "url": "https://github.com/Wikid82/Charon/commit/0f450ce71c7f301a6d4307b6b20898377b5e9bda"
        },
        "date": 1795630314583,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 98451,
            "unit": "ns/op\t   47171 B/op\t     670 allocs/op",
            "extra": "12381 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 98451,
            "unit": "ns/op",
            "extra": "12381 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47171,
            "unit": "B/op",
            "extra": "12381 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 670,
            "unit": "allocs/op",
            "extra": "12381 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 82424,
            "unit": "ns/op\t   42606 B/op\t     519 allocs/op",
            "extra": "16170 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 82424,
            "unit": "ns/op",
            "extra": "16170 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 42606,
            "unit": "B/op",
            "extra": "16170 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_NoSettings (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 519,
            "unit": "allocs/op",
            "extra": "16170 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 167703,
            "unit": "ns/op\t   86592 B/op\t    1261 allocs/op",
            "extra": "7220 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 167703,
            "unit": "ns/op",
            "extra": "7220 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86592,
            "unit": "B/op",
            "extra": "7220 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1261,
            "unit": "allocs/op",
            "extra": "7220 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 79454,
            "unit": "ns/op\t   47618 B/op\t     789 allocs/op",
            "extra": "15057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 79454,
            "unit": "ns/op",
            "extra": "15057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47618,
            "unit": "B/op",
            "extra": "15057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListRuleSets (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 789,
            "unit": "allocs/op",
            "extra": "15057 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 36537,
            "unit": "ns/op\t   20660 B/op\t     238 allocs/op",
            "extra": "31675 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 36537,
            "unit": "ns/op",
            "extra": "31675 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 20660,
            "unit": "B/op",
            "extra": "31675 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetConfig (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 238,
            "unit": "allocs/op",
            "extra": "31675 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 45476,
            "unit": "ns/op\t   46842 B/op\t     569 allocs/op",
            "extra": "26271 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 45476,
            "unit": "ns/op",
            "extra": "26271 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 46842,
            "unit": "B/op",
            "extra": "26271 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_GetStatus_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 569,
            "unit": "allocs/op",
            "extra": "26271 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 121578,
            "unit": "ns/op\t   86120 B/op\t    1264 allocs/op",
            "extra": "8694 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 121578,
            "unit": "ns/op",
            "extra": "8694 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 86120,
            "unit": "B/op",
            "extra": "8694 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ListDecisions_Parallel (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 1264,
            "unit": "allocs/op",
            "extra": "8694 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers)",
            "value": 98673,
            "unit": "ns/op\t   47170 B/op\t     675 allocs/op",
            "extra": "12556 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - ns/op",
            "value": 98673,
            "unit": "ns/op",
            "extra": "12556 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - B/op",
            "value": 47170,
            "unit": "B/op",
            "extra": "12556 times\n4 procs"
          },
          {
            "name": "BenchmarkSecurityHandler_ManySettingsLookups (github.com/Wikid82/charon/backend/internal/api/handlers) - allocs/op",
            "value": 675,
            "unit": "allocs/op",
            "extra": "12556 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 596.4,
            "unit": "ns/op\t    1696 B/op\t       6 allocs/op",
            "extra": "1937457 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 596.4,
            "unit": "ns/op",
            "extra": "1937457 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1696,
            "unit": "B/op",
            "extra": "1937457 times\n4 procs"
          },
          {
            "name": "BenchmarkEncrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 6,
            "unit": "allocs/op",
            "extra": "1937457 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto)",
            "value": 474.4,
            "unit": "ns/op\t    1472 B/op\t       4 allocs/op",
            "extra": "2516343 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - ns/op",
            "value": 474.4,
            "unit": "ns/op",
            "extra": "2516343 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - B/op",
            "value": 1472,
            "unit": "B/op",
            "extra": "2516343 times\n4 procs"
          },
          {
            "name": "BenchmarkDecrypt (github.com/Wikid82/charon/backend/internal/crypto) - allocs/op",
            "value": 4,
            "unit": "allocs/op",
            "extra": "2516343 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 4.738,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "248127873 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 4.738,
            "unit": "ns/op",
            "extra": "248127873 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "248127873 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "248127873 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network)",
            "value": 184400,
            "unit": "ns/op\t   17873 B/op\t     126 allocs/op",
            "extra": "5935 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 184400,
            "unit": "ns/op",
            "extra": "5935 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 17873,
            "unit": "B/op",
            "extra": "5935 times\n4 procs"
          },
          {
            "name": "BenchmarkNewInternalServiceHTTPClient_Request (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 126,
            "unit": "allocs/op",
            "extra": "5935 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network)",
            "value": 47.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26250620 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 47.21,
            "unit": "ns/op",
            "extra": "26250620 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "26250620 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Private (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "26250620 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network)",
            "value": 77.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16152196 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 77.74,
            "unit": "ns/op",
            "extra": "16152196 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "16152196 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv4Public (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "16152196 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network)",
            "value": 77.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15956564 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 77.72,
            "unit": "ns/op",
            "extra": "15956564 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "15956564 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP_IPv6 (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "15956564 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network)",
            "value": 176.4,
            "unit": "ns/op\t     856 B/op\t       8 allocs/op",
            "extra": "6867132 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - ns/op",
            "value": 176.4,
            "unit": "ns/op",
            "extra": "6867132 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - B/op",
            "value": 856,
            "unit": "B/op",
            "extra": "6867132 times\n4 procs"
          },
          {
            "name": "BenchmarkNewSafeHTTPClient (github.com/Wikid82/charon/backend/internal/network) - allocs/op",
            "value": 8,
            "unit": "allocs/op",
            "extra": "6867132 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services)",
            "value": 78.06,
            "unit": "ns/op\t       8 B/op\t       1 allocs/op",
            "extra": "15284689 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 78.06,
            "unit": "ns/op",
            "extra": "15284689 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8,
            "unit": "B/op",
            "extra": "15284689 times\n4 procs"
          },
          {
            "name": "BenchmarkFormatDuration (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 1,
            "unit": "allocs/op",
            "extra": "15284689 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services)",
            "value": 264.2,
            "unit": "ns/op\t     204 B/op\t       9 allocs/op",
            "extra": "4579864 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 264.2,
            "unit": "ns/op",
            "extra": "4579864 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 204,
            "unit": "B/op",
            "extra": "4579864 times\n4 procs"
          },
          {
            "name": "BenchmarkExtractPort (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 9,
            "unit": "allocs/op",
            "extra": "4579864 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services)",
            "value": 16307,
            "unit": "ns/op\t    8234 B/op\t     165 allocs/op",
            "extra": "71486 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 16307,
            "unit": "ns/op",
            "extra": "71486 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 8234,
            "unit": "B/op",
            "extra": "71486 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_IsConfigured (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 165,
            "unit": "allocs/op",
            "extra": "71486 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services)",
            "value": 345.6,
            "unit": "ns/op\t     488 B/op\t       5 allocs/op",
            "extra": "3846997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - ns/op",
            "value": 345.6,
            "unit": "ns/op",
            "extra": "3846997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - B/op",
            "value": 488,
            "unit": "B/op",
            "extra": "3846997 times\n4 procs"
          },
          {
            "name": "BenchmarkMailService_BuildEmail (github.com/Wikid82/charon/backend/internal/services) - allocs/op",
            "value": 5,
            "unit": "allocs/op",
            "extra": "3846997 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil)",
            "value": 0.000146,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n4 procs"
          },
          {
            "name": "BenchmarkMemoryDSN_AcceptsB (github.com/Wikid82/charon/backend/internal/testutil) - ns/op",
            "value": 0.000146,
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
            "value": 4.898,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "245841166 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.898,
            "unit": "ns/op",
            "extra": "245841166 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "245841166 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/equal (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "245841166 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 4.973,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "249221511 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 4.973,
            "unit": "ns/op",
            "extra": "249221511 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "249221511 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_first_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "249221511 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util)",
            "value": 5.04,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223024246 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - ns/op",
            "value": 5.04,
            "unit": "ns/op",
            "extra": "223024246 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "223024246 times\n4 procs"
          },
          {
            "name": "BenchmarkConstantTimeCompare/different_last_char (github.com/Wikid82/charon/backend/internal/util) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "223024246 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils)",
            "value": 51.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22542628 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - ns/op",
            "value": 51.53,
            "unit": "ns/op",
            "extra": "22542628 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "22542628 times\n4 procs"
          },
          {
            "name": "BenchmarkIsPrivateIP (github.com/Wikid82/charon/backend/internal/utils) - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "22542628 times\n4 procs"
          }
        ]
      }
    ]
  }
}