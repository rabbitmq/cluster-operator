#!/bin/bash

kubectl get pod sidecar-server-0 -o jsonpath="{.spec.containers[*].image}" | grep busybox
kubectl get pod sidecar-server-0 | grep 2/2

