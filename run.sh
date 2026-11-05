#!/bin/bash
# Build and run Tailwatch

set -e

echo "Building Tailwatch..."
go build ./cmd/tailwatch

echo "Running Tailwatch with example config..."
./tailwatch -verbose
