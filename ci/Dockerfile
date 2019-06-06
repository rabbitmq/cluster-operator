FROM ubuntu:18.04

ARG CLOUD_SDK_VERSION=240.0.0
ARG GO_VERSION=1.12.5
ARG KAPP_VERSION="v0.7.0"

RUN apt-get update \
  && apt-get install -y curl gnupg \
  # ubuntu latest go is 1.10.x, downloading a newer version. Can remove after ubuntu updates the package using `apt-get install golang-go`
  && curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && curl -o go.tar.gz https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz \
  && curl -o /usr/local/bin/kustomize -sL https://go.kubebuilder.io/kustomize/linux/amd64 \
  && chmod +x /usr/local/bin/kustomize \
  && tar -xvf go.tar.gz -C /usr/local \
  && echo "deb https://packages.cloud.google.com/apt cloud-sdk-xenial main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
  && apt-get update \
  && apt-get install -y google-cloud-sdk=${CLOUD_SDK_VERSION}-0 kubectl make jq gcc git wget

RUN curl -L -o /usr/local/bin/kapp https://github.com/k14s/kapp/releases/download/${KAPP_VERSION}/kapp-linux-amd64 \
  && chmod +x /usr/local/bin/kapp

ENTRYPOINT /bin/bash
