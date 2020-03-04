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

# Upgrade RabbitMQ for Kubernetes

## Table of Contents

A table of contents is helpful for quickly jumping to sections of a proposal and for highlighting
any additional information provided beyond the standard proposal template.
[Tools for generating](https://github.com/ekalinin/github-markdown-toc) a table of contents from markdown are available.

- [Title](#title)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [User Stories [optional]](#user-stories-optional)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Implementation Details/Notes/Constraints [optional]](#implementation-detailsnotesconstraints-optional)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Design Details](#design-details)
    - [Test Plan](#test-plan)
    - [Graduation Criteria](#graduation-criteria)
      - [Examples](#examples)
        - [Alpha -> Beta Graduation](#alpha---beta-graduation)
        - [Beta -> GA Graduation](#beta---ga-graduation)
        - [Removing a deprecated flag](#removing-a-deprecated-flag)
    - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
    - [Version Skew Strategy](#version-skew-strategy)
  - [Implementation History](#implementation-history)
  - [Drawbacks [optional]](#drawbacks-optional)
  - [Alternatives [optional]](#alternatives-optional)
  - [Infrastructure Needed [optional]](#infrastructure-needed-optional)

## Glossary [WIP]

Refer to the [Cluster API Book Glossary](https://cluster-api.sigs.k8s.io/reference/glossary.html).

If this proposal adds new terms, or defines some, make the changes to the book's glossary when in PR stage.

## Summary

We propose a high level overview of the expected customer experience we would like users to have when upgrading all RabbitMQ for Kubernetes components. At the moment we see four broad topics to be covered in depth related to upgrades:
- Upgrading CRD versions
- Upgrading RabbitMQ version
- Bulk upgrades for CRD version
- Bulk upgrades for RabbitMQ version

Each topic will be explored more deeply with a separate KEP.
## Motivation

As we experiment and understand more about upgrades, we would like a single place to refer to when trying to understand our current progress towards creating a wholistic upgrade experience. We believe that this KEP will allow us to both track current progress and plan next steps towards improvements.

We know that the ability to make a minimal set of guarantees about upgrades is essential towards deciding when to promote our API version to GA. We have had previous discussions about what this set of guarantees might look like for individual components of our product. The motivation is therefore to track and improve on that set of guarantees while providing an appropriate medium for distributed collaboration.

### Goals

- To track current and future sets of guarantees made about the upgrades of of RabbitMQ for Kubernetes components
- Prioritise these behaviours to form a road map
- Create a release mark that indicates when we would be confident to say that our upgrade journey is ready for GA

### Non-Goals/Future Work

- To create detailed solutions for the upgrade journey of each component
- To designate a release marker for GA - this document will simply detail the upgrade requirements for GA, other system requirements may still block the path to GA.

## Proposal [WIP]

- Explain upgrade philosophy
Our objective with upgrades is to seek to acquire a definition of the minimum set of required conditions for an upgrade journey to start from, so we can iterate towards the ideal upgrade journey over time. This set of requirements can, or Minimum Viable Upgrade, can then be used as a target for GA, because we can be confident that we have covered the minimum required user journey while giving ourselves a chance to iterate towards implementing better workflows.

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
