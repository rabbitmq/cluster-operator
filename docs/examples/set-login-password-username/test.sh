
set -ex

kubectl exec -it myrabbitmq-server-0 -c rabbitmq -- rabbitmqctl authenticate_user guest guest
