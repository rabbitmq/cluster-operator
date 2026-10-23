#!/bin/bash
set -euo pipefail

YJ="${YJ:-yj}"

tmp=$(mktemp)
"$YJ" -yj < config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml | jq 'delpaths([.. | paths(scalars)|select(contains(["spec","versions",0,"schema","openAPIV3Schema","properties","spec","properties","override","description"]))])' | "$YJ" -jy > "$tmp"
mv "$tmp" config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml
