# Mutual TLS Peer Verification (Mutual TLS Authentication, mTLS) for Inter-node Traffic Example

This example is an addition to two other TLS-related examples:

 * [basic TLS example](../tls)
 * [mutual peer verification ("mTLS") for client connections](../mtls)

It is recommended to get familiar at least with the basics of [TLS setup in RabbitMQ](https://www.rabbitmq.com/ssl.html)
before going over this example, in particular with [how TLS peer verification works](https://www.rabbitmq.com/ssl.html#peer-verification).
While those guides focus on client connections to RabbitMQ, the general verification process is identical
when performed by two RabbitMQ nodes that attempt to establish a connection.


## Enabling Peer Verification for Inter-node Connections

When a clustered RabbitMQ node connects to its cluster peer, both
can [verify each other's certificate chain](https://www.rabbitmq.com/ssl.html#peer-verification) for trust.

When such verification is performed on both ends, the practice is sometimes
referred to "mutual TLS authentication" or simply "mTLS". This example
focuses on enabling mutual peer verifications for inter-node connections (as opposed to [client communication](../mtls)).

This example makes RabbitMQ cluster nodes [communicate via TLS-enabled cluster links](https://www.rabbitmq.com/clustering-ssl.html)
for additional security, fully automated by the Cluster Operator via the
[cert-manager CSI driver](https://cert-manager.io/docs/usage/csi-driver/): setting `spec.tls.interNode` is all that is
needed, with no [`envConfig`](https://www.rabbitmq.com/kubernetes/operator/using-operator.html#env-config) or
[`override`](https://www.rabbitmq.com/kubernetes/operator/using-operator.html#override) required. Each pod gets its
own certificate, issued at mount time with an identity (SANs) matching that specific pod, so this also survives
`spec.replicas` changes with no CR edit.

If you cannot run the CSI driver in your cluster, the manual approach — writing `spec.rabbitmq.envConfig` and
`spec.override.statefulSet` by hand to mount an out-of-band certificate — still works unchanged; it is just no
longer necessary.

### Prerequisites

This example requires both [cert-manager](https://cert-manager.io/docs/installation/) and the separately-installed
[cert-manager-csi-driver](https://cert-manager.io/docs/usage/csi-driver/) chart to be present in the cluster. The
operator does not detect or install either of them:

```shell
helm repo add jetstack https://charts.jetstack.io --force-update
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true
helm install cert-manager-csi-driver jetstack/cert-manager-csi-driver --namespace cert-manager
```

The most important parts of this example are:

- `rabbitmq.yaml` - `RabbitmqCluster` definition with `spec.tls.interNode` enabled
- `issuer.yaml` - a self-signed root Issuer, a CA Certificate, and the CA Issuer that `rabbitmq.yaml`'s
  `spec.tls.interNode.issuerRef` points at

**`spec.tls.interNode.issuerRef` must point at a CA-type issuer.** The operator-generated Erlang distribution TLS
terms reference `cacertfile` unconditionally, and the CSI driver only writes `ca.crt` when the issuer it talks to is
backed by a CA. Pointing `issuerRef` at a `SelfSigned` or ACME issuer instead leaves `ca.crt` missing or empty, and
nodes fail to start with an opaque Erlang error. See `issuer.yaml` for a CA issuer that works.

**`issuer.yaml`'s `Issuer`s are namespace-scoped**, and the cert-manager CSI driver resolves `issuerRef` in the
*pod's* namespace, not wherever you happen to apply from. `issuer.yaml` pins its resources to `examples`, so
`rabbitmq.yaml` must land in that same namespace, or the pods will sit `Pending` on a mount timeout with no
indication that a namespace mismatch is the cause:

```shell
kubectl create namespace examples --dry-run=client -o yaml | kubectl apply -f -
kubectl apply --namespace=examples -f issuer.yaml -f rabbitmq.yaml
```

If you deploy `RabbitmqCluster`s across multiple namespaces, use a `ClusterIssuer` instead (set
`spec.tls.interNode.issuerRef.kind: ClusterIssuer`), which is cluster-scoped and needs no such alignment.

To validate that RabbitMQ nodes connect over TLS, run the following checks:

```shell
# the CSI driver issued a cert per pod
kubectl get certificaterequests -n examples

# check that the distribution port has TLS enabled (this command should return `Verification: OK`)
kubectl exec -it -n examples mtls-inter-node-server-0 -- bash -c 'openssl s_client -connect ${HOSTNAME}${K8S_HOSTNAME_SUFFIX}:25672 -state -cert /etc/rabbitmq-inter-node-tls/tls.crt -key /etc/rabbitmq-inter-node-tls/tls.key -CAfile /etc/rabbitmq-inter-node-tls/ca.crt 2>&1 | grep Verification'

# check that distribution uses TLS (this command should return `{ok,[["inet_tls"]]}`)
kubectl exec -it -n examples mtls-inter-node-server-0 -- rabbitmqctl eval 'init:get_argument(proto_dist).'
```


## Known Limitations

- **The issuer must be a CA-type issuer** (see above): a `SelfSigned` or ACME `issuerRef` leaves `ca.crt`
  missing or empty and nodes fail to start.
- **Certificate renewal is not hot-reloaded into existing connections.** cert-manager's CSI driver writes
  renewed certificate files in place, but Erlang distribution only reads them when a connection is set up;
  already-established inter-node connections keep using the certificate that was live when they were
  established, and are not re-keyed until they are re-established (e.g. on pod restart).
- **The Vault `default-user-updater` sidecar does not get the inter-node cert volume.** If it needs to reach a
  node running `-proto_dist inet_tls` (e.g. via `rabbitmqctl`), this has not been exercised with this feature.
- **No operator-side detection of whether `cert-manager-csi-driver` is installed.** If it is absent, pods stay
  `Pending` on a mount error (see Troubleshooting below) with no explicit signal from the operator.
- **The default `fs-group` (`999`) assumes the container's primary gid resolves from the RabbitMQ image's own
  `/etc/passwd` entry**, which in turn requires the operator's explicit `RunAsUser: 999` to reach the pod
  unchanged. If `spec.override.statefulSet` replaces that explicit uid with one that has no matching
  `/etc/passwd` entry, the RabbitMQ process's primary gid falls back to `0`, the certificate files (group
  `999`) become unreadable, and a workaround may be needed depending on which override caused it — see
  Troubleshooting below. (A platform admission controller that *mutates the generated Pod* — e.g. a Kyverno
  mutate policy — could cause the same failure, but this is distinct from a restricted SCC, which validates an
  explicitly-set `RunAsUser` rather than rewriting it; see Troubleshooting.)

## Troubleshooting

RabbitMQ has a guide that explains a methodology for [troubleshooting TLS](https://www.rabbitmq.com/troubleshooting-ssl.html) using
OpenSSL command line tools. This methodology helps narrow down connectivity issues quicker.

In the context of Kubernetes, OpenSSL CLI tools can be run on RabbitMQ nodes using `kubectl exec`, e.g.:

``` shell
kubectl exec -it -n examples mtls-inter-node-server-0 -- openssl s_client -connect mtls-inter-node-server-0.mtls-inter-node-nodes.examples.svc.cluster.local:25672 -cert /etc/rabbitmq-inter-node-tls/tls.crt -key /etc/rabbitmq-inter-node-tls/tls.key -CAfile /etc/rabbitmq-inter-node-tls/ca.crt </dev/null
```

A client certificate must be supplied: the distribution listener sets `fail_if_no_peer_cert, true` and
`verify, verify_peer` unconditionally (see `internal/resource/configmap.go`), so a bare `openssl s_client`
invocation with no `-cert`/`-key`/`-CAfile` will fail the handshake rather than complete it.

### Pod stuck `Pending` with a mount error

Check that the `cert-manager-csi-driver` chart (not just cert-manager itself) is installed; the operator does
not detect whether it is present.

### Pod stuck `ContainerCreating` with a "gid" or "fs-group" mount error

If the pod is stuck `ContainerCreating` (not `Pending`) with a `NodePublishVolume`/mount error mentioning a "gid"
or "fs-group", the CSI driver's `csi.cert-manager.io/fs-group` attribute is missing or not a positive integer.
The operator always sets this attribute to `"999"` on the generated volume, so this should only come up via a
`spec.override.statefulSet` that overwrote it with an empty string or a non-positive value (strategic merge can
overwrite a `volumeAttributes` key but cannot delete one, so the attribute itself cannot be "removed" by an
override): `cert-manager-csi-driver` only honours a `fs-group` attribute that is present and `> 0` — if it is
missing or non-positive, it falls back to the CSI `VolumeMountGroup` field, which kubelet fills from the pod's
`securityContext.fsGroup` (which the operator's pod template fixes to `0`, for OpenShift-style arbitrary-uid
compatibility — see `internal/resource/statefulset.go`), and the driver rejects any value `<= 0` outright,
failing the mount rather than leaving the files unreadable.

### RabbitMQ cannot read its own certificate files (permission denied)

If the pod starts but RabbitMQ cannot read its own certificate files under `/etc/rabbitmq-inter-node-tls/`
(permission denied): the CSI driver writes the certificate files as `<uid>:<fs-group>`, mode `0440` — owner
unchanged (whichever uid the driver's node plugin runs as), group set to the `fs-group` attribute's value. The
operator sets that attribute to `"999"`, matching the primary gid of uid `999` in the official RabbitMQ image's
`/etc/passwd` (`rabbitmq:x:999:999:...`). Because the pod sets `RunAsUser: 999` without a `RunAsGroup`, the
container runtime resolves the RabbitMQ process's primary gid from that same passwd entry — so on a standard
Kubernetes distribution the process already has gid `999` and can read the files, with no dependency on the
pod-wide `securityContext.fsGroup` (which stays fixed at `0`, for OpenShift-style arbitrary-uid compatibility,
and is otherwise unrelated to this volume).

### Platforms that override `RunAsUser` (e.g. OpenShift SCCs)

This breaks down if something replaces the operator's explicit `RunAsUser: 999` with a uid that has no
matching `/etc/passwd` entry. This is *not* the out-of-the-box behavior of a restricted SCC on OpenShift: SCCs
validate an explicitly-set `RunAsUser` against their allocated range rather than silently rewriting it, so the
operator's hardcoded `999` either matches the namespace's allocated range or the pod is rejected outright at
admission — it does not silently start running as some other uid. There are two ways
`spec.override.statefulSet` can reach the failure state, and only one of them has a fix that stops there:

- **Overriding `runAsUser` to a different, explicit uid** (e.g. a namespace-allocated uid your SCC requires)
  merges cleanly with the operator's other pod `securityContext` fields, so `supplementalGroups` can be added
  in the same override to grant gid `999` back:

  ```yaml
  spec:
    override:
      statefulSet:
        spec:
          template:
            spec:
              securityContext:
                runAsUser: 1000670000
                supplementalGroups:
                  - 999
              containers:
                - name: rabbitmq
  ```

  This works **without** touching the pod-wide `fsGroup` (which stays fixed at `0`, for the reason above, and
  which some SCCs' `fsGroup` strategy — `MustRunAs`, restricted to the namespace's allocated range — would
  reject as an out-of-range explicit value if set to `999` instead) and without re-chowning the persistence
  volume the way a pod-wide `fsGroup` override would. OpenShift's bootstrapped `restricted-v2` SCC sets
  `fsGroup: {type: MustRunAs}` but `supplementalGroups: {type: RunAsAny}` (verified directly against
  `openshift/cluster-kube-apiserver-operator`'s
  [`scc-restricted-v2.yaml`](https://raw.githubusercontent.com/openshift/cluster-kube-apiserver-operator/master/bindata/bootkube/scc-manifests/0000_20_kube-apiserver-operator_00_scc-restricted-v2.yaml)),
  so an explicit `supplementalGroups` value is not subject to the same namespace-range validation. The
  `csi.cert-manager.io/fs-group` volume attribute does not need overriding here either: the operator already
  sets it to `"999"` by default (see above).

- **Clearing `securityContext` to an empty struct (`{}`)**, to let the SCC assign the uid entirely, is a dead
  end for this fix. The operator only drops its own pod-level security defaults (`RunAsUser`, `FSGroup`, etc.)
  when the override's `securityContext` is exactly that empty struct; adding `supplementalGroups` to the
  override makes it non-empty, so those defaults — including `RunAsUser: 999` — get merged back in alongside
  it, which is what you were trying to clear, and is likely outside a `MustRunAsRange` SCC's allocated range,
  making the pod un-admittable. If your platform requires clearing `RunAsUser` entirely rather than replacing
  it with a specific allocated uid, this `fs-group`/gid-`999` approach does not apply.

Only apply either override if you have confirmed the default does not work on your platform.

See [issue #1319](https://github.com/rabbitmq/cluster-operator/issues/1319) for the client-facing half of
directly-mounted TLS certificates, which this example does not cover.
