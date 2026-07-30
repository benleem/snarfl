#!/bin/bash
# run from base project directory
set -e

echo "Running tests..."
go test -v ./...

echo "Tests passed! Building binary..."
go build -o ./bin/snarfl ./cmd/cli

echo "Build successful."
