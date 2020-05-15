---
title: CRD Spec Refactor
authors:
  - "@ChunyiLyu"
reviewers:
  -
creation-date: 2020-03-10
last-updated: 2020-05-12
status: provisional
see-also:
replaces:
superseded-by:
---

# CRD Spec Refactor

## Table of Contents

- [CRD Spec Refactor](#crd-spec-refactor)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      - [How do we apply overwrites](#how-do-we-apply-overwrites)
      - [How should we handle reconciliation errors](#how-should-we-handle-reconciliation-errors)
    - [Update](#update)
    - [Risks and Mitigation](#risks-and-mitigation)
  - [Alternatives](#alternatives)
    - [Alternative 1](#alternative-1)
    - [Alternative 2](#alternative-2)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)

## Summary

Our current CRD spec has a limited number of exposed properties for users to configure which restricts supported use cases. This KEP proposes to add an overwrite section as an additional way for users to configure their cluster. Users will be able to configure any field of the StatefulSet and ingress Service through editing the spec directly through this change.

## Motivation

Today, users are currently limited to a set of kubernetes properties that we select and define in the CRD spec to configure their RabbitMQ deployments. This approach has its downside, as we are currently not getting enough concrete user feedbacks to justify or prioritize each use case. And, the effort of adding these properties is repetitive and does not scale.

In our team roadmap we decided to coup the lack of user feedbacks by offering [different layers of abstractions](https://miro.com/app/board/o9J_kvlRPnc=/) to deploy RabbitMQ. The current `RabbitmqCluster` CRD, which is at the bottom of the abstraction layer, should provide little opinion on how users can configure their `RabbitmqCluster` instances. Instead, the CRD needs to allow users to customize it to whichever configurations that adhere to users' own requirements. For users with little specific requirements and just want to use an existing RabbitMQ configuration that their cluster operator has defined, they can use a different CRD from `RabbitmqCluster`.

## Goals

* Increase flexibility at configuring how to deploy RabbitMQ.

## Non-Goals/Future Work

* Increase flexibility at configuring RabbitMQ. We have already addressed this problem with the work on [rabbitmq conf](https://github.com/pivotal/rabbitmq-for-kubernetes/pull/91) and [enabled plugins](https://github.com/pivotal/rabbitmq-for-kubernetes/pull/87). In the future, we may add support for advanced configuration file. However that is out of scope for this KEP.
* To provide detailed guidelines on how to configure each property. I assume that users who choose to configure the StatefulSet and ingress Service overwrite know their specific use cases, and how to use kubernetes.

## Proposal

This proposal adds the ability to overwrite statefulSet and ingress service template directly through the CRD spec. Our operator creates 9 kubernetes child recourses directly for each `RabbitmqCluster`: ingress Service, headless Service, StatefulSet, ConfigMap, erlang cookie secret, admin secret, rbac role, role binding, service account. Among these resources, we allow users to partially configure the StatefulSet, the ingress Service, and the pods that StatefulSet creates. The proposal focuses on 2 child recourses: ingress Service and StatefulSet to increase configurability, since there is no obvious use case for now that involves configuring any of the other child resources. We can add overwrites for other resources when we see fit in the future.

A brief summary of the proposed plan:
* Add an overwrite section to CRD spec which supports statefulSetOverwrite and ingressServiceOverwrite
* All existing CRD properties are kept and users can configure rabbitmq CRD through top level properties as before
* When the same property is configured at the top level and the overwrite level , overwrite value wins

```go
type RabbitmqCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec   RabbitmqClusterSpec   `json:"spec,omitempty"`
	Status RabbitmqClusterStatus `json:"status,omitempty"`
}

type RabbitmqClusterSpec struct {
	Replicas int32 `json:"replicas"`
	Image string `json:"image,omitempty"`
	ImagePullSecret string                         `json:"imagePullSecret,omitempty"`
	Service         RabbitmqClusterServiceSpec     `json:"service,omitempty"`
	Persistence     RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	Resources       *corev1.ResourceRequirements   `json:"resources,omitempty"`
	Affinity        *corev1.Affinity               `json:"affinity,omitempty"`
	Tolerations []corev1.Toleration              `json:"tolerations,omitempty"`
	Rabbitmq    RabbitmqClusterConfigurationSpec `json:"rabbitmq,omitempty"`
	Overwrite   RabbitmqClusterOverwriteSpec     `json:"rabbitmqClusterOverwriteSpec,omitempty"`
}

type RabbitmqClusterOverwriteSpec struct {
	IngressServiceOverwrite *corev1.Service     `json:"ingressServiceOverwrite,omitempty"`
	StatefulSetOverwrite    *appsv1.StatefulSet `json:"statefulSetOverwrite,omitempty"`
}
```

### Implementation Details/Notes/Constraints

#### How do we apply overwrites

We should use [Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch) from kubernetes. Specifically, the function [`StrategicMergePatch`](https://github.com/kubernetes/apimachinery/blob/030f3066cc768bdc3c23f2fc03de06d18bc8147d/pkg/util/strategicpatch/patch.go#L812) can be used to apply the user provided overwrite on top of generated StatefulSet and ingress Service definitions. This kubernetes specific patch formats contains specialized directives to control how specific fields are merged. There are several properties that are lists where the controller currently set defaults for. When users provide overwrites to configure a specific item in such list, our controller should merge values from the default generated definitions and values from overwrites, rather than replacing the entire list. Strategic Merge Patch can do just that.

```go

// originalStatefulSet is the generated statefulSet (what the controller already does today)
// statefulSetOverwrite is user-provided statefulSet Overwrite

updated, err := strategicpatch.StrategicMergePatch(originalStatefulSet, statefulSetOverwrite, appsv1.StatefulSet{})
```

Lets take a look of the list of containers `spec.statefulSetOverWrite.spec.template.containers` as an example, where our controller defines one container: `rabbitmq`.

Case 1) Addition to lists

To add a side car container, users can specify the following manifest on creation:

```yaml
kind: RabbitmqCluster
  spec:
    overWrite:
      statefulSetOverWrite:
        spec:
          template:
            spec:
              containers:
              - name: rabbit-side-cat
                image: rabbit-side-cat
            ...
```
[Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch) will append the `rabbit-side-car` container users provided. Created statefulSet will have both the `rabbitmq` container and the `rabbit-side-cat` container in the same pod.

Case 2) Overwriting values in lists

To modify the already defined `rabbitmq` container, users need to specify the `patchMergeKey` for the list of Containers in the PodTemplate, which is `name`, and values they'd like to overwrite:

```yaml
kind: RabbitmqCluster
  spec:
  overWrite:
    statefulSetOverWrite:
      spec:
        template:
          spec:
           containers:
            - name: rabbitmq # patchMergeKey
              image: rabbitmq:3.8-enterprise # overwrite value
              env:
              - name: "RABBITMQ_DEFAULT_PASS_FILE"
                value: "/opt/my-custom-secret-file" # overwrite value
...
```

`patchMergeKey` is defined for lists that support a merge operation. It is defined through go struct tags. Different list properties can have different `patchMergeKey`. For example, for list of containers in podTemplate, [`patchMergeKey` is "name"](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2869); whereas for list of ports in the Service spec, [`patchMergeKey` is "port"](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L3875).

Here are all properties that are lists where the controller sets defaults:

* spec.statefulSetTemplate.spec.template.volumes (`patchMergeKey`: name)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].env (`patchMergeKey`: name)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].ports (`patchMergeKey`: containerPort)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].volumeMounts (`patchMergeKey`: mountPath)
* spec.statefulSetTemplate.spec.template.initContainers (`patchMergeKey`: name)
* spec.ingressServiceTemplate.spec.ports (`patchMergeKey`: ports)

Using [Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch), we ensure that users have the flexibility to configure properties and that they do not need to specify properties that they don't modify.

_Side note_ Not all lists support this merging operation. For example, statefulSetSpec.volumeClaimTemplates does not support merge, only replace.

#### How should we handle reconciliation errors

By using kubernetes native types, we can get schema validations for free. However, there are errors that occur when creating and updating child resources. Our controller must find a consistent and obvious way to surface these errors as users actions are required to fix them. Such errors are only logged at the moment, which is not ideal.

Examples on reconcilation errors that happen during create and update:
 
* Updates to StatefulSet spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden and will only return errors during update calls.
* Set ingress Service ports protocol to values other than "TCP", "UDP", and "SCTP" will return an error on create or update.

We should revisit/prioritize [github issue #10](https://github.com/pivotal/rabbitmq-for-kubernetes/issues/10) which requests a new status.condition to surface reconciliation errors.

### Update

Any update to the StatefulSet and ingress Service overwrite will be reconciled by our controller. This means that certain updates will trigger reconciliation errors. In that case, reconciliation errors should be surfaced through status.conditions. Some updates on the pod template of the StatefulSet template will trigger a StatefulSet restart. My assumption here is that users who use these overwrite knows what they are doing and triggering a StatefulSet restart won't be a concern for us.

### Risks and Mitigation

* Increase of possibility of user errors. There will be many more kubernetes related properties for users to configure. Without proper understanding, users are now exposed at a greater risk of misconfigured deployments.

**Mitigation** Add the other layers of abstraction to our products. Users will then have a choice about their preferred granularity on how they would like to configure their RabbitMQ deployments.

* Two places to configure values to achieve the same effect can be confusing

**Mitigation**  we will need to make it clear that the overwrite section is to configure any values that users cannot access from the top level. 

## Alternatives

### Alternative 1

Define our own StatefulSet and Service template to have better control over what people can configure.

### Alternative 2

A new structure for the CRD spec where it uses the StatefulSet, and Service kubernetes template directly in the spec. This has been explored from previous version of this KEP. It achieves the same level of flexibility and configurability as the this KEP's proposed solution. However, it has the downside of increasing complexity.

## Upgrade Strategy

Schema changes in this proposal will be backward compatible. This feature can use as an experiment to drive out CRD versioning.

## Additional Details

n/a