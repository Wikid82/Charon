#!/usr/bin/env bash
set -u

# Find python executable
if command -v python3 &>/dev/null; then
    PYTHON_CMD="python3"
elif command -v python &>/dev/null; then
    PYTHON_CMD="python"
else
    echo "Error: neither python3 nor python found." >&2
    exit 1
fi

# Run compileall and capture output
# We capture both stdout and stderr
OUTPUT=$($PYTHON_CMD -m compileall -q . 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
    echo "Python compile check FAILED (Exit Code: $EXIT_CODE)" >&2
    echo "Output:" >&2
    echo "$OUTPUT" >&2
    exit $EXIT_CODE
fi

exit 0
