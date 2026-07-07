#!/usr/bin/env bash

# Script to check for updates to Go-installable tools
# Usage: ./hack/check-go-tool-versions.sh <makefile-path> <updates-file>
#
# This script reads tool versions from the Makefile and checks for updates
# using `go list -m -versions`. Results are written to the updates file.

set -euo pipefail

MAKEFILE="${1:-Makefile}"
UPDATES_FILE="${2:-updates.txt}"

echo "Checking Go-installable tools for updates..."

# Function to check Go module version
check_go_version() {
  local var_name=$1
  local module=$2
  local current_version=$3
  local repo_url=$4
  
  echo "Checking $var_name (current: $current_version)..."
  
  # Get all versions for the module
  versions=$(go list -m -versions -f '{{range .Versions}}{{.}} {{end}}' "$module" 2>/dev/null | tr ' ' '\n' | sed '/^$/d' || echo "")
  
  if [ -z "$versions" ]; then
    echo "  Warning: Could not fetch versions for $module"
    return
  fi
  
  # Filter out pre-release versions and find the latest stable version
  latest=$(echo "$versions" | grep -v -E '(alpha|beta|rc)' | tail -1)
  
  if [ -z "$latest" ]; then
    echo "  Warning: No stable version found for $module"
    return
  fi
  
  # Compare versions (remove 'v' prefix for comparison if present)
  current_clean="${current_version#v}"
  latest_clean="${latest#v}"
  
  if [ "$current_clean" != "$latest_clean" ]; then
    echo "  ✓ Update available: $current_version → $latest"
    echo "$var_name|$current_version|$latest|$repo_url" >> "$UPDATES_FILE"
  else
    echo "  Already up-to-date"
  fi
}

# Read current versions from Makefile
KUSTOMIZE_VERSION=$(grep '^KUSTOMIZE_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')
CONTROLLER_TOOLS_VERSION=$(grep '^CONTROLLER_TOOLS_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')
YJ_VERSION=$(grep '^YJ_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')
GOVULNCHECK_VERSION=$(grep '^GOVULNCHECK_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')

# Check each Go tool
check_go_version "KUSTOMIZE_VERSION" "sigs.k8s.io/kustomize/kustomize/v5" "$KUSTOMIZE_VERSION" "https://github.com/kubernetes-sigs/kustomize"
check_go_version "CONTROLLER_TOOLS_VERSION" "sigs.k8s.io/controller-tools" "$CONTROLLER_TOOLS_VERSION" "https://github.com/kubernetes-sigs/controller-tools"
check_go_version "YJ_VERSION" "github.com/sclevine/yj/v5" "$YJ_VERSION" "https://github.com/sclevine/yj"
check_go_version "GOVULNCHECK_VERSION" "golang.org/x/vuln" "$GOVULNCHECK_VERSION" "https://github.com/golang/vuln"

echo "Go tools check complete"
