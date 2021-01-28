#!/bin/bash
set -x
port12345=$(kubectl get pod -l app.kubernetes.io/name=rabbit \
  -ojsonpath='{.items[0].spec.containers[0].ports[?(@.containerPort==12345)].name}' 2> /dev/null)
## kubectl std. error is redirectd to null because the error output of jsonpath
## is not very helpful to troubleshoot

[[ "$port12345" == "additional-port" ]] || exit 1

