---
title: CRD Spec Refactor
authors:
  - "@ChunyiLyu"
reviewers:
  -
creation-date: 2020-03-10
last-updated: yyyy-mm-dd
status: provisional
see-also:
replaces:
superseded-by:
---

# CRD Spec Refactor

## Table of Contents

- [CRD Spec Refactor](#crd-spec-refactor)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [Update](#update)
    - [User Stories](#user-stories)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Risks and Mitigation](#risks-and-mitigation)
  - [Alternatives](#alternatives)
    - [Alternative 1](#alternative-1)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
  - [Implementation History](#implementation-history)

## Glossary

Refer to the [Cluster API Book Glossary](https://cluster-api.sigs.k8s.io/reference/glossary.html).

If this proposal adds new terms, or defines some, make the changes to the book's glossary when in PR stage.

## Summary

This KEP defines a new structure for the `RabbitmqCluster` spec to achieve increased flexibility. The proposed solution suggests using the StatefulSet, and Service kubernetes template directly in the `RabbitmqCluster` spec. Users will be able to configure any field of the StatefulSet and ingress Service through editing the spec directly.

## Motivation

Today, users are currently limited to a set of kubernetes properties that we select and define in the CRD spec to configure their RabbitMQ deployments. For each property we have added to the CRD spec, we usually thought of a specific use case and added them one by one. This approach has its downside, as we are currently not getting enough concrete user feedbacks to justify or prioritize each use case. And, the effort of adding these properties is repetitive and does not scale. 

In our team roadmap we decided to coup the lack of user feedbacks by offering [different layers of abstractions](https://miro.com/app/board/o9J_kvlRPnc=/) to deploy RabbitMQ. The current `RabbitmqCluster` CRD, which is at the bottom of the abstraction layer, should provide little opinion on how users can configure their `RabbitmqCluster` instances. Instead, the CRD needs to allow users to customize it to whichever configurations that adhere to users' own requirements. For users with little specific requirements and just want to use an existing RabbitMQ configuration that their cluster operator has defined, they can use a different CRD from `RabbitmqCluster`.

## Goals

* Increase flexibility at configuring how to deploy RabbitMQ by enabling users to configure any field in the StatefulSet and ingress Service spec
* Reduce effort to add new properties by introducing an extensible CRD spec structure

## Non-Goals/Future Work

* Increase flexibility at configuring RabbitMQ by enabling users to configure rabbitmq conf file and list of plugins. This feature will be addressed separately by [github issue on rabbitmq conf](https://github.com/pivotal/rabbitmq-for-kubernetes/issues/57), [github issue on rabbitmq plugins](https://github.com/pivotal/rabbitmq-for-kubernetes/issues/58).
* To provide detailed guidelines on how to configure each spec property. We are assuming that users who choose to configure the StatefulSet and ingress Service spec knows their specific use cases, and how to use kubernetes.
* To prioritize simplicity over flexibility. We should offer better usability and opinions on how to deploy `RabbitmqCluster` at the different abstraction layer, such as `Claims` and `Plans`, as defined in [Stev's design proposal](https://miro.com/app/board/o9J_kvlRPnc=/)

## Proposal

The proposed new `RabbitmqCluster` spec uses StatefulSet and Service kubernetes spec directly. Our operator creates 9 kubernetes child recourses directly for each `RabbitmqCluster`: ingress Service, headless Service, StatefulSet, ConfigMap, erlang cookie secret, admin secret, rbac role, role binding, service account. Among these resources, we allow users to partially configure the StatefulSet, the ingress Service, and the pods that StatefulSet creates (look at manifest example below for all configurations we allow through the `RabbitmqCluster` spec). The proposal focuses on two child recourses: ingress Service and StatefulSet to increase configurability, since there is no obvious use case for now that involves configuring any of the other child resources.

At the moment, if someone customizes every field in the `RabbitmqCluster` spec, their manifest can look like:

```yaml
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

With the proposed `RabbitmqCluster` spec structure, users can configure and create the same `RabbitmqCluster` as the above example by providing the below manifest. The manifest below also configures a list of plugins and rabbitmq conf file through configMapTemplate:

```yaml
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
          template:
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
```

Since our controller fills in all default values that we impose on `RabbitmqCluster` at the time of creation. After the above `RabbitmqCluster` was created, the updated manifest will look like:

```yaml
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
      replicas: 1
      selector:
        matchLabels:
          app.kubernetes.io/name: rabbitmqcluster-sample
      serviceName: rabbitmqcluster-sample-rabbitmq-headless
      template:
        metadata:
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
          - name: rabbitmq
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
            env:
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
          labels:
            app.kubernetes.io/component: rabbitmq
            app.kubernetes.io/name: rabbitmqcluster-sample
            app.kubernetes.io/part-of: pivotal-rabbitmq
          name: persistence
          namespace: pivotal-rabbitmq-system
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
        ports:
        - name: prometheus
          nodePort: 30040
          port: 15692
          protocol: TCP
        - name: amqp
          nodePort: 31965
          port: 5672
          protocol: TCP
        - name: http
          nodePort: 30090
          port: 15672
          protocol: TCP
        selector:
          app.kubernetes.io/name: rabbitmqcluster-sample
        type: LoadBalancer
```

The updated manifest contains almost the complete manifest for the StatefulSet and ingress Service. It does not have any fields like the status, kind, or apiversion, which require updates from the apiserver, not the controller.

### Update

Any update to the StatefulSet and ingress Service template will be reconciled by our controller. This means that certain updates will trigger reconciliation errors. In that case, reconciliation errors should be surfaced through status.conditions as usual. Some updates on the pod template of the StatefulSet template will trigger a StatefulSet restart. Because we are not offering opinions about how people should configure and deploy their RabbitMQ, a StatefulSet restart should not be a concern to us.

### User Stories

#### Story 1

Users can inject any container that they would like to the StatefulSet. For example, they can have side cars to aggregate logs.

#### Story 2

Users can set environment variables in the rabbitmq container. They can set environment variables that configures the rabbitmq server.

### Implementation Details/Notes/Constraints

There are several properties that are lists where we currently have defaults fo them. Lets take a look of the list of containers `spec.statefulSetTemplate.spec.template.Containers` as an example, where our controller defines one container: `rabbitmq`. There are roughly two different use cases: one is to add a new container, and the other is to modify a certain property in the already defined `rabbitmq` container.

Case 1) As a user, I would like to add a sidecar container.

In this case, users can do so by specifying only the sidecar container they want to add to the list of containers:

```yaml
kind: RabbitmqCluster
spec:
  statefulSetTemplate:
    spec:
      template:
        spec:
          containers:
          - name: rabbit-side-cat
            image: rabbit-side-cat
            ...
```
Our controller should append the `rabbit-side-car` container users provided.`RabbitmqCluster` will have both the `rabbitmq` container and the `rabbit-side-cat` container in the same pod.

Case 2) As a user, I would like to define resource requests and limits for the `rabbitmq` container

In this case, users will need to define their resource requests and limits inside the `rabbitmq` container:

```yaml
kind: RabbitmqCluster
spec:
  statefulSetTemplate:
    spec:
      template:
        spec:
          containers:
          - name: rabbitmq
            resources:
              limits:
                cpu: "10"
                memory: 10Gi
              requests:
                cpu: "1"
                memory: 10Gi
```

User does not need to provide other configurations for container `rabbitmq`. Our controller will set defaults for properties that user hasn't provided value for.

We should apply the same logic to merge other lists values, e.g.:

* spec.statefulSetTemplate.spec.template.volumes
* spec.statefulSetTemplate.spec.template.spec.template.Containers["rabbitmq"].env
* spec.statefulSetTemplate.spec.template.initContainers
* spec.ingressServiceTemplate.spec.ports

This way, we ensure that users have the flexibility to configure properties and that they do not need to provide values that they do not need to modify or care about.

### Risks and Mitigation

* Increase of complexity. This proposal asks to include the entire StatefulSet and Service definitions as part of our `RabbitmqCluster` spec. Our current design decision is to have the spec reflects all its default values. This means that:
  * CRD manifest will become much harder to navigate. Even with minimal configurations, after creating RabbitMQ, users will see almost the entire manifest of StatefulSet and ingress Service as part of the CRD manifest.
  * Properties in the CRD spec will come much more nested. For example, a top-level property `spec.replicas` will become `spec.statefulSetTemplate.spec.replicas`, and `spec.tolerations` will become `spec.statefulSetTemplate.spec.template.spec.tolerations`.
* Increase of possibility of user errors. There will be many more kubernetes related properties for users to configure. Without proper understanding, users are now exposed at a greater risk of misconfigured deployments.

**Mitigation** to these above risks is to add the other layers of abstraction to our products. Users will then have a choice about their preferred granularity on how they would like to configure their RabbitMQ deployments. People with specific requirements on their deployment can choose `RabbitmqCluster` CRD, whereas users with minimal requirements on how to deploy RabbitMQ can choose a different CRD that's easy to navigate and maintain.

## Alternatives

### Alternative 1

Define our own StatefulSet and Service template to have better control over what people can configure.

## Upgrade Strategy

n/a

## Additional Details


## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY