#!/bin/bash

kubectl exec community-plugins-server-0 -c rabbitmq -- rabbitmq-plugins is_enabled rabbitmq_message_timestamp

