#!/bin/bash

set -ex

name=$(kubectl get pod -l app.kubernetes.io/name=priority-class \
  -ojsonpath='{.items[0].spec.priorityClassName}' 2> /dev/null)

[[ "$name" == "high-priority" ]] || exit 1
