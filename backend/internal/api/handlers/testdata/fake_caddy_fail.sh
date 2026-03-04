#!/bin/sh
if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi
exit 1
