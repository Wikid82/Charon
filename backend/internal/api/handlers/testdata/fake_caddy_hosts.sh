#!/bin/sh
if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi
if [ "$1" = "adapt" ]; then
  # Read the domain from the input Caddyfile (stdin or --config file)
  DOMAIN="example.com"
  if [ "$2" = "--config" ]; then
    DOMAIN=$(cat "$3" | head -1 | tr -d '\n')
  fi
  echo "{\"apps\":{\"http\":{\"servers\":{\"srv0\":{\"routes\":[{\"match\":[{\"host\":[\"$DOMAIN\"]}],\"handle\":[{\"handler\":\"reverse_proxy\",\"upstreams\":[{\"dial\":\"localhost:8080\"}]}]}]}}}}}"
  exit 0
fi
exit 1
