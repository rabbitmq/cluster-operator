Steps to test an SSL connection to the server

1. [Deploy cert-manager](https://cert-manager.io/docs/installation/kubernetes/) in the cluster
1. create issuer `k apply -f issuer.yaml`
1. request a certificate `k apply -f cetificate.yaml`. I've used the ingress service name as the DNS name as that's what `kubefwd` provides (see below)
1. deploy the operator
1. deploy a the rabbitmqcluster-sample (with `spec.tls: true`)
1. download the cacert with `k get secrets self-signed-cert -o jsonpath="{.data['ca\.crt']}" | base64 -D > ca.crt`
1. Add the rabbit admin creds to `send.py`
1. port forward the rabbit service to localhost `sudo kubefwd svc -n pivotal-rabbitmq-system`
1. run `./send.py` and verify the message is on the queue
1. you can also check the handshake with `openssl s_client -connect rabbitmqcluster-sample-rabbitmq-ingress:5671 -CAfile ca.crt`

