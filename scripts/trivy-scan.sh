#!/bin/bash
set -e

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh security-scan-trivy
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh security-scan-trivy" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

# Build the local image first to ensure it's up to date
echo "Building charon:local..."
docker build -t charon:local .

# Run Trivy scan
echo "Running Trivy scan on charon:local..."
docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $HOME/.cache/trivy:/root/.cache/trivy \
    -v $(pwd)/.trivy_logs:/logs \
    aquasec/trivy:latest image \
    --severity CRITICAL,HIGH \
    --output /logs/trivy-report.txt \
    charon:local

echo "Scan complete. Report saved to .trivy_logs/trivy-report.txt"
cat .trivy_logs/trivy-report.txt
