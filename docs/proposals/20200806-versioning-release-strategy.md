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
last-updated: 2020-09-24
status: provisional
see-also:
  - https://github.com/rabbitmq/cluster-operator/issues/190
---

# RabbitMQ Cluster Operator Versioning & Release Strategy

## Table of Contents

<!--ts-->
   * [RabbitMQ Cluster Operator Versioning &amp; Release Strategy](#rabbitmq-cluster-operator-versioning--release-strategy)
      * [Table of Contents](#table-of-contents)
      * [Glossary](#glossary)
         * [Preferred / Storage version](#preferred--storage-version)
      * [Summary](#summary)
      * [Motivation](#motivation)
         * [Goals](#goals)
            * [Discovery](#discovery)
            * [Comprehension](#comprehension)
            * [Support](#support)
            * [Consumption](#consumption)
         * [Non-Goals/Future Work](#non-goalsfuture-work)
      * [Versioning of related software &amp; dependencies](#versioning-of-related-software--dependencies)
         * [RabbitMQ](#rabbitmq)
         * [Kubernetes &amp; components](#kubernetes--components)
         * [Other operators](#other-operators)
      * [Options](#options)
         * [Release per-commit](#release-per-commit)
         * [SemVer](#semver)
         * [FerVer](#ferver)
         * [CalVer](#calver)
      * [Proposal - versioning](#proposal---versioning)
         * [Alternatives](#alternatives)
      * [Proposal - releasing](#proposal---releasing)
         * [Compatibility](#compatibility)
            * [Supported range of RabbitMQ versions](#supported-range-of-rabbitmq-versions)
            * [Supported range of Kubernetes server versions](#supported-range-of-kubernetes-server-versions)
            * [API Versions](#api-versions)
         * [GitHub Release](#github-release)
            * [Release notes](#release-notes)
            * [Artefacts](#artefacts)
      * [User Stories](#user-stories)
         * [Story 1 - Consuming the latest operator release](#story-1---consuming-the-latest-operator-release)
         * [Story 2 - Consuming a specific operator release](#story-2---consuming-a-specific-operator-release)
         * [Story 3 - Consuming the latest commit of the operator](#story-3---consuming-the-latest-commit-of-the-operator)
         * [Story 4 - New patch release of RabbitMQ](#story-4---new-patch-release-of-rabbitmq)
         * [Story 5 - New minor release of RabbitMQ](#story-5---new-minor-release-of-rabbitmq)
         * [Story 6 - Release of API Group rabbitmq.com/v2](#story-6---release-of-api-group-rabbitmqcomv2)
      * [Accepted Proposal - use non-strict SemVer](#accepted-proposal---use-non-strict-semver)
      * [Accepted Proposal - releasing](#accepted-proposal---releasing)
      * [Implementation History](#implementation-history)

<!-- Added by: coro, at: Mon Aug 10 14:50:15 UTC 2020 -->

<!--te-->

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

Currently, at least internally, we generate a new release per-commit. This involves tagging the operator Docker image for every commit to this repo. As more people try out the operator, the ability to
help any people who encounter issues is hindered by this approach - instead of asking what version they are on, we have to find out a whole SHA of the commit. It also becomes difficult to offer
technical support in the way of fixes, save for simply asking them to upgrade (we're not going to fork off every commit and backport fixes!).

### SemVer

[Semantic Versioning](https://semver.org/spec/v2.0.0.html), or SemVer for short, attributes any software change to one of three buckets: MAJOR, MINOR or PATCH. Only MAJOR version changes contain non-backwards compatible changes, and so
theoretically one could automatically consume any MINOR or PATCH changes without fear of breakages. The model of SemVer works well where there is a use case for maintainance branches of code; it is theoretically possible that any MINOR or PATCH
changes can be backported over to old MAJOR versions of the software.

SemVer is certainly ambitious. [Critics of the versioning system](https://gist.github.com/jashkenas/cbd2b088e20279ae2c8e) claim it attempts to compress too much contextual information about changes into a single number change,
ultimately misleading the consumers of the software by providing false confidence in especially MINOR changes. There is an implicit association of 3-digit versioning with SemVer, meaning that regardless of if a project follows
SemVer or not, there is a likelihood that consumers of the software will draw conclusions about whether they can upgrade safely from this version number, without reading the changelog.

One issue with SemVer is that it attempts to give equal weight to all breaking changes, resulting in major version explosion. If a piece of software introduces a change that breaks
only 5% of users, SemVer would have that be a MAJOR version bump. This would be true even if no-one used the feature that was being removed in a breaking fashion.
The Kuberenetes API allows us to create and deprecate API versions as often we please (provided we continue to support them for a fixed term after announcing deprecation).

Following strict-SemVer, every time we remove one of these, it's a MAJOR change, even if it's a beta API group.
Due to commercial support policies, this may result in the required support of many major versions of a product at a time.

Perhaps this is why many operators remain in a v0.x.x state - the benefit of being able to ship useful changes without hindrance of maintaining old MAJOR versions is appealing, however one must question if these projects
are actually gaining any benefit from SemVer at all in such a state.

Ultimately, if the operator uses a SemVer-like versioning system, it runs the risk of confusing users. If the operator bumped the default RabbitMQ image from 3.8.3 to 3.8.4, strict-SemVer would result in a MAJOR
version change of the operator, since a small number of users may experience a breaking change. However, it would be surprising for a user to look at the changelog and see the only difference was a PATCH version bump
in RabbitMQ, and a nightmare for developers to attempt to maintain many MAJOR branches every time a sometimes-breaking change came out. Would a user bump from operator version 1.2.3 to 2.0.0 based solely on the version number?
Probably not. Would they if they knew that the bump resulted from a PATCH change in RabbitMQ which had only a small chance of breaking them, having read the changelog? Probably! Representing such changes in purely
SemVer form carries too little context of the changes being made, especially where we have so many different types of dependency, each with their own versioning scheme.

An alternative is following non-strict SemVer, or in other words, using the three numbers as a rough human-interpretable representation of the extent of changes. This way, MAJOR version bumps are saved for when the change feels
to the developers significant enough to warrant maintaining a pre-bump MAJOR branch. While this runs the risk of misleading some users affected by breaking changes in MINOR or PATCH bumps, it would be 'familiar' to users, potentially
increasing adoption, and may lead to users more frequently updating due to fewer MAJOR version bumps (for better or worse).

### FerVer

[Fear Driven Versioning](https://github.com/jaredly/ferver) is more a representation of observed practices than a theoretical framework for versioning. FerVer is suitable for projects where the focus is on breaking changes only: MAJOR
and MINOR version bumps are both reserved for breaking changes, where the demarcation between the two is judged by some measurement of severity; a breaking change affecting 5% of users would likely be a MINOR version. FerVer favours
bumping MINOR versions liberally, and reserves the 0.x.x versioning for [development channels](https://github.com/jaredly/ferver/blob/master/dev-channel.md).

[An interesting quirk](https://github.com/jaredly/ferver/blame/master/dev-channel.md#L13-L16) is that all versions x.y.z where x > 0 must correlate to a development version 0.y.z. For example, 1.15.2 corresponds to the development version 0.15.2. Thus, you develop on the 0.y.z branch and release your development version as x.y.z when it's "stable".

FerVer appears more human-friendly than SemVer - the question of breakingness becomes less binary and more interpretive. Theoretically, it returns the significance of the changelog, and encourages
humans to not automatically consume without investigating the consequences. It is less used, however - at time of writing I couldn't find anything that uses it in earnest! It's likely that a user
may see the 3-number version scheme and assume the existence of a SemVer-like contract of absolutely no breakingness from MINOR version bumps.

### CalVer

[Calendar Versioning](https://calver.org/) directs the significance of version numbers away from arbitrary numbers representing compatibility, and towards real dates. The canonical example (har-har) of this versioning system is Ubuntu,
which has used CalVer since its release.

CalVer leaves more to the owning repo to decide upon the contract of versioning. CalVer provides a [composible scheme](https://calver.org/overview.html#scheme) of time-based elements to include in version numbers. Some repos use purely
time-based components, where others use a hybrid of time and traditional MAJOR/MINOR/MICRO numbers. CalVer is not a one-size-fits-all scheme, and can be designed around the needs of the repo; this is especially useful for
software with commercial support policies - it becomes trivial to determine whether a piece of software is still in commercial support.

CalVer also gets around some of the version paralysis that SemVer-like schemes suffer; [when CockroachDB made the switch](https://www.cockroachlabs.com/blog/calendar-versioning/), they saw less frustration with internal debates over
what qualified as a MAJOR version bump, and could set expectations of release schedules more easily. Users naturally perceive a MAJOR version bump as more significant than a MINOR bump;
where CockroachDB were releasing large new features without breaking API changes, CalVer allowed for the same excitement as would have been reserved for a MAJOR SemVer version bump.

On the other hand, some configurations of CalVer may be incompatible with opinionated tooling which assumes the presence of a three-digit version number, as in SemVer. There may be less support for CalVer versioning systems in
third-party tools such as [release notes & versioning](https://github.com/release-drafter/release-drafter) helpers.

There is also sometimes confusion from developers interacting with a product following CalVer. Since versioning has become synonymous with SemVer and SemVer-like versioning,
[there is sometimes pushback](https://github.com/saltstack/salt/issues/15406) where the commonly-held practice is not followed.

The power and drawback of CalVer is its modular nature - projects following the scheme can design their versioning as suits them, however they must be accountable to explaining the scheme,
through a VERSIONING.md, or something of similar ilk.

## Proposal - versioning

For the operator, attempting to represent the extent of a change in absence of the changelog is difficult. There is a lot of information a user would want to know about a change, including:

- The range of supported versions of:
  - RabbitMQ
  - Kubernetes
  - API versions of the RabbitmqCluster resource
- The storage version of the RabbitmqCluster resource
- The default version of RabbitMQ used in pods
- Fixes to the operator code or Dockerfile

Any of these might involve a breaking change (with the exception of the storage version, as the API Server is required to be able to convert between versions), regardless of whether the subcomponents themselves
are bumping a MAJOR, MINOR or PATCH/MICRO version number. For any given operator upgrade, we would expect users to consult the changelog in order to investigate the changes and the impact it would have on
their system.

My recommendation is that the project begins to follow a hybrid CalVer versioning scheme:

```
YYYY.MINOR.MICRO
```

This matches the versioning scheme of Pip, JetBrains PyCharm, Unity & Spring Cloud.

The year date component exists for supportability purposes: assuming a commercial support schedule of 18 months, a user may look at a version number and it is either from the current year,
from last year (and they should probably update) or from the year before that (and they should definitely update). Realistically one could include the months in this version, but whether that
would encourage more adoption of later versions is dubious.

As for the difference between MINOR and MICRO, I propose that the only change that can be a MICRO change are:

- Bugfixes to the operator or its Docker image
- Bumping the default RabbitMQ image to a new patch version (see [Proposal - releasing](#proposal---releasing)

Anything else, breaking or otherwise, feature or otherwise, is a MINOR version change. This way users can safely consume MICRO changes, while remaining on the latest RabbitMQ image, without worry
(Once upgrade is handled safely).

### Alternatives

One alternative to the proposed CalVer scheme is:

```
(YY)YY.0M.MICRO
```

This matches the versioning scheme of Ubuntu & Slack for Mobile.

This scheme focuses more on the release date than the caliber of the release. Here, there is no differentiation between MINOR and MICRO changes, either through significance or breakingness.
It may be preferable if we decide that the operator is unlikely to require backports of functionality (since the operator is already required to maintain multiple API versions by the
Kubernetes Deprecation policy). It also encourages users to check the release notes for every release, though may lead to some users being confused by potentially breaking changes in a version bump that
only consists of the last number bumping.

## Proposal - releasing

### Compatibility

Each release will have an implicit association with each of the following sections. It is assumed that there will be sufficient automated checking of each of these compatibilities through smoke tests in CI to warrant 'supporting' them.

#### Supported range of RabbitMQ versions

Each release of the operator will have a minimum & maximum version of RabbitMQ that is supported for that release. This shall be determined thus:

**Maximum**: By default, the latest RabbitMQ version that is GA at time of release of the operator release.
**Minimum**: By default, the earliest RabbitMQ version that is GA which is needed for minimum functionality of the operator (at time of writing, 3.8.0).

These are subject to change with each release; for example, if a future operator release required a minimum of RabbitMQ 3.8.9, the Minimum RabbitMQ version would change. Such a change in Minimum version would not be performed in a MICRO version change. The Maximum RabbitMQ version may change in a MICRO release, provided the newly-supported RabbitMQ release is not a new minor version of RabbitMQ.

Each release does not lock a user into a specific version of RabbitMQ - while the default is set per-release, they are free to change the deployment manifest to a supported version for that release. As mentioned before, if this default changes in a release of the operator, it will be a MICRO version change if it is a patch release of RabbitMQ, or MINOR otherwise.

I am proposing that we do not reject manifest changes to versions of RabbitMQ that lie outside the supported range for the release, for testability purposes.

From user feedback, we saw an expectation that for new releases of RabbitMQ, there was an expectation that the operator would need to upgrade first so that it knew how to handle the new RabbitMQ version in the cluster. This expectation is held with the above described pattern; users would upgrade their operator to support a new Maximum RabbitMQ version, and then upgrade their RabbitMQ nodes.

In the future, we may decide to test future RabbitMQ release candidates with older versions of the operator. This would allow us to backport support for new RabbitMQ versions to older versions of the operator, meaning a user would not have to upgrade their operator to support a new version of RabbitMQ. This is likely a future goal and not the first step to make now.

#### Supported range of Kubernetes server versions

Each release will support a Minimum and Maximum Kubernetes server version. At time of writing, that is 1.15-1.17. This should usually just match the range of GA versions of Kubernetes server available at time of release of the operator version. There may be times where a version does not support a GA version of Kubernetes, such as right now with 1.18. Cases like these should be the exception, and should be called out explicitly in the release notes of the operator.

#### API Versions

Each release supports a range of API versions for the resources managed by the operator. This support aligns with the [Kubernetes Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/).

For the case of Rule #4b, a 'release' is counted as a MINOR version release of the operator, and not a MICRO version release. This ensures that an API version can only be deprecated or removed in a MINOR release.

### GitHub Release

Each release of the operator generates a GitHub release for the new version. The RabbitMQ Core team may know some good tooling for generating these releases through CI.

#### Release notes

The release notes should be seen as the go-to place to decide whether and how to consume a new operator release. The release notes should maintain a table detailing the range of supported RabbitMQ, Kubernetes & CR API versions for each release, and explicitly call out where a new release drops or deprecates support for one of these.

Each release should also include the Preferred / Storage version of the API Group that the operator manages, and announce deprecation of older API versions.

#### Artefacts

For now, I propose the only artefact we publish per-version is the output of kustomize commands used to generate the operator manifest (see [this issue](https://github.com/rabbitmq/cluster-operator/issues/228#issuecomment-668506070)).

This would allow a user to install the operator into their cluster with just one kubectl command.

## User Stories

The following are some draft user stories for common operations or upgrade considerations.

### Story 1 - Consuming the latest operator release

As a user who wants to install the latest version of the RabbitMQ Cluster Operator, I can read the README.md of the repo and see an instruction to run on my cluster:
```
kubectl create -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/rabbitmq-cluster-operator.yaml
```

When I run this command, I can see that the RabbitMQ Cluster Operator is installed on my cluster, using the version of the operator Docker image corresponding to the latest release, and I can create a RabbitmqCluster resource on my cluster.

### Story 2 - Consuming a specific operator release

As a user who wants to install a specific version of the RabbitMQ Cluster Operator, I can read the README.md of the repo and see an instruction to run on my cluster:
```
kubectl create -f https://github.com/rabbitmq/cluster-operator/releases/download/$RMQ_OPERATOR_VERSION/rabbitmq-cluster-operator.yaml
```

When I run this command, I can see that the RabbitMQ Cluster Operator is installed on my cluster, using the version of the operator Docker image corresponding to the specific release, and I can create a RabbitmqCluster resource on my cluster.

### Story 3 - Consuming the latest commit of the operator

As a user who wants to install the latest commit of the RabbitMQ Cluster Operator, I can read the README.md of the repo and see an instruction to run on my cluster:
```
kustomize build github.com/rabbitmq/cluster-operator//config/namespace/base | kubectl apply -f -
kustomize build github.com/rabbitmq/cluster-operator//config/rbac/ | k apply -f -
kustomize build github.com/rabbitmq/cluster-operator//config/crd/ | k apply -f -
kustomize build github.com/rabbitmq/cluster-operator//config/default/base/ | k apply -f - # Note this command doesn't at time of writing work
```
(or better, a single command to run all of these)

When I run this command, I can see that the RabbitMQ Cluster Operator is installed on my cluster, using the latest tag of the operator Docker image, and I can create a RabbitmqCluster resource on my cluster.

### Story 4 - New patch release of RabbitMQ

As a user of RabbitMQ on Kubernetes, I see there is a new patch version of RabbitMQ available through RabbitMQ's website.
When I go to the Cluster Operator repo, I see a new MICRO release of the operator.
I can consume this by running in my cluster:
```
kubectl replace -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/rabbitmq-cluster-operator.yaml
```
or by changing `spec.template.spec.image` to the tag corresponding to the new version of the operator in the operator deployment manifest.
I can see that pods are recreated with the new patch versioned image of RabbitMQ.

### Story 5 - New minor release of RabbitMQ

As a user of RabbitMQ on Kubernetes, I see there is a new minor version of RabbitMQ available through RabbitMQ's website with some features I would like to consume.
When I go to the Cluster Operator repo, I see a new MINOR release of the operator, and in the release notes I see I can use the new RabbitMQ feature with the operator by providing additional config.

I can consume the new operator version by running in my cluster:
```
kubectl replace -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/rabbitmq-cluster-operator.yaml
```
or by changing `spec.template.spec.image` to the tag corresponding to the new version of the operator in the operator deployment manifest.
I can see that pods are recreated with the new minor versioned image of RabbitMQ.

### Story 6 - Release of API Group rabbitmq.com/v2

As a user of the Cluster Operator, I see from looking at the GitHub release notes that there is a new MINOR release of the operator.
I can see from the release notes that there is a new API Group rabbitmq.com/v2, and that rabbitmq.com/v1 is now considered deprecated.
I can see that the preferred / storage version of this new release is still v1.
I can see that there is action required upon me to migrate to the new version of the API, and that I have 12 months to make the transtition before the API is removed.

At the time of the following MINOR release of the operator, I can see that the preferred / storage version is now v2.

## Accepted Proposal - use non-strict SemVer

We discussed the above proposal as a team and have decided to **use non-strict SemVer for the Operator**. We decided on this for the following reasons - 
1. Tanzu RabbitMQ - a commercial bundle of RabbitMQ products that includes the Kubernetes Operator, will be using a form of CalVer. We felt that a CalVer product wrapped in a CalVer bundle would be confusing. For example, Tanzu RabbitMQ version 2020.10.2 packaging Operator version 2019.4.5 can be confusing for two reasons - 1. The versioning scheme is similar but may not be the same (the middle number could represent the minor version in one case, and the month number in another!), and 2. the dates are different and may raise questions around support. We could synchonise on both the CalVer scheme and on the dates being the same but would not like to be coupled so tightly with commercial versioning.
2. SemVer is widely used in the industry already, meaning our users will already come in with some experience and understanding.
3. We decided to follow non-strict SemVer since we would like to avoid maintaining too many major versions at the same time. Strict SemVer dictates that we bump major versions for each breaking change. We may however be committed to supporting each major version for a certain number of months, and supporting many versions at the same time will increase both our workload and it's complexity. We will define the non-strict in [another issue in this repository](https://github.com/rabbitmq/cluster-operator/issues/265).

## Accepted Proposal - releasing

In the same team meeting as above, we have agreed to the following with regards to releasing:
1. We should bump the major version to 1 when we GA. 
2. We cut new releases regularly, without a fixed timelime. We may use just the minor version for now until we GA (0.31.0, 0.32.0, ...). This will make the support experience for any early users easier since they will just have to mention the version number to us (than a commit hash for example).
3. We should define the guidelines for our "non-strict" SemVer and make this available as a README in this repository. This is addressed in the [following issue](https://github.com/rabbitmq/cluster-operator/issues/265).

## Implementation History

- [x] 2020-08-07: Open proposal PR
- [x] 2020-08-21: Add accepted proposal to PR
- [x] 2020-09-24: Release frequently, instead of every commit
