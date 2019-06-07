#!/usr/bin/env bash

set -e

fly -t rmq4k8s set-pipeline -p operator -c pipeline.yml -l vars.yml
