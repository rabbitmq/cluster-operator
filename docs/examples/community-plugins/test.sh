#!/bin/bash

kubectl exec community-plugins-server-0 -c rabbitmq -- rabbitmq-plugins is_enabled rabbitmq_delayed_message_exchange
