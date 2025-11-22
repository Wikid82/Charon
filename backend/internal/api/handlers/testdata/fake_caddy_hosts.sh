#!/bin/sh
if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi
if [ "$1" = "adapt" ]; then
  echo '{"apps":{"http":{"servers":{"srv0":{"routes":[{"match":[{"host":["example.com"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"localhost:8080"}]}]}]}}}}}'
  exit 0
fi
exit 1
