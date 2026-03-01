#!/bin/bash

# This script updates Go module dependencies for the project.

cd /projects/Charon/backend || exit

echo "Updating Go module dependencies..."

go get -u ./...
go mod tidy
go mod verify
go vet ./...
go list -m -u all
go build ./...

echo "Go module dependencies updated successfully."
