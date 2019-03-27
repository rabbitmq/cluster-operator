FROM ubuntu:18.04

ARG CLOUD_SDK_VERSION=240.0.0

RUN apt-get update \
    && apt-get install -y apt-transport-https curl gnupg \
    && curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
    && echo "deb https://packages.cloud.google.com/apt cloud-sdk-xenial main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y google-cloud-sdk=${CLOUD_SDK_VERSION}-0 kubectl make

