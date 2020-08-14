#!/bin/bash

set -e

TARGET_YAML=$1
SCRIPT_DIR="$(dirname $0)"

if [[ -z $TARGET_YAML ]]
then
  echo "No target YAML provided"
  exit 1
fi

OUTPUT=$(mktemp)

cat "$SCRIPT_DIR/NOTICE.yaml.txt" "$TARGET_YAML" > "$OUTPUT"
mv "$OUTPUT" "$TARGET_YAML"
