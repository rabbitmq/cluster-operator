# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum


# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

FROM ubuntu:latest

ARG GIT_COMMIT
LABEL GitCommit=$GIT_COMMIT

WORKDIR /
COPY --from=builder /workspace/manager .

# Create operator system user & group
RUN set -eux; \
	groupadd --gid 1000 --system rabbitmq-cluster-operator; \
	useradd --uid 1000 --system --gid rabbitmq-cluster-operator rabbitmq-cluster-operator

USER 1000:1000

ENTRYPOINT ["/manager"]
