#!/bin/bash

set -ex
kubectl exec -t mtls-inter-node-server-0 -c rabbitmq -- rabbitmq-diagnostics command_line_arguments > kubectl.out
grep '{proto_dist,\["inet_tls"\]}' kubectl.out

