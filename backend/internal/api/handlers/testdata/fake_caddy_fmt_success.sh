#!/bin/sh
# Fake caddy that handles fmt (formats single-line to multi-line) and adapt

if [ "$1" = "version" ]; then
  echo "v2.0.0"
  exit 0
fi

if [ "$1" = "fmt" ] && [ "$2" = "--overwrite" ]; then
  # Read the file content
  CONTENT=$(cat "$3")
  # Check if it looks like a single-line Caddyfile
  if echo "$CONTENT" | grep -q '{ .* }$'; then
    # Simulate formatting: write formatted content back to the file
    DOMAIN=$(echo "$CONTENT" | sed 's/ {.*//')
    cat > "$3" << EOF
${DOMAIN} {
    reverse_proxy localhost:8080
}
EOF
  fi
  exit 0
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
