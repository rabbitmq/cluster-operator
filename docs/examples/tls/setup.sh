
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.1/cert-manager.yaml
kubectl wait pod -n cert-manager --for=condition=Ready=True -l app=cert-manager
kubectl wait pod -n cert-manager --for=condition=Ready=True -l app=webhook

kubectl apply -f certificate.yaml
