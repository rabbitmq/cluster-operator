---
title: RabbitMQ Cluster Operator Versioning & Release Strategy
authors:
  - "@coro"
reviewers:
  - "@ChunyiLyu"
  - "@ferozjilla"
  - "@harshac"
  - "@j4mcs"
  - "@mkuratczyk"
  - "@michaelklishin"
  - "@Zerpet"
creation-date: 2020-08-06
last-updated: 2020-08-06
status: provisional
see-also:
  - https://github.com/rabbitmq/cluster-operator/issues/190
---

# RabbitMQ Cluster Operator Versioning & Release Strategy

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

## Glossary

### Preferred / Storage version

For a given Kubernetes resource with multiple API versions, one API version is considered the 'preferred' or 'storage' version for that resource. Any differently-versioned representations of that resource are losslessly converted to the storage version by the Kubernetes API server prior to saving the resource in etcd.

The latest API version of a resource is almost always the storage version, with the exception of the first release of the software providing the resource, which is not permitted to use the latest API version as the storage version as per the [Kubernetes Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/).

## Summary

Versioning exists primarily as a simple method of communicating changes to a piece of software. Some users will infer the impact of an upgrade solely on the version number associated with a release, and so it is important that the versioning scheme does not misrepresent or imply a scope of change that does not match reality.

This proposal investigates the different options for versioning the operator, and puts forward a release management strategy to outline how users may observe & consume these versions.

## Motivation

In the case of the operator, a change in version might represent:

- A change in functionality, e.g. a bug fix in the operator or new optional configuration option
- A change in compatibility, e.g. supporting/deprecating compatibility for a Kubernetes server version, or RabbitMQ version
- A change in resource API versions, e.g. introducing a new API version for the RabbitmqCluster or changing the Storage version
- A change in commercial support, since new versions are typically supported for a number of months after GA release

It is imperative that a user might interpret a change in version of the operator and be able to make **informed** decisions about how / when to consume the release.

### Goals

#### Discovery

- Make it simple for users to find the latest version of the operator

#### Comprehension

- Avoid misleading interpretations of version numbers
- Allow users to understand the changes to the operator between versions
- Instruct users to take action where necessary to consume a new version of the operator (such as where an API version is no longer supported as per the [Kubernetes Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/).

#### Support

- Allow for compatibility with the [Kubernetes Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/).
  - In particular, the operator release strategy must provide support for new API versions of the RabbitmqCluster CR for a minimum term:
    - GA: 12 months or 3 releases (whichever is longer)
    - Beta: 9 months or 3 releases (whichever is longer)
    - Alpha: 0 releases
- Allow for a sensible commercial support schedule of the operator
- Provide support for all GA versions of RabbitMQ & Kubernetes server, where possible

#### Consumption

- Allow users to consume & deploy the version of their choice as simply as possible, preferrably with a single command

### Non-Goals/Future Work

This proposal deliberately avoids the term 'best practice'. This is for two reasons:

- The nomenclature of 'Best practice' can falsely imply a history of debate & rigour
- Consistency with similar software is definitely an advantage to reduce confusion of a user, however is not the only consideration to be made
- Best practices do not remain 'best' forever


## Versioning of related software & dependencies

### RabbitMQ

`rabbitmq-server` follows a MAJOR.MINOR.PATCH syntax for its releases, however does not follow strict SemVer. For example, [release 3.8.4](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v3.8.4) included potentially
breaking changes in response to a medium severity bug, and was a PATCH release.

Some tools in RabbitMQ do stick to strict SemVer, for example the [`rabbitmq-dotnet-client`](https://github.com/rabbitmq/rabbitmq-dotnet-client). However, this has been found to hinder the team's ability to ship small
improvements which would necessitate interface changes.

### Kubernetes & components

Kubernetes [purportedly follows Semantic Versioning](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/release/versioning.md#kubernetes-release-versioning). However, as demonstrated by the release of 1.18,
their MINOR version changes can still contain breaking changes, and so its following of SemVer will not be considered strict.

### Other operators

After scrolling through [OperatorHub.io](https://operatorhub.io/), it appears that the default versioning for operators is SemVer. Many of them are not post-v1, allowing for breaking changes in minors (see the [Prometheus Operator 0.24.0](https://github.com/prometheus-operator/prometheus-operator/releases/tag/v0.24.0) for example). There appears to be little use of patch versions, favouring a versioning scheme of 0.x.0.

## Options

### Release per-commit

### CalVer

### SemVer

[Semantic Versioning](https://semver.org/spec/v2.0.0.html), or SemVer for short, attributes any software change to one of three buckets: MAJOR, MINOR or PATCH. Only MAJOR version changes contain non-backwards compatible changes, and so
theoretically one could automatically consume any MINOR or PATCH changes without fear of breakages. The model of SemVer works well where there is a use case for maintainance branches of code; it is theoretically possible that any MINOR or PATCH
changes can be backported over to old MAJOR versions of the software.

One issue with SemVer is that it attempts to give equal weight to all breaking changes, resulting in major version explosion. Due to commercial support policies, this may result in the required support of many major versions of a product.
It also makes it harder to ship small improvements that require interface changes. Consider what a MAJOR version change might look like in the operator. At the very least, with Kubernetes cluster versions being deprecated quarterly,
the operator arguably would need a MAJOR version change every quarter to correctly inform of breaking changes. 



## Proposal




### User Stories

#### Story 1 - New patch release of RabbitMQ

#### Story 2 - 

### Implementation Details/Notes/Constraints

- What are some important details that didn't come across above.
- What are the caveats to the implementation?
- Go in to as much detail as necessary here.
- Talk about core concepts and how they releate.

### Risks and Mitigations

- What are the risks of this proposal and how do we mitigate? Think broadly.
- How will UX be reviewed and by whom?
- How will security be reviewed and by whom?
- Consider including folks that also work outside the SIG or subproject.

## Alternatives

The `Alternatives` section is used to highlight and record other possible approaches to delivering the value proposed by a proposal.

## Additional Details

## Implementation History

- [ ] MM/DD/YYYY: Open proposal PR

