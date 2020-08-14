---
title: CRD Spec Refactor
authors:
  - "@ChunyiLyu"
creation-date: 2020-03-10
last-updated: 2020-07-10
status: implemented
---

# CRD Spec Refactor

## Status

This KEP has already been implemented. Different from what's outlined in this KEP, we had to define some custome types instead of using `appsv1.StatefulSet` directly. For updated implementation details, refer to [PR #175](https://github.com/rabbitmq/cluster-operator/pull/175).

## Table of Contents

- [CRD Spec Refactor](#crd-spec-refactor)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      - [How do we apply overrides](#how-do-we-apply-overrides)
      - [Override values take precedence](#override-values-take-precedence)
      - [How should we handle reconciliation errors](#how-should-we-handle-reconciliation-errors)
    - [Additional context](#additional-context)
      - [Top level properties](#top-level-properties)
      - [kubernetes version requirement](#kubernetes-version-requirement)
    - [Update](#update)
    - [Risks and Mitigation](#risks-and-mitigation)
  - [Alternatives](#alternatives)
    - [Alternative 1](#alternative-1)
    - [Alternative 2](#alternative-2)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)

## Summary

Our current CRD spec has a limited number of exposed properties for users to configure which restricts supported use cases. This KEP proposes to add an override section as an additional way for users to configure their cluster. Users will be able to configure any field of the StatefulSet and client Service through editing the spec directly through this change.

## Motivation

Today, users are currently limited to a set of kubernetes properties that we select and define in the CRD spec to configure their RabbitMQ deployments. This approach has its downside, as we are currently not getting enough concrete user feedbacks to justify or prioritize each use case. And, the effort of adding these properties is repetitive and does not scale.

In our team roadmap we decided to coup the lack of user feedbacks by offering [different layers of abstractions](https://miro.com/app/board/o9J_kvlRPnc=/) to deploy RabbitMQ. The current `RabbitmqCluster` CRD, which is at the bottom of the abstraction layer, should provide little opinion on how users can configure their `RabbitmqCluster` instances. Instead, the CRD needs to allow users to customize it to whichever configurations that adhere to users' own requirements. For users with little specific requirements and just want to use an existing RabbitMQ configuration that their cluster operator has defined, they can use a different CRD from `RabbitmqCluster`.

## Goals

* Increase flexibility at configuring how to deploy RabbitMQ.

## Non-Goals/Future Work

* Increase flexibility at configuring RabbitMQ. We have already addressed this problem with the work on [rabbitmq conf](https://github.com/rabbitmq/cluster-operator/pull/91) and [enabled plugins](https://github.com/rabbitmq/cluster-operator/pull/87). In the future, we may add support for advanced configuration file. However that is out of scope for this KEP.
* To provide detailed guidelines on how to configure each property. I assume that users who choose to configure the StatefulSet and client Service override know their specific use cases, and how to use kubernetes.

## Proposal

This proposal adds the ability to override statefulSet and client service template directly through the CRD spec. Our operator creates 9 kubernetes child recourses directly for each `RabbitmqCluster`: client Service, headless Service, StatefulSet, ConfigMap, erlang cookie secret, admin secret, rbac role, role binding, service account. Among these resources, we allow users to partially configure the StatefulSet, the client Service, and the pods that StatefulSet creates. The proposal focuses on 2 child recourses: client Service and StatefulSet to increase configurability, since there is no obvious use case for now that involves configuring any of the other child resources. We can add overrides for other resources when we see fit in the future.

A brief summary of the proposed plan:
* Add an override section to CRD spec which supports statefulSetOverride and clientServiceOverride
* All existing CRD properties are kept and users can configure rabbitmq CRD through top level properties as before
* When the same property is configured at the top level and the override level , override value wins

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
	Override   RabbitmqClusterOverrideSpec     `json:"rabbitmqClusterOverrideSpec,omitempty"`
}

type RabbitmqClusterOverrideSpec struct {
	ClientService *corev1.Service     `json:"clientService,omitempty"`
	StatefulSet    *appsv1.StatefulSet `json:"statefulSet,omitempty"`
}
```

### Implementation Details/Notes/Constraints

#### How do we apply overrides

We should use [Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch) from kubernetes. Specifically, the function [`StrategicMergePatch`](https://github.com/kubernetes/apimachinery/blob/030f3066cc768bdc3c23f2fc03de06d18bc8147d/pkg/util/strategicpatch/patch.go#L812) can be used to apply the user provided override on top of generated StatefulSet and client Service definitions. This kubernetes specific patch formats contains specialized directives to control how specific fields are merged. There are several properties that are lists where the controller currently set defaults for. When users provide overrides to configure a specific item in such list, our controller should merge values from the default generated definitions and values from overrides, rather than replacing the entire list. Strategic Merge Patch can do just that.

```go

// originalStatefulSet is the generated statefulSet (what the controller already does today)
// statefulSetOverride is user-provided statefulSet Override

updated, err := strategicpatch.StrategicMergePatch(originalStatefulSet, statefulSetOverride, appsv1.StatefulSet{})
```

Lets take a look of the list of containers `spec.statefulSet.spec.template.containers` as an example, where our controller defines one container: `rabbitmq`.

Case 1) Addition to lists

To add a side car container, users can specify the following manifest on creation:

```yaml
kind: RabbitmqCluster
  spec:
    override:
      statefulSet:
        spec:
          template:
            spec:
              containers:
              - name: rabbit-side-car
                image: rabbit-side-car
            ...
```
[Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch) will append the `rabbit-side-car` container users provided. Created statefulSet will have both the `rabbitmq` container and the `rabbit-side-car` container in the same pod.

Case 2) Overwriting values in lists

To modify the already defined `rabbitmq` container, users need to specify the `patchMergeKey` for the list of Containers in the PodTemplate, which is `name`, and values they'd like to override:

```yaml
kind: RabbitmqCluster
  spec:
  override:
    statefulSet:
      spec:
        template:
          spec:
           containers:
            - name: rabbitmq # patchMergeKey
              image: rabbitmq:3.8-enterprise # override value
              env:
              - name: "RABBITMQ_DEFAULT_PASS_FILE"
                value: "/opt/my-custom-secret-file" # override value
...
```

`patchMergeKey` is defined for lists that support a merge operation. It is defined through go struct tags. Different list properties can have different `patchMergeKey`. For example, for list of containers in podTemplate, [`patchMergeKey` is "name"](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2869); whereas for list of ports in the Service spec, [`patchMergeKey` is "port"](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L3875).

Here are all properties that are lists where the controller sets defaults:

* spec.statefulSetTemplate.spec.template.volumes (`patchMergeKey`: name)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].env (`patchMergeKey`: name)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].ports (`patchMergeKey`: containerPort)
* spec.statefulSetTemplate.spec.template.spec.template.containers["rabbitmq"].volumeMounts (`patchMergeKey`: mountPath)
* spec.statefulSetTemplate.spec.template.initContainers (`patchMergeKey`: name)
* spec.clientServiceTemplate.spec.ports (`patchMergeKey`: ports)

Using [Strategic Merge Patch](https://github.com/kubernetes/apimachinery/tree/master/pkg/util/strategicpatch), we ensure that users have the flexibility to configure properties and that they do not need to specify properties that they don't modify.

_Side note_ Not all lists support this merging operation. For example, statefulSetSpec.volumeClaimTemplates does not support merge, only replace.

#### Override values take precedence

Values we currently expose in CRD spec top level will be able to configure through StatefulSet and client Service override. When users provide values both in the top level properties and in override, values in override will win. For example, if both `spec.replicas` and `spec.replicas.override.statefulSet.spec.replicas` are provided, value provided in `spec.replicas.override.statefulSet.spec.replicas` will take precedence. On the other hand, if replicas value is not provided in statefulSet override, operator will use the provided `spec.replicas` value or the default value if `spec.replicas` is not set neither.

This also applies for map values, like annotations. Right now, there are two places where people can specify client service annotations: `metadata.annotations`, which is for annotations applied to all child resources including client Service; `spec.service.annotations`, which is client Service only. Since `annotations` is a map value and not a list value, there is no "merging" behavior if client Service annotation is specified in the override. Whatever users provided in `spec.overide.clientService.metadata.annotations` will be used for client Service.

#### How should we handle reconciliation errors

By using kubernetes native types, we can get schema validations for free. However, there are errors that occur when creating and updating child resources. Our controller must find a consistent and obvious way to surface these errors as users actions are required to fix them. Such errors are only logged at the moment, which is not ideal.

Examples on reconcilation errors that happen during create and update:
 
* Updates to StatefulSet spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden and will only return errors during update calls.
* Set client Service ports protocol to values other than "TCP", "UDP", and "SCTP" will return an error on create or update.

We should revisit/prioritize [github issue #10](https://github.com/rabbitmq/cluster-operator/issues/10) which requests a new status.condition to surface reconciliation errors.

### Additional context

#### Top level properties

This proposal does not stop us from exposing more fields in top level CRD spec. We can continue to add more properties to top level CRD spec if we see a common use case for the properties. For example, if many users configure the override for the StatefulSet in a certain way, then we should probably think about how we can address the use case by providing a higher-level more concrete property in the CR for usability.

#### kubernetes version requirement

The proposal asks to use StatefulSet and Service k8s api objects directly in the code. At the moment, we use k8s api version `1.17`  which has a new set of golang marker defined in both StatefulSet and Service templates that are added since version `1.16` for server-side apply functionality. If we continue to use k8s api `1.17` and together with using k8s api `1.17` objects directly in our CRD, users have to deploy to a `1.16` cluster or above because `1.15` k8s api does not recognize new markers, specifically marker [`listType` and `listMapKeys`](https://kubernetes.io/docs/reference/using-api/api-concepts/#merge-strategy).

### Update

Any update to the StatefulSet and client Service override will be reconciled by our controller. This means that certain updates will trigger reconciliation errors. In that case, reconciliation errors should be surfaced through status.conditions. Some updates on the pod template of the StatefulSet template will trigger a StatefulSet restart. My assumption here is that users who use these override knows what they are doing and triggering a StatefulSet restart won't be a concern for us.

### Risks and Mitigation

* Increase of possibility of user errors. There will be many more kubernetes related properties for users to configure. Without proper understanding, users are now exposed at a greater risk of misconfigured deployments.

**Mitigation** This proposal keeps the existing high-level properties as they are, e.g. replicas, persistence, service type, images, etc. This proposal assumes that only a small fraction of people will have to need to use the override section of the CR. If they do, they have specific use cases and we expect them to have basic understanding about kubernetes prior to using our operator. In addition to that, we will add the other layers of abstraction to our products. Users will then have a choice about their preferred granularity on how they would like to configure their RabbitMQ deployments.

* Two places to configure values to achieve the same effect can be confusing

**Mitigation**  we will need to make it clear that the override section is to configure any values that users cannot access from the top level.

## Alternatives

### Alternative 1

Define our own StatefulSet and Service template to have better control over what people can configure.

### Alternative 2

A new structure for the CRD spec where it uses the StatefulSet and Service kubernetes templates directly in the spec. This has been explored from previous version of this KEP. It achieves the same level of flexibility and configurability as the this KEP's proposed solution.

However, this alternative approach has the downside of increasing complexity. As users will need to understand  the internal resources behind the CRD to be able to configure common properties that we expose at top level today. For example, for users to configure rabbitmq image, instead of using `spec.image` at the top level, they will have to configure `spec.statefulSetTemplate.spec.template.spec.containers[0].image` with this alternative approach. In addition, if our controller fills in defaults for templates like what the controller currently does, CRD manifests will have the complete manifests of the statefulSet and client Service. CRD manifests will become harder to navigate and to modify for users after creation.

## Upgrade Strategy

Schema changes in this proposal will be backward compatible. This feature can use as an experiment to drive out CRD versioning.

## Additional Details

n/a
