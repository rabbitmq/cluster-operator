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

@test "install-cluster-operator with too many args fails" {
  run kubectl rabbitmq install-cluster-operator too-many-args

  [ "$status" -eq 1 ]
  [ "${lines[0]}" = "USAGE:" ]
}

@test "install-cluster-operator installs cluster operator" {
  kubectl rabbitmq install-cluster-operator

  eventually 'kubectl -n rabbitmq-system get deployment rabbitmq-cluster-operator | grep 1/1' 600
}

@test "create creates RabbitMQ cluster" {
  kubectl rabbitmq create bats-default

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

  sts_spec=$(kubectl get statefulset bats-configured-rabbitmq-server -o jsonpath='{.spec}')
  [[ $(jq -r '.replicas' <<< "$sts_spec") -eq "$replicas" ]]
  [[ $(jq -r '.template.spec.containers | .[0].image' <<< "$sts_spec") == "$image" ]]

  [[ $(kubectl get service bats-configured-rabbitmq-client -o jsonpath='{.spec.type}') == "$service" ]]
  [[ $(kubectl get pvc persistence-bats-configured-rabbitmq-server-0 -o jsonpath='{.spec.storageClassName}') == "$storage_class" ]]
}

@test "list lists all RabbitMQ clusters" {
  run kubectl rabbitmq list

  [ "$status" -eq 0 ]
  [[ "${lines[0]}" =~ ^NAME ]]
  [[ "${lines[1]}" =~ ^bats-configured ]]
  [[ "${lines[2]}" =~ ^bats-default ]]
}

@test "get gets child resources" {
  run kubectl rabbitmq get bats-default

  [ "$status" -eq 0 ]
  [[ "$output" == *"statefulset.apps/bats-default-rabbitmq-server"* ]]
  [[ "$output" == *"pod/bats-default-rabbitmq-server-0"* ]]
  [[ "$output" == *"service/bats-default-rabbitmq-headless"* ]]
  [[ "$output" == *"service/bats-default-rabbitmq-client"* ]]
  [[ "$output" == *"configmap/bats-default-rabbitmq-server-conf"* ]]
  [[ "$output" == *"configmap/bats-default-rabbitmq-plugins-conf"* ]]
  [[ "$output" == *"secret/bats-default-rabbitmq-default-user"* ]]
  [[ "$output" == *"secret/bats-default-rabbitmq-erlang-cookie"* ]]
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

  states=$(kubectl exec bats-default-rabbitmq-server-0 -- rabbitmqctl list_feature_flags --silent state --formatter=json)
  [[ $(jq 'map(select(.state=="disabled")) | length' <<< "$states") -eq 0 ]]
}

@test "perf-test runs perf-test" {
  kubectl rabbitmq perf-test bats-default --rate 1

  eventually "kubectl exec bats-default-rabbitmq-server-0 -- rabbitmqctl list_connections client_properties | grep perf-test " 600

  kubectl delete pod -l "app=perf-test,run=perf-test"
  kubectl delete svc -l "app=perf-test,run=perf-test"
}

@test "debug sets log level to debug" {
  kubectl rabbitmq debug bats-default

  # '[debug] <pid> Lager installed handler' is logged even without enabling debug logging
  eventually "kubectl logs bats-default-rabbitmq-server-0 | grep -v ' \[debug\] .* Lager installed handler ' | grep ' \[debug\] '" 30
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
