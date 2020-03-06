---
title: proposal Template
authors:
  - "@chewymeister"
reviewers:
creation-date: 2020-02-26
last-updated: 2020-02-26
status: provisional
see-also:
replaces:
superseded-by:
---

# Upgrade Criterias

## Table of Contents

- [Upgrade RabbitMQ for Kubernetes](#upgrade-rabbitmq-for-kubernetes)
  - [Table of Contents](#table-of-contents)
  - [Glossary [WIP]](#glossary-wip)
    - [What is an upgrade?](#what-is-an-upgrade)
    - [RabbitMQ Instance](#rabbitmq-instance)
    - [Control plane](#control-plane)
    - [Minimum Viable Upgrade](#minimum-viable-upgrade)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [The upgrade journey](#the-upgrade-journey)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal [WIP]](#proposal-wip)
    - [Data loss](#data-loss)
    - [Consistency](#consistency)
    - [Availability](#availability)
    - [Repeatability (TODO define repeatability)](#repeatability-todo-define-repeatability)
    - [Performance degradation](#performance-degradation)
    - [User Stories [WIP]](#user-stories-wip)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Implementation Details/Notes/Constraints [WIP]](#implementation-detailsnotesconstraints-wip)
    - [Risks and Mitigations [WIP]](#risks-and-mitigations-wip)
  - [Alternatives [WIP]](#alternatives-wip)
  - [Upgrade Strategy [WIP]](#upgrade-strategy-wip)
  - [Additional Details [WIP]](#additional-details-wip)
    - [Test Plan [optional]](#test-plan-optional)
    - [Graduation Criteria [optional]](#graduation-criteria-optional)
    - [Version Skew Strategy [optional]](#version-skew-strategy-optional)
  - [Implementation History](#implementation-history)

## Glossary

### What is an upgrade?
An upgrade is a transition from software version A to B. For sake of simplicity this document assumes that version B is always higher than A, i.e. we do not look at downgrades.

### RabbitMQ Instance
This is a software boundary that describes the components running native Kubernetes resources that ultimately make up the Rabbitmq deployment. This includes:
- StatefulSet
- Pods
- Service
- ConfigMap
- Secret

One of the most significant upgrade changes in this category is bumping the version of the RabbitMQ server, i.e. upgrading the RabbitMQ container image.

### Control plane
This is a software boundary that helps us describe the components that manage the deployment of the RabbitMQ instance. This includes:
- CRD
- Controller

### Minimum Viable Upgrade
For each layer in our stack

## Summary

<!-- We propose a high level overview of the expected customer experience we would like users to have when upgrading all RabbitMQ for Kubernetes components. At the moment we see four broad topics to be covered in depth related to upgrades:
- Upgrading CRD versions
- Upgrading RabbitMQ version
- Bulk upgrades for CRD version
- Bulk upgrades for RabbitMQ version

Each topic will be explored more deeply with a separate KEP. -->

We have come up with a set of criteria that we think is important to consider when thinking about upgrades. We hope to use the definitions of criteria in this document to think about future broad topics related to upgrades.

## Motivation

 We believe that if we analyse these different criteria across the Operator and the RabbitMQ Instance, we can better analyse how the system will behave during upgrades, and communicate our expectations to the user.

### The upgrade journey
There are a number of potential ways that customers could get from version A to B:
- They could delete the version A of their workload and deploy the new version B
- They could migrate data from the version A workload to the new version B workload
- They could manually perform a blue-green deployment and switch load balancer to point to the workload version B once it has been successfully migrated
- They could manually upgrade in place the workload (RabbitMQ server) and the control plane (CRD)
- They could have an automated blue-green deployment
- They could have an automated in-place upgrade

All of these are a possible journey that Alanas and Codys could take, and each journey contains a number of trade-offs. It is the objective of this document to help discover what those trade-offs are and to create a framework to ask our customers which trade-offs they are willing to accept in order to guide our product towards continually maintaining an upgrade journey that is within the risk appetite of our customers.

In other words, our objective should be to seek to acquire a definition of the minimum set of required conditions for an upgrade journey to start from, so we can iterate towards the ideal upgrade journey over time. This set of requirements can then be used as a target for GA, because we can be confident that we have covered the minimum required user journey while giving ourselves a chance to iterate towards implementing better workflows.

### Goals

- To track current and future sets of guarantees made about the upgrades of of RabbitMQ for Kubernetes components
- Prioritise these behaviours to form a road map

### Non-Goals/Future Work

- To create detailed solutions for the upgrade journey of each component
- To designate a release marker for GA - this document will simply detail the upgrade requirements for GA, other system requirements may still block the path to GA.

## Proposal

Our objective with upgrades is to seek to acquire a definition of the minimum set of required conditions for an upgrade journey to start from, so we can iterate towards the ideal upgrade journey over time. This set of requirements, or Minimum Viable Upgrade, can then be used as a target for GA, because we can be confident that we have covered the minimum required user journey while giving ourselves a chance to iterate towards implementing better workflows in future versions.

We can summarise the upgrade journeys by listing out aspects of the behaviour of the system during an upgrade:

### Data loss
Data loss during an upgrade could happen at either the Data Plane (losing messages, RabbitMQ resource metadata, or configuration data), the Control Plane (losing track of existing instances and their current state or losing existing configuration).

We consider data loss prevention on both planes to be the most important aspect of the upgrade experience we want to provide.

### Consistency
Somewhat related to Data Loss but not quite the same, consistency is all about making sure that at the very minimum the upgrade moves the system from one valid state to another one. In an ideal world, the system would always remain in a consistent state even during the upgrade. A good upgrade solution would make sure that even in those cases where the upgrade fails the system is left in an operational and consistent state.

In our case, consistency might mean different things depending on the user we are looking at. For example, consistency for application developers could mean that their service bindings (coordinates and credentials) stay the same at any point in time. They probably also care about not consuming one and the same message twice; or that if they published a message and got an acknowledgment from the server, they can be sure that the message is queued correctly and not being lost subsequently.

For the platform operator, consistency might mean that they do not see multiple incarnations of one and the same RabbitMQ instance once the upgrade is finished or that the number of nodes in a given RabbitMQ cluster remains the same after an upgrade.

### Availability
Availability affects both the data plane and the control plane, i.e. availability of the RabbitMQ instances and availability of the Control Plane. Moreover, we should consider potential side-effects the way we conduct control plane and instance upgrades might have on the availability of the K8s cluster (e.g. API server, etc).

Different users may have different requirements for the availability for either the data or the control plane. This is something we will have to find out more about from our customers.

### Repeatability (TODO define repeatability)

### Performance degradation
RabbitMQ is commonly used as the backing messaging system responsible for communication within a Microservice ecosystem. Different users will have different performance tolerations for upgrades. We will therefore have to set expectations by providing well researched data that demonstrates the level of performance they can expect during an upgrade. Like any research paper, we will have to frame the data with a given set of constraints. Commonly 

Usability (a scale between automation and manual steps)

To help us track
- Include Michal's points about minimal viable upgrade for GA. We can include some suggestion to be discussed in the solutions section.

### User Stories [WIP]

- Detail the things that people will be able to do if this proposal is implemented.
- Include as much detail as possible so that people can understand the "how" of the system.
- The goal here is to make this feel real for users without getting bogged down.

#### Story 1

#### Story 2

### Implementation Details/Notes/Constraints [WIP]

- What are some important details that didn't come across above.
- What are the caveats to the implementation?
- Go in to as much detail as necessary here.
- Talk about core concepts and how they releate.

### Risks and Mitigations [WIP]

- What are the risks of this proposal and how do we mitigate? Think broadly.
- How will UX be reviewed and by whom?
- How will security be reviewed and by whom?
- Consider including folks that also work outside the SIG or subproject.

## Alternatives [WIP]

The `Alternatives` section is used to highlight and record other possible approaches to delivering the value proposed by a proposal.

## Upgrade Strategy [WIP]

If applicable, how will the component be upgraded? Make sure this is in the test plan.

Consider the following in developing an upgrade strategy for this enhancement:
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to make on upgrade in order to make use of the enhancement?

## Additional Details [WIP]

### Test Plan [optional]

**Note:** *Section not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all of the test cases, just the general strategy.
Anything that would count as tricky in the implementation and anything particularly challenging to test should be called out.

All code is expected to have adequate tests (eventually with coverage expectations).
Please adhere to the [Kubernetes testing guidelines][testing-guidelines] when drafting this test plan.

[testing-guidelines]: https://git.k8s.io/community/contributors/devel/sig-testing/testing.md

### Graduation Criteria [optional]

**Note:** *Section not required until targeted at a release.*

Define graduation milestones.

These may be defined in terms of API maturity, or as something else. Initial proposal should keep
this high-level with a focus on what signals will be looked at to determine graduation.

Consider the following in developing the graduation criteria for this enhancement:
- [Maturity levels (`alpha`, `beta`, `stable`)][maturity-levels]
- [Deprecation policy][deprecation-policy]

Clearly define what graduation means by either linking to the [API doc definition](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning),
or by redefining what graduation means.

In general, we try to use the same stages (alpha, beta, GA), regardless how the functionality is accessed.

[maturity-levels]: https://git.k8s.io/community/contributors/devel/sig-architecture/api_changes.md#alpha-beta-and-stable-versions
[deprecation-policy]: https://kubernetes.io/docs/reference/using-api/deprecation-policy/

### Version Skew Strategy [optional]

If applicable, how will the component handle version skew with other components? What are the guarantees? Make sure
this is in the test plan.

Consider the following in developing a version skew strategy for this enhancement:
- Does this enhancement involve coordinating behavior in the control plane and in the kubelet? How does an n-2 kubelet without this feature available behave when this feature is used?
- Will any other components on the node change? For example, changes to CSI, CRI or CNI may require updating that component before the kubelet.

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY
