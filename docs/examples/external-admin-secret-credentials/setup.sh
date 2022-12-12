#!/bin/bash
set -eo pipefail

echo "Creating external secret"

kubectl create -f my-secret.yml