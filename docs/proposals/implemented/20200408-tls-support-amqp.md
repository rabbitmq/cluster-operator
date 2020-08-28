---
title:TLS support - AMQP
authors:
  - "@jrmcneil"
reviewers:
  -
creation-date: 2020-05-08
status: implemented
---

# TLS Support - AMQP

## Table of Contents

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
  - [Implementation History](#implementation-history)
  - [Drawbacks [optional]](#drawbacks-optional)
  - [Alternatives [optional]](#alternatives-optional)
  - [Infrastructure Needed [optional]](#infrastructure-needed-optional)

## Summary
RabbitMQ has [inbuilt support for TLS](https://www.rabbitmq.com/ssl.html). This enhancement considers how best to configure this feature on an operator-deployed RabbitMQ cluster:
- Outline the steps which need to be taken to allow a TLS-enabled AMPQ connection to an operator managed RabbitMQ broker
- Assess the feasibility of integrating TLS utilities (e.g. a certificate manager) into the core operator offering

## Motivation

As a RabbitMQ client (whether application or end user), I want to be sure that the broker I am contacting can be trusted and that our communicaiton will be encrypted. TLS support is a bare-minimum requirement for many, if not all, production RabbitMQ use-cases. Adding TLS support to the RabbitMQ for Kubernetes operator will contribute to making it a secure, production-ready solution.

### Goals

- Write/Read a TLS 1.2 encrypted AMQP 0-9-1 message from an operator deployed RabbitMQ broker (standalone and Tanzu Service Manager deployments)
- Survey other TLS implementations in K8s operators for common patterns. Priority should be given to operators in the VMware portfolio
- Document our standard approach to configuring TLS via the RabbitMQ Custom Resource
- Document options for certificate management

### Non-Goals/Future Work

- Configuring or managing a CA. We will assume for the sake of any work with cert-manager that an appropriate [Issuer](https://cert-manager.io/docs/concepts/issuer/) exists in the cluster
- Certificate rotation e.g. Pod restart vs. RabbitMQ restart on Secret change
- Configuring non-default ports for SSL connections. See the [list of default RabbitMQ ports](https://www.rabbitmq.com/networking.html#ports)
- Implementation details of TLS for other messaging protocols (e.g. MQTT, STOMP, AMQP 1.0) which have yet to be themselves implemented in the Custom Resource
- [client-server mutual TLS/peer verification](https://www.rabbitmq.com/ssl.html#peer-verification)
- [inter-node and CLI TLS](https://www.rabbitmq.com/clustering-ssl.html)
- Accessing the management dashboard/API over HTTPS. Management TLS is configured via a seperate cert/keyfile. We have evidence from a [user implementation](https://groups.google.com/forum/?utm_medium=email&utm_source=footer#!msg/rabbitmq-users/YnqEbaFNv-c/dtX_YHsfAgAJ) that these two sets of certs won't necessarily be the same.

## Proposal

- When `tls.secretName` is set in the CR:
  - Block deployment of all RabbitMQCluster resources until the Secret can be retrieved
  - Mount the Secret as a volume on each RabbitMQ Pod
  - Set , `ssl_options.certfile`, `ssl_options.keyfile` in rabbitmq.conf to paths on the mount
  - Add `5671` to the Container Ports in the Pod Template
  - Add `5671` to the port map in the Client Service
- If we expose the Client Service template we can potentially depend on the user to specify the port
- When deploying via Tanzu Service Manager, a [Certificate Request](https://cert-manager.io/docs/concepts/certificaterequest/) is templated if the plan specified `tls: true`

### User Stories

#### Story 1
```
Given I have a RabbitMQ for Kubernetes operator deployed
And I deploy a RabbitMQ broker via a RabbitmqCluster Custom Resource
And I specify the location of a valid certfile and private key file in the Custom Resource
And my client trusts the CA used to sign the RabbitMQ broker's certificate
When I send an AMPQ message over TLS (default port 5671)
Then my message is succesfully stored on a queue in the RabbitMQ broker
And I can retrieve that message over the same port
```
#### Story 2
```
Given I have a Tanzu Service Manager environment
And a certificate manager is provisioned in the cluster with an appropriate CA
And I deploy the RabbitMQ operator
And I request a new RabbitMQCluster with TLS enabled
When I send an AMPQ message over TLS (default port 5671)
Then my message is succesfully stored on a queue in the RabbitMQ broker
And I can retrieve that message over the same port
```
### Implementation Details/Notes/Constraints
#### 'Turning on' TLS for a K8s deploy RabbitMQ cluster
- (server-side) TLS for RabbitMQ protocols is enabled based on a set of settings in rabbitmq.conf. Two filepaths are the bare mimimum required configuration: `ssl_options.certfile`, `ssl_options.keyfile`, and `tls.ssl.default` (set to `5671`).
- Our challenge is relating those settings to K8s resource configuration, and handling situations where they do not match.
- With the proposed solution, when `tls.secretName` is set, we will set the relevant config in rabbitmq.conf, add the volume mount in the Pod Template, and add the Service Port in the Client Service. There are several potential or planned enhancements that create edge cases worth considering:
  - Once we add the `additional_config` in the CR, the user could effectively disable TLS by setting their own values for `ssl_options`.
  - The user could also change `listeners.ssl.default` and the Service port would no longer be valid.
  - If we allow the Service to be templated the user could block the port neeeded for `listeners.ssl.default`
  - If we allow the StatefulSet to be templated the user could overwrite the required volume mount
- The benefit of adding a new field to name the TLS Secret is that we can wait for the Secret to be created if we are using a certificate manager. The operator creates the other Secrets that will be mounted as volumes before the StatefulSet so that Pod creation succeeds. But for a Certificate Secret the process is asynchronous and we will need to poll to be sure it exists (e.g. in cert-manager there are several intermediate CRs: Certificate -> CertificateRequest -> Challenge -> Secret).
- It may be enough to simply document that if `tls.secretName` is set there is no need to set `ssl_options.certfile`, `ssl_options.keyfile` or a Volume mount for the TLS Secret (and that doing so will lead to unexpected behaviour).

#### TLS Secret
- We should expect the certfiles and keyfile in a Kubernetes [TLS Secret](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls)
  - We can mount the Secret as a volume on each RabbitMQ node Pod without making cert-manager (or equivalent) an explicit dependency.
  - The RabbitMQ Pods will fail to deploy with an 'unable to mount volume' error unless the Secret created by the Certificate (issued at some point after deploying the CertificateRequest) is available. We should check for the existence of the named Secret in the CR and requeue with a delay until it appears.
  - One strategy could be to deploy a Secret with dummy data and update it once a certificate is ready:
    - The volume mount will be updated if the data in the Secret changes.
    - RabbitMQ can also update its certificates at runtime.
  - [cert-manager](https://github.com/jetstack/cert-manager) stores the key pair ([certfile + keyfile](https://github.com/jetstack/cert-manager/blob/15d1735688a907c3fde90e5801e86d0e1abbaefe/pkg/controller/certificates/sync.go#L851)) in a TLS Secret under the standard [core v1](https://github.com/kubernetes/api/blob/2433a9db3db38d5e177eac8495dec1cd3d15b128/core/v1/types.go#L5600) (map) keys: `tls.crt` and `tls.key`.
#### Disabling non-TLS
- Should we expose non-TLS ports when TLS is enabled? Would this be a blanket setting or per protocol? `listeners.tcp = none`, `mqtt.listeners.tcp = none` etc...

#### Tanzu Service Manager
- `plans` are too high level an abstraction to expect users to provide certificate details. We should consider how an operator would be configured and deployed with the ability to inject certificates for all the TLS-enabled RabbitMQ brokers.
- This proposal make cert-manager a dependency for Tanzu Service Manager deployed RabbitMQ for K8s. A plan with `tls: true` will deploy a cert-manager CertificateRequest with the RabbitMQCluster. The changes implemented at the operator will then ensure that the deployed RabbitMQCluster has the mounted certs.
  - However, cert-manager is expects cluster-wide privileges. cert-manager also requires [Issuers](https://cert-manager.io/docs/concepts/issuer/) to be configured before Certificates can be issued. Both of these tasks seem out of scope and more general than RabbitMQ operator config. We are therefore assuming that cert-manager configuration will either be part of a higher-level Tanzu cluster setup or at least done ahead of Rabbit deployment.
- bind.yaml needs to be configurable to enable ssl, specify the correct port and point to an https URI

### Peer verification/mutual TLS
- Although it is out of scope for this work, it is worth capturing some notes on how mutual TLS is configured
- In order to trust certificates provided by clients, the RabbitMQ broker needs an additional filepath in `rabbitmq.conf`: `ssl_options.cacertfile`. The `cacertfile` is the concatenated certificate chain which includes the CA that signed the certificates provided by the client.
- [cert-manager](https://github.com/jetstack/cert-manager) stores the ca bundle in the same Secret as the certfile and private key using a key [defined by cert-manager](https://github.com/jetstack/cert-manager/blob/ba46885ee43b031c0bad934def6b051b0c640eb0/pkg/internal/apis/meta/types.go#L60): `ca.crt`. By contrast, the rabbitmq-ha helm chart sets the ca cert in a seperate secret under [`ca.key`](https://github.com/helm/charts/blob/1b724a195f31be3e548574c8326071167eb8ea21/stable/pomerium/templates/tls-secrets.yaml#L113).
- The other requirement for multual TLS is set in rabbitmq.conf and won't require any extra work (`ssl_options.verify = verify_peer`)

### Risks and Mitigations

- The biggest risk is that we create a situation where conflicting configuration makes it confusing to debug actual state. For instance, setting the secret name to `foo` but the `ssl_options` to `/bar/ca.crt` etc.. and overwriting the TLS Secret volume mount to point to `/bar/`.
  - We could check resources before creation but that might be brittle. Particularly if we try to parse rabbitmq.conf settings vs. expectations.
  - It might be enough to clearly document this, i.e. "if you set `tls.secretName` don't set etc..."

## Alternatives

- Instead of configuring RabbitMQ to handle encrypted connections, we could implement a proxy for TLS termination. The proxy could be implemented as a sidecar on each Pod containing a RabbitMQ node.
- The proxy could live at the edge of a VPN containing the RabbitMQ cluster. However, we have no such network in mind for our operator use case and can't count on one existing.

## Additional Details

### Test Plan [optional]

// TODO

[testing-guidelines]: https://git.k8s.io/community/contributors/devel/sig-testing/testing.md

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY
