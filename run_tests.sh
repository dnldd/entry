#!/bin/sh

set -ex

go version

# set output format
OUT_FORMAT="github-actions"

# run tests
env GORACE="halt_on_error=1" go test -timeout 10s -race -cover ./...


echo "------------------------------------------"
echo "Tests completed successfully!"