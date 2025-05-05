#!/bin/sh

set -ex

go version

# set output format
OUT_FORMAT="github-actions"

# run tests
env GORACE="halt_on_error=1" go test -count 1 -timeout 20s -race -cover ./...


echo "------------------------------------------"
echo "Tests completed successfully!"