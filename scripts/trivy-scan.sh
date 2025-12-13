#!/bin/bash
set -e

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
