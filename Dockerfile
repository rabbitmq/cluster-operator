ARG GO_TAG=1.25
ARG DOCKER_REGISTRY=docker.io
# Build the manager binary
FROM --platform=$BUILDPLATFORM ${DOCKER_REGISTRY}/library/golang:${GO_TAG} AS builder

WORKDIR /workspace

# Dependencies are cached unless we change go.mod or go.sum
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
COPY pkg/ pkg/

# Build
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

# FIPS
ARG FIPS_MODE=off
ENV GOFIPS140=$FIPS_MODE

RUN CGO_ENABLED=0 GO111MODULE=on go build -a -tags timetzdata -o manager main.go

# ---------------------------------------
ARG DOCKER_REGISTRY=docker.io
FROM ${DOCKER_REGISTRY}/library/alpine:latest AS etc-builder


RUN echo "rabbitmq-cluster-operator:x:1000:" > /etc/group && \
    echo "rabbitmq-cluster-operator:x:1000:1000::/home/rabbitmq-cluster-operator:/usr/sbin/nologin" > /etc/passwd

RUN apk add -U --no-cache ca-certificates

# ---------------------------------------
FROM scratch

ARG GIT_COMMIT
LABEL GitCommit=$GIT_COMMIT

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=etc-builder /etc/passwd /etc/group /etc/
COPY --from=etc-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

USER 1000:1000

ENTRYPOINT ["/manager"]
