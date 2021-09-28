#!/bin/bash

set -ex

kubectl exec myrabbitmq-server-0 -c rabbitmq -- rabbitmqctl authenticate_user guest guest
