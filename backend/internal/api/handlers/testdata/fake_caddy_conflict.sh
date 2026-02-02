#!/bin/sh
if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi
if [ "$1" = "fmt" ]; then
  exit 0
fi
if [ "$1" = "adapt" ]; then
  # Return a host that conflicts with existing (conflict.example.com)
  echo "{\"apps\":{\"http\":{\"servers\":{\"srv0\":{\"routes\":[{\"match\":[{\"host\":[\"conflict.example.com\"]}],\"handle\":[{\"handler\":\"reverse_proxy\",\"upstreams\":[{\"dial\":\"192.168.1.100:9000\"}]}]}]}}}}}"
  exit 0
fi
exit 1
