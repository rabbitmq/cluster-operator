FROM ubuntu:18.04

ARG CLOUD_SDK_VERSION=240.0.0

RUN apt-get update \
    && apt-get install -y software-properties-common curl \
    && add-apt-repository ppa:longsleep/golang-backports \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable" \
    && apt-get install -y apt-transport-https curl gnupg \
    && curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
    && echo "deb https://packages.cloud.google.com/apt cloud-sdk-xenial main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y google-cloud-sdk=${CLOUD_SDK_VERSION}-0 kubectl make jq git golang-go wget \
    && apt-get install -y docker-ce

RUN wget -O /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/0.2.1/kind-linux-amd64 && chmod 755 /usr/local/bin/kind
