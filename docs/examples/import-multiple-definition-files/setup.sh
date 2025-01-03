#!/bin/bash

kubectl create configmap definitions --from-file=definitions.json
kubectl create secret generic users --from-file=users.json
