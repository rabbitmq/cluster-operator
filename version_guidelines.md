### RabbitMQ Cluster Kubernetes Operator Versioning Scheme
---
RabbitMQ Cluster Kubernetes Operator follows non-strict semver. More details about other versioning strategies that were considered can be found
in [the versioning proposal](https://github.com/rabbitmq/cluster-operator/blob/main/docs/proposals/20200806-versioning-release-strategy.md). This document explains the non-strict semver versioning scheme used by RabbitMQ Cluster Kubernetes Operator.

_**Operator** refers to *RabbitMQ Cluster Kubernetes Operator* in the document henceforth._

The key words “MUST”, “MUST NOT”, “REQUIRED”, “SHALL”, “SHALL NOT”, “SHOULD”, “SHOULD NOT”, “RECOMMENDED”, “MAY”, and “OPTIONAL” in 
this document are to be interpreted as described in [RFC 2119](https://tools.ietf.org/html/rfc2119).

:warning: **Please note that these are only guidelines which MAY NOT be always followed. Users MUST read the release notes to understand the changes and the potential impact of these changes.**

1. A normal version number MUST take the form X.Y.Z where X, Y, and Z are non-negative integers, and MUST NOT contain leading zeroes. 
X is the major version, Y is the minor version, and Z is the patch version. Each element MUST increase numerically. For instance: 1.9.0 -> 1.10.0 -> 1.11.0.

2. Major version zero (0.y.z) is for initial development. This is a Beta version and anything MAY change at anytime. The Operator SHOULD NOT be considered stable.

3. Version 1.0.0 defines the GA/stable version of Operator. The way in which the version number is incremented after this release is dependent 
on how the Operator changes.

4. Patch version Z (x.y.Z | x > 0) MUST be incremented if only backwards compatible bug fixes and/or CVE fixes are introduced. 
A bug fix is defined as an internal change that fixes incorrect behavior.

5. Minor version Y (x.Y.z | x > 0): 
- It MUST be incremented if new, functionality is introduced to the Operator. The new functionality MAY contain breaking changes.
- It MUST be incremented if any OPERATOR functionality is marked as deprecated. 
- It MUST be incremented if underlying Kubernetes server version and/or RabbitMQ version are marked as supported and/or deprecated. 
- It MAY contain breaking changes. Breaking changes MUST be documented in the release notes.
- It MAY include patch level changes. Patch version MUST be reset to 0 when minor version is incremented.

6. Major version X (X.y.z | X > 0):
- It MAY be incremented if any backwards incompatible changes are introduced to the public API. 
- It MAY also include minor and patch level changes. Patch and minor version MUST be reset to 0 when major version is incremented.
