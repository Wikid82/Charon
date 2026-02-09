#!/bin/sh
# Fake caddy that fails fmt but succeeds on adapt (for testing normalization fallback)

if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi

if [ "$1" = "fmt" ]; then
  # Simulate fmt failure
  echo "Error: fmt failed" >&2
  exit 1
fi

if [ "$1" = "adapt" ]; then
  DOMAIN="example.com"
  if [ "$2" = "--config" ]; then
    # Read domain from first line of file
    DOMAIN=$(head -1 "$3" | awk '{print $1}')
  fi
  echo "{\"apps\":{\"http\":{\"servers\":{\"srv0\":{\"routes\":[{\"match\":[{\"host\":[\"$DOMAIN\"]}],\"handle\":[{\"handler\":\"reverse_proxy\",\"upstreams\":[{\"dial\":\"localhost:8080\"}]}]}]}}}}}"
  exit 0
fi

exit 1
