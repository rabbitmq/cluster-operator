#!/usr/bin/env bash

if [ -n "$1" ]; then
    echo "BUNDLE_VERSION=$1"
    exit 0
fi

if [ "$GITHUB_REF_TYPE" != "tag" ]; then
  echo "BUNDLE_VERSION=0.0.0"
  exit 0
fi

printf "BUNDLE_VERSION=%s\n" "${GITHUB_REF_NAME:1}"
