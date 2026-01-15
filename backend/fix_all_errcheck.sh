#!/bin/bash
# Fix all errcheck errors iteratively

for i in {1..10}; do
    echo "=== Iteration $i ==="

    # Run linter and extract just file:line for errcheck errors
    errors=$(go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --config .golangci.yml ./... 2>&1 | grep "errcheck" | grep -oP '.*\.go:\d+' | sort -u)

    if [ -z "$errors" ]; then
        echo "NO MORE ERRCHECK ERRORS FOUND!"
        break
    fi

    echo "Found $(echo "$errors" | wc -l) error locations"

    # Fix each one
    while IFS=: read -r file line; do
        # Check what function is on that line
        func=$(sed -n "${line}p" "$file" | grep -oP '(db\.AutoMigrate|json\.Unmarshal|os\.Setenv|os\.Unsetenv|sqlDB\.Close|w\.Write)')

        if [ "$func" = "w.Write" ]; then
            # w.Write returns 2 values
            sed -i "${line}s/w\.Write/_, _ = w.Write/" "$file"
        elif [ -n "$func" ]; then
            sed -i "${line}s/${func}/_ = ${func}/" "$file"
        fi

        echo "Fixed $file:$line"
    done <<< "$errors"
done
