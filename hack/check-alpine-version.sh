#!/usr/bin/env bash

# Script to check for updates to the pinned Alpine base image
# Usage: ./hack/check-alpine-version.sh <dockerfile-path> <updates-file>
#
# This script reads the Alpine image reference (alpine:<tag>@sha256:<digest>)
# from the Dockerfile and checks Docker Hub for a newer release, or for a new
# digest of the same tag. Results are appended to the updates file as:
#   ALPINE_IMAGE|<current tag@digest>|<new tag@digest>|<url>

set -euo pipefail

DOCKERFILE="${1:-Dockerfile}"
UPDATES_FILE="${2:-updates.txt}"

echo "Checking Alpine base image for updates..."

current=$(grep -oE 'alpine:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}' "$DOCKERFILE" | head -n1 | sed 's/^alpine://' || true)
if [ -z "$current" ]; then
  echo "  Error: Could not find a pinned alpine:<tag>@sha256:<digest> reference in $DOCKERFILE"
  exit 1
fi

echo "Checking ALPINE_IMAGE (current: $current)..."

# Docker Hub returns the multi-arch index digest for each tag
tags_json=$(curl -fsSL "https://hub.docker.com/v2/repositories/library/alpine/tags?page_size=100&ordering=last_updated" 2>/dev/null || echo "")
if [ -z "$tags_json" ]; then
  echo "  Warning: Could not fetch tags from Docker Hub"
  exit 0
fi

latest_tag=$(echo "$tags_json" | jq -r '.results[].name' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n1)
latest_digest=$(echo "$tags_json" | jq -r --arg tag "$latest_tag" '.results[] | select(.name == $tag) | .digest')

if [ -z "$latest_tag" ] || [[ ! "$latest_digest" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "  Warning: Could not determine latest Alpine release"
  exit 0
fi

latest="${latest_tag}@${latest_digest}"

if [ "$current" != "$latest" ]; then
  echo "  ✓ Update available: $current → $latest"
  echo "ALPINE_IMAGE|$current|$latest|https://hub.docker.com/_/alpine" >> "$UPDATES_FILE"
else
  echo "  Already up-to-date"
fi

echo "Alpine image check complete"
