# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the go source
COPY cmd/operator/main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum


# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Changed base image to ubuntu for OSL compliance
FROM ubuntu@sha256:c303f19cfe9ee92badbbbd7567bc1ca47789f79303ddcef56f77687d4744cd7a
WORKDIR /
COPY --from=builder /workspace/manager .
ARG COMMIT_SHA
LABEL commit=$COMMIT_SHA

# Create operator system user & group
RUN set -eux; \
	groupadd --gid 1000 --system p-rmq; \
	useradd --uid 1000 --system --gid p-rmq p-rmq

USER 1000:1000

ENTRYPOINT ["/manager"]
