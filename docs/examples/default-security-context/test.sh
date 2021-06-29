#!/bin/bash
set -x

uid="$(kubectl exec default-security-context-server-0 -- \
  id -u 2> /dev/null)"
groups="$(kubectl exec default-security-context-server-0 -- \
  id -G 2> /dev/null)"
## kubectl std. error is redirectd to null because the error output of jsonpath
## is not very helpful to troubleshoot

[[ "$uid" == "0" && "$groups" == "0" ]] || exit 1

