#!/bin/bash

kubectl exec community-plugins-server-0 -- rabbitmq-plugins is_enabled rabbitmq_message_timestamp

