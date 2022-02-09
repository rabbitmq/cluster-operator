#!/bin/bash

set -ex
kubectl exec -t multiple-disks-server-0 -c rabbitmq -- rabbitmqctl environment > rabbitmq-environment.out

grep 'data_dir,"/var/lib/rabbitmq/quorum-segments"' rabbitmq-environment.out
grep 'wal_data_dir,"/var/lib/rabbitmq/quorum-wal"' rabbitmq-environment.out

