#!/usr/bin/env bash

set -e

prev=$(gh release list --exclude-drafts --exclude-pre-releases --limit 2 --json tagName --jq '.[1].tagName')

printf "PREVIOUS_VERSION=%s\n" "${prev}"
