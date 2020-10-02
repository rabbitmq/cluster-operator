#!/bin/bash

tmp=$(mktemp)
yq -y 'delpaths([.. | paths(scalars)|select(contains(["spec","versions",0,"schema","openAPIV3Schema","properties","spec","properties","override","description"]))])' < config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml > "$tmp"
mv "$tmp" config/crd/bases/rabbitmq.com_rabbitmqclusters.yaml
