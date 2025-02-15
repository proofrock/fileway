#!/bin/bash

# This script is used to build a binary, passing to the linker
# those values that are needed to build reproducibly.

# Copy inside the src/ dir
# Env vars must be already set: VERSION, SOURCE_DATE_EPOCH
CGO_ENABLED=0 

go build -trimpath -a -o fileway \
  -tags="netgo osusergo" \
  -ldflags="-w -buildid=\"$VERSION\" -X \"main.version=$VERSION\" -X \"main.buildTime=$SOURCE_DATE_EPOCH\" -extldflags \"-static\""
