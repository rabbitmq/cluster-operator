#!/bin/bash

set -euo pipefail

wikiDir=$1

generateVersionedEntry() {
  echo "* [$1](https://github.com/rabbitmq/cluster-operator/wiki/API_Reference_$1)"
}

generateLatestEntry() {
  echo "* [Latest](https://github.com/rabbitmq/cluster-operator/wiki/API_Reference)"
}

lead='^<!--- BEGIN API REFERENCE LIST -->$'
tail='^<!--- END API REFERENCE LIST -->$'

unorderedlistfile="$(mktemp)"
orderedlistfile="$(mktemp)"

apiVersions="$(find "$wikiDir/API_Reference_*" -printf "%f\n" | sed -r 's/API_Reference_(v[0-9]+.[0-9]+.[0-9]+).asciidoc/\1/' | sort -Vr )"
for version in $apiVersions
do
  generateVersionedEntry "$version" >> "$unorderedlistfile"
done

# Latest API version is a special case, that is always top of the list
generateLatestEntry > "$orderedlistfile"
cat "$unorderedlistfile" >> "$orderedlistfile"

sed -e "/$lead/,/$tail/{ /$lead/{p; r $orderedlistfile
}; /$tail/p; d }"  "$wikiDir/Wiki_Sidebar.md"
