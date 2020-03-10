---
title: CRD spec configurability
authors:
  - "@ChunyiLyu"
reviewers:
  -
creation-date: 2020-03-10
last-updated: yyyy-mm-dd
status: provisional|implementable|implemented|deferred|rejected|withdrawn|replaced
see-also:
replaces:
superseded-by:
---

# CRD spec configurability

<!-- END Remove before PR -->

## Table of Contents

A table of contents is helpful for quickly jumping to sections of a proposal and for highlighting
any additional information provided beyond the standard proposal template.
[Tools for generating](https://github.com/ekalinin/github-markdown-toc) a table of contents from markdown are available.

- [CRD spec configurability](#crd-spec-configurability)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
    - [Alternative 1](#alternative-1)
    - [Alternative 2](#alternative-2)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan [optional]](#test-plan-optional)
    - [Graduation Criteria [optional]](#graduation-criteria-optional)
    - [Version Skew Strategy [optional]](#version-skew-strategy-optional)
  - [Implementation History](#implementation-history)

## Glossary

Refer to the [Cluster API Book Glossary](https://cluster-api.sigs.k8s.io/reference/glossary.html).

If this proposal adds new terms, or defines some, make the changes to the book's glossary when in PR stage.

## Summary

This KEP describes a new structure for the `RabbitmqCluster` spec to achieve better flexibilty and configurability for the `RabbitmqCluster`. The proposed solution suggests using the StatefulSet, Service, and the ConfigMap kubernetes template directly in our spec. Users will be able to configure any field that of the StatefulSet and ingress Service template through editting the `RabbitmqCluster` spec.

## Motivation

We would like to make our `RabbitmqCluster` CRD as flexible as possible. This is aligned with our team roadmap discussions. We decided that we are going to provide different layers of abstructions to create RabbitMQ, and the `RabbitmqCluster` CRD, which is at the 'bottom' of the abstruction layer, should offer little opinion on how users should configure and create `RabbitmqCluster` instances. The `RabbitmqCluster` CRD should enable users to customize it to whichever configurations that adhere to users' requirements.

### Goals

- To make the `RabbitmqCluster` CRD as flexible as possible
- To leverage kubernetes native object templates

### Non-Goals/Future Work

- To provide structures or guidelines so users know exactly what to configure for each spec properties (under the assumption that users who choose to configure the StatefulSet and ingress Service templates knows what they are configuring and why)
- To prioritize simplicity over flexibility. We should offer better usability and opinions on how to deploy `RabbitmqCluster` at the different abstruction layer (this is related to the CRD work that Stev has outlined. Related work can be found here: [Stev's miro board](https://miro.com/app/board/o9J_kvlRPnc=/)).

## Proposal

The proposed new `RabbitmqCluster` spec uses StatefulSet, Service, and the ConfigMap kubernetes template directly. Our operator currently creates 9 kuberentes child recources directly for each `RabbitmqCluster`: ingress Service, headless Service, StatefulSet, ConfigMap, erlang cookie secret, admin secret, rbac role, role binding, service account. Out of all these resources, we are currently allowing users to partially configure the StatefulSet, the ingress Service, and the pods that StatefulSet creates (look at manifest example below for all configurations we allow through the `RabbitmqCluster` spec). I think it makes sense to focus the two child recources (ingress Service and StatefulSet) for adding configurability, as there is no abvious use case we can think of now that involves configuring any of the other child resources. Our ConfigMap current tracks the list of rabbitmq plug-ins that's enabled by default and the rabbitmq conf file. The current process for our users to update that the plug-ins list is to modify the ConfigMap object directly, and recreate the StatefulSet object. With this new structure of the `RabbitmqCluster` spec, user will be able to modify the ConfigMap directly in the `RabbitmqCluster` spec.

At the moment, if someone customizes every field in the `RabbitmqCluster` spec, their manifest can look like:

```
apiVersion: rabbitmq.pivotal.io/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmqcluster-sample
  namespace: pivotal-rabbitmq-system
spec:
  replicas: 1
  service:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
  image: rabbitmq:3.8.2
  imagePullSecret: my-secret
  tolerations:
  - key: "dedicated"
    operator: "Equal"
    value: "rabbitmq"
    effect: "NoSchedule"
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/hostname
            operator: In
            values:
            - node-1
  resources:
    requests:
      cpu: 1000m
      memory: 2Gi
    limits:
      cpu: 1000m
      memory: 2Gi
  persistence:
    storageClassName: fast
    storage: 20Gi
```

With the proposed `RabbitmqCluster` spec structure, users can configure and create the same `RabbitmqCluster` as the above example by providing the below manifest. The manifest below also configures list of plug-ins and rabbitmq conf file through configMapTemplate:

```
apiVersion: rabbitmq.pivotal.io/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmqcluster-sample
  namespace: pivotal-rabbitmq-system
spec:
  statefulSetTemplate:
    spec:
      replicas: 1
        spec:
          tolerations:
          - key: "dedicated"
            operator: "Equal"
            value: "rabbitmq"
            effect: "NoSchedule"
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                - matchExpressions:
                  - key: kubernetes.io/hostname
                    operator: In
                    values:
                    - node-1
          containers:
          - image: rabbitmq:3.8.2
            name: rabbitmq
            resources:
              limits:
                cpu: 1000m
                memory: 2Gi
              requests:
                cpu: 1000m
                memory: 2Gi
      volumeClaimTemplates:
        spec:
          storageClassName: fast
          resources:
            requests:
              storage: 20Gi
  ingressServiceTemplate:
    metadata:
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
    spec:
      type: LoadBalancer
  configMapTemplate:
    data:
      enabled_plugins: '[rabbitmq_management,rabbitmq_peer_discovery_k8s,rabbitmq_federation,rabbitmq_federation_management,rabbitmq_shovel,rabbitmq_shovel_management,rabbitmq_prometheus].'
      rabbitmq.conf: |-
        cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
        cluster_formation.k8s.host = kubernetes.default.svc.cluster.local
        cluster_formation.k8s.address_type = hostname
        cluster_formation.node_cleanup.interval = 30
        cluster_formation.node_cleanup.only_log_warning = true
        cluster_partition_handling = pause_minority
        queue_master_locator = min-masters
```
TODO: what about use spec directly instead of the template? What's the benefits of letting people modifying the name and namespace fields?

Since our controller fills in any default values that we impose on `RabbitmqCluster` at the time of creation. After the above `RabbitmqCluster` was created, the updated manifest will look like:

```
apiVersion: rabbitmq.pivotal.io/v1beta1
kind: RabbitmqCluster
metadata:
  name: rabbitmqcluster-sample
  namespace: pivotal-rabbitmq-system
spec:
  statefulSetTemplate:
    metadata:
      labels:
        app.kubernetes.io/component: rabbitmq
        app.kubernetes.io/name: rabbitmqcluster-sample
        app.kubernetes.io/part-of: pivotal-rabbitmq
      name: rabbitmqcluster-sample-rabbitmq-server
      namespace: pivotal-rabbitmq-system
    spec:
      podManagementPolicy: OrderedReady
      replicas: 1
      revisionHistoryLimit: 10
      selector:
        matchLabels:
          app.kubernetes.io/name: rabbitmqcluster-sample
      serviceName: rabbitmqcluster-sample-rabbitmq-headless
      template:
        metadata:
          creationTimestamp: null
          labels:
            app.kubernetes.io/component: rabbitmq
            app.kubernetes.io/name: rabbitmqcluster-sample
            app.kubernetes.io/part-of: pivotal-rabbitmq
        spec:
          automountServiceAccountToken: true
          tolerations:
          - key: "dedicated"
            operator: "Equal"
            value: "rabbitmq"
            effect: "NoSchedule"
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                - matchExpressions:
                  - key: kubernetes.io/hostname
                    operator: In
                    values:
                    - node-1
          containers:
          - env:
            - name: RABBITMQ_ENABLED_PLUGINS_FILE
              value: /opt/server-conf/enabled_plugins
            - name: RABBITMQ_DEFAULT_PASS_FILE
              value: /opt/rabbitmq-secret/password
            - name: RABBITMQ_DEFAULT_USER_FILE
              value: /opt/rabbitmq-secret/username
            - name: RABBITMQ_MNESIA_BASE
              value: /var/lib/rabbitmq/db
            - name: MY_POD_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.name
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.namespace
            - name: K8S_SERVICE_NAME
              value: rabbitmqcluster-sample-rabbitmq-headless
            - name: RABBITMQ_USE_LONGNAME
              value: "true"
            - name: RABBITMQ_NODENAME
              value: rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local
            - name: K8S_HOSTNAME_SUFFIX
              value: .$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local
            image: rabbitmq:3.8.2
            imagePullPolicy: IfNotPresent
            lifecycle:
              preStop:
                exec:
                  command:
                  - /bin/bash
                  - -c
                  - while true; do rabbitmq-queues check_if_node_is_quorum_critical 2>&1;
                    if [ $(echo $?) -eq 69 ]; then sleep 2; continue; fi; rabbitmq-queues
                    check_if_node_is_mirror_sync_critical 2>&1; if [ $(echo $?) -eq 69
                    ]; then sleep 2; continue; fi; break; done
            name: rabbitmq
            ports:
            - containerPort: 4369
              name: epmd
              protocol: TCP
            - containerPort: 5672
              name: amqp
              protocol: TCP
            - containerPort: 15672
              name: http
              protocol: TCP
            - containerPort: 15692
              name: prometheus
              protocol: TCP
            readinessProbe:
              exec:
                command:
                - /bin/sh
                - -c
                - rabbitmq-diagnostics check_port_connectivity
              failureThreshold: 3
              initialDelaySeconds: 10
              periodSeconds: 30
              successThreshold: 1
              timeoutSeconds: 5
            resources:
              limits:
                cpu: "2"
                memory: 2Gi
              requests:
                cpu: "1"
                memory: 2Gi
            terminationMessagePath: /dev/termination-log
            terminationMessagePolicy: File
            volumeMounts:
            - mountPath: /opt/server-conf/
              name: server-conf
            - mountPath: /opt/rabbitmq-secret/
              name: rabbitmq-admin
            - mountPath: /var/lib/rabbitmq/db/
              name: persistence
            - mountPath: /etc/rabbitmq/
              name: rabbitmq-etc
            - mountPath: /var/lib/rabbitmq/
              name: rabbitmq-erlang-cookie
          dnsPolicy: ClusterFirst
          initContainers:
          - command:
            - sh
            - -c
            - cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >>
              /etc/rabbitmq/rabbitmq.conf ; cp /tmp/erlang-cookie-secret/.erlang.cookie
              /var/lib/rabbitmq/.erlang.cookie && chown 999:999 /var/lib/rabbitmq/.erlang.cookie
              && chmod 600 /var/lib/rabbitmq/.erlang.cookie
            image: rabbitmq:3.8.2
            imagePullPolicy: IfNotPresent
            name: copy-config
            resources:
              limits:
                cpu: 1000m
                memory: 2Gi
              requests:
                cpu: 1000m
                memory: 2Gi
            terminationMessagePath: /dev/termination-log
            terminationMessagePolicy: File
            volumeMounts:
            - mountPath: /tmp/rabbitmq/
              name: server-conf
            - mountPath: /etc/rabbitmq/
              name: rabbitmq-etc
            - mountPath: /var/lib/rabbitmq/
              name: rabbitmq-erlang-cookie
            - mountPath: /tmp/erlang-cookie-secret/
              name: erlang-cookie-secret
          restartPolicy: Always
          schedulerName: default-scheduler
          securityContext:
            fsGroup: 999
            runAsGroup: 999
            runAsUser: 999
          serviceAccount: rabbitmqcluster-sample-rabbitmq-server
          serviceAccountName: rabbitmqcluster-sample-rabbitmq-server
          terminationGracePeriodSeconds: 604800
          volumes:
          - name: rabbitmq-admin
            secret:
              defaultMode: 420
              items:
              - key: username
                path: username
              - key: password
                path: password
              secretName: rabbitmqcluster-sample-rabbitmq-admin
          - configMap:
              defaultMode: 420
              name: rabbitmqcluster-sample-rabbitmq-server-conf
            name: server-conf
          - emptyDir: {}
            name: rabbitmq-etc
          - emptyDir: {}
            name: rabbitmq-erlang-cookie
          - name: erlang-cookie-secret
            secret:
              defaultMode: 420
              secretName: rabbitmqcluster-sample-rabbitmq-erlang-cookie
      updateStrategy:
        rollingUpdate:
          partition: 0
        type: RollingUpdate
      volumeClaimTemplates:
      - metadata:
          creationTimestamp: null
          labels:
            app.kubernetes.io/component: rabbitmq
            app.kubernetes.io/name: rabbitmqcluster-sample
            app.kubernetes.io/part-of: pivotal-rabbitmq
          name: persistence
          namespace: pivotal-rabbitmq-system
          ownerReferences:
          - apiVersion: rabbitmq.pivotal.io/v1beta1
            blockOwnerDeletion: true
            controller: true
            kind: RabbitmqCluster
            name: rabbitmqcluster-sample
            uid: c090c807-22c7-4ae6-97d8-dc4252b568c7
        spec:
          accessModes:
          - ReadWriteOnce
          storageClassName: "fast"
          resources:
            requests:
              storage: 20Gi
          volumeMode: Filesystem
    ingressServiceTemplate:
      metadata:
        labels:
          app.kubernetes.io/component: rabbitmq
          app.kubernetes.io/name: rabbitmqcluster-sample
          app.kubernetes.io/part-of: pivotal-rabbitmq
          service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
        name: rabbitmqcluster-sample-rabbitmq-ingress
        namespace: pivotal-rabbitmq-system
      spec:
        clusterIP: 10.51.248.13
        externalTrafficPolicy: Cluster
        ports:
        - name: prometheus
          nodePort: 30040
          port: 15692
          protocol: TCP
          targetPort: 15692
        - name: amqp
          nodePort: 31965
          port: 5672
          protocol: TCP
          targetPort: 5672
        - name: http
          nodePort: 30090
          port: 15672
          protocol: TCP
          targetPort: 15672
        selector:
          app.kubernetes.io/name: rabbitmqcluster-sample
        sessionAffinity: None
        type: LoadBalancer
  configMapTemplate:
      metadata:
        labels:
          app.kubernetes.io/component: rabbitmq
          app.kubernetes.io/name: rabbitmqcluster-sample
          app.kubernetes.io/part-of: pivotal-rabbitmq
        name: rabbitmqcluster-sample-rabbitmq-server-conf
        namespace: pivotal-rabbitmq-system
      data:
        enabled_plugins: '[rabbitmq_management,rabbitmq_peer_discovery_k8s,rabbitmq_federation,rabbitmq_federation_management,rabbitmq_shovel,rabbitmq_shovel_management,rabbitmq_prometheus].'
        rabbitmq.conf: |-
          cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
          cluster_formation.k8s.host = kubernetes.default.svc.cluster.local
          cluster_formation.k8s.address_type = hostname
          cluster_formation.node_cleanup.interval = 30
          cluster_formation.node_cleanup.only_log_warning = true
          cluster_partition_handling = pause_minority
          queue_master_locator = min-masters
```

The updated manifest contains almost the complete manifest for the StatefulSet, ingress Service, and ConfigMap (it does not have any fields like the status, kind, or apiversion, which require updates from the apiserver, not the controller).

### Update

TODO

### User Stories

#### Story 1

Users will be able to configure rabbitmq plugins and rabbitmq conf file at the time of creation. Users will have a documented way to update enabled rabbitmq plugins and rabbitmq conf file which does not require them from recreating the StatefulSet.

#### Story 2

Users can inject any container that they would like to the StatefulSet.

### Implementation Details/Notes/Constraints

- Merging defaults and users provided values could be difficult

### Risks and Mitigations

- Increase of complexity of the RabbitmqCluster.spec. The feature asks to include the entire StatefulSet, ConfigMap Service definitions as part of our RabbitmqCluster spec. Because the spec reflects all its default values, with this feature, RabbitmqCluster spec will include almost the entire manifest of these child objects and become much harder to read for users.
- Decreased abstructuctions. With this feature, we will show much configuration details than what users may care about. This can make RabbitmqCluster harder to use.

## Alternatives

### Alternative 1

Keep currently CRD spec structure

### Alternative 2

Define our own StatefulSet and Service template to have better control over what people can configure.


## Upgrade Strategy

n/a

## Additional Details

### Test Plan [optional]

Unit, integration, system(side effect)

### Graduation Criteria [optional]

n/a

### Version Skew Strategy [optional]
 
n/a

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY
