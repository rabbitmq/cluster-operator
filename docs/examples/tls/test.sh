
set -e
kubectl exec -it tls-server-0 -c rabbitmq -- openssl s_client -connect tls-nodes.examples.svc.cluster.local:5671 </dev/null

kubectl exec -it tls-server-0 -c rabbitmq -- openssl s_client -connect tls-nodes.examples.svc.cluster.local:15671 </dev/null

