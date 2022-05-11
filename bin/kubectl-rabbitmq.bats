#!/usr/bin/env bats

eventually() {
 assertion="$1"
 timeout_in_seconds="$2"

  for _ in $(seq 0 "$timeout_in_seconds")
  do
    if eval "$assertion" ; then
      return 0
    fi
    sleep 1
  done

  echo assertion timed out: "$assertion"
  return 1
}

@test "version outputs version" {
  run kubectl rabbitmq version

  if ! command -v kubectl-krew &> /dev/null
  then
    [ "$status" -eq 1 ]
    [ "${lines[0]}" = "version cannot be determined because plugin was not installed via krew" ]
  else
    [ "$status" -eq 0 ]
    version_regex='kubectl-rabbitmq v([0-9]+)\.([0-9]+)\.([0-9]+)$'
    [[ "${lines[0]}" =~ $version_regex ]]
  fi
}

@test "install-cluster-operator with too many args fails" {
  run kubectl rabbitmq install-cluster-operator too-many-args

  [ "$status" -eq 1 ]
  [ "${lines[0]}" = "USAGE:" ]
}

@test "create creates RabbitMQ cluster" {
  kubectl rabbitmq create bats-default \
    --unlimited # otherwise scheduling the pod fails on kind in GitHub actions because of insufficient CPU

  eventually '[[ $(kubectl get rabbitmqcluster bats-default -o json | jq -r '"'"'.status.conditions | .[] | select(.type=="AllReplicasReady").status'"'"') == "True" ]]' 600
}

@test "create with invalid flag does not create RabbitMQ cluster" {
  run kubectl rabbitmq create bats-invalid \
    --replicas 3 \
    --invalid "flag"

  [ "$status" -eq 1 ]
  [[ "${lines[0]}" == "Option '--invalid' not recongnised" ]]
}

@test "create with flags creates RabbitMQ cluster configured accordingly" {
  replicas=3
  image="rabbitmq:3.8.8"
  service="NodePort"
  storage_class="my-storage-class"

  kubectl rabbitmq create bats-configured \
    --replicas "$replicas" \
    --image "$image" \
    --service "$service" \
    --storage-class "$storage_class"

  sleep 10 # let the RabbitMQ controller create the K8s objects

  sts_spec=$(kubectl get statefulset bats-configured-server -o jsonpath='{.spec}')
  [[ $(jq -r '.replicas' <<< "$sts_spec") -eq "$replicas" ]]
  [[ $(jq -r '.template.spec.containers | .[0].image' <<< "$sts_spec") == "$image" ]]

  [[ $(kubectl get service bats-configured -o jsonpath='{.spec.type}') == "$service" ]]
  [[ $(kubectl get pvc persistence-bats-configured-server-0 -o jsonpath='{.spec.storageClassName}') == "$storage_class" ]]
}

@test "list lists all RabbitMQ clusters in the current namespace" {
  run kubectl rabbitmq list

  [ "$status" -eq 0 ]
  [[ "${lines[0]}" =~ ^NAME ]]
  [[ "${lines[1]}" =~ ^bats-configured ]]
  [[ "${lines[2]}" =~ ^bats-default ]]
}

@test "list supports -n NAMESPACE flag" {
  CLUSTER_NAMESPACE="$(kubectl config view --minify --output 'jsonpath={..namespace}')"
  # if namespace is not set in kube config, take the default namespace
  CLUSTER_NAMESPACE="${CLUSTER_NAMESPACE:=default}"

  kubectl create namespace kubectl-rabbitmq-tests
  kubectl config set-context --current --namespace=kubectl-rabbitmq-tests

  run kubectl rabbitmq -n "${CLUSTER_NAMESPACE}" list

  [ "$status" -eq 0 ]
  [[ "${lines[0]}" =~ ^NAME ]]
  [[ "${lines[1]}" =~ ^bats-configured ]]
  [[ "${lines[2]}" =~ ^bats-default ]]

  kubectl config set-context --current --namespace="${CLUSTER_NAMESPACE}"
  kubectl delete namespace kubectl-rabbitmq-tests
}

@test "list supports -A flag" {
  CLUSTER_NAMESPACE="$(kubectl config view --minify --output 'jsonpath={..namespace}')"

  kubectl create namespace kubectl-rabbitmq-tests
  kubectl config set-context --current --namespace=kubectl-rabbitmq-tests

  run kubectl rabbitmq -A list

  [ "$status" -eq 0 ]
  [[ "$output" == *"NAMESPACE"* ]]
  [[ "$output" == *"bats-configured"* ]]
  [[ "$output" == *"bats-default"* ]]

  kubectl config set-context --current --namespace="${CLUSTER_NAMESPACE}"
  kubectl delete namespace kubectl-rabbitmq-tests
}

@test "get gets child resources" {
  run kubectl rabbitmq get bats-default

  [ "$status" -eq 0 ]
  [[ "$output" == *"statefulset.apps/bats-default-server"* ]]
  [[ "$output" == *"pod/bats-default-server-0"* ]]
  [[ "$output" == *"service/bats-default-nodes"* ]]
  [[ "$output" == *"service/bats-default"* ]]
  [[ "$output" == *"configmap/bats-default-server-conf"* ]]
  [[ "$output" == *"configmap/bats-default-plugins-conf"* ]]
  [[ "$output" == *"secret/bats-default-default-user"* ]]
  [[ "$output" == *"secret/bats-default-erlang-cookie"* ]]
}

@test "pause-and-resume-reconciliation" {
  kubectl rabbitmq pause-reconciliation bats-default
  kubectl get rabbitmqclusters.rabbitmq.com bats-default --show-labels | grep rabbitmq.com/pauseReconciliation

  run kubectl rabbitmq list-pause-reconciliation-instances

  [ "$status" -eq 0 ]
  [[ "$output" == *"bats-default"* ]]

  kubectl rabbitmq resume-reconciliation bats-default
  kubectl get rabbitmqclusters.rabbitmq.com bats-default --show-labels | grep none
}

@test "secrets prints secrets of default-user" {
  run kubectl rabbitmq secrets bats-default

  [ "$status" -eq 0 ]
  # 24 bytes base64 encoded makes 32 characters
  username_regex='^username: .{32}$'
  password_regex='^password: .{32}$'
  [[ "${lines[0]}" =~ $username_regex ]]
  [[ "${lines[1]}" =~ $password_regex ]]
}

@test "enable-all-feature-flags enables all feature flags" {
  kubectl rabbitmq enable-all-feature-flags bats-default

  states=$(kubectl exec bats-default-server-0 -- rabbitmqctl list_feature_flags --silent state --formatter=json)
  [[ $(jq 'map(select(.state=="disabled")) | length' <<< "$states") -eq 0 ]]
}

@test "perf-test runs perf-test" {
  kubectl rabbitmq perf-test bats-default --rate 1

  eventually "kubectl exec bats-default-server-0 -- rabbitmqctl list_connections client_properties | grep perf-test " 600

  kubectl delete job -l "app=perf-test"
}

@test "debug sets log level to debug" {
  kubectl rabbitmq debug bats-default

  eventually "kubectl logs -c rabbitmq bats-default-server-0 | grep ' \[debug\] '" 30
}

@test "delete deletes RabbitMQ cluster" {
  kubectl rabbitmq delete bats-configured bats-default

  [[ $(kubectl get rabbitmqclusters -o jsonpath='{.items}' | jq length) -eq 0 ]]
}

@test "help prints help" {
  run kubectl rabbitmq help

  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "USAGE:" ]
}

@test "--help prints help" {
  run kubectl rabbitmq --help

  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "USAGE:" ]
}

@test "-h prints help" {
  run kubectl rabbitmq -h

  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "USAGE:" ]
}
