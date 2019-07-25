# Build the manager binary
FROM golang:1.12.5 as builder

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

# Changed base image to ubuntu for OSL compliance
FROM ubuntu:18.04
WORKDIR /
COPY --from=builder /workspace/manager .
ENTRYPOINT ["/manager"]
