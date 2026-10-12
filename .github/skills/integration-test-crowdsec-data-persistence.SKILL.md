---
# agentskills.io specification v1.0
name: "integration-test-crowdsec-data-persistence"
version: "1.0.0"
description: "Verify CrowdSec hub data files persist across container recreation"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "crowdsec"
  - "persistence"
  - "hub"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "docker"
    version: ">=24.0"
    optional: false
environment_variables:
  - name: "CHARON_TEST_IMAGE"
    description: "Image to test"
    default: "charon:local"
    required: false
  - name: "INIT_TIMEOUT_SECONDS"
    description: "Maximum wait for CrowdSec hub initialization per container start"
    default: "300"
    required: false
parameters: []
outputs:
  - name: "test_results"
    type: "stdout"
    description: "PASS/FAIL results per check"
metadata:
  category: "integration-test"
  subcategory: "crowdsec"
  execution_time: "long"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test CrowdSec Data Persistence

## Overview

CrowdSec hub items are stored on the `/app/data` volume, but their data files (blocklists,
GeoLite2 databases) live in CrowdSec's `data_dir`. This test starts throwaway containers on a
temporary named volume and proves that:

1. A fresh volume really installs the hub items and populates `data_dir` on the volume.
2. Recreating the container keeps the data files and does NOT run `cscli hub upgrade`.
3. Recreating after the data files were lost runs `cscli hub upgrade` once and restores them.
4. A legacy volume whose `config.yaml` still points at `/var/lib/crowdsec/data` is migrated.

## Usage

```bash
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-data-persistence
```

Requires network access to the CrowdSec hub. Uses no published ports and removes its container
and volume on exit.

**Source**: `scripts/crowdsec_data_persistence_test.sh`
