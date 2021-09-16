FROM golang:1.17 as builder

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...

ENV CGO_ENABLED=0
RUN go build -o /go/bin/app

FROM scratch
COPY --from=builder /go/bin/app /rabbitmq-admin-password-updater
ENTRYPOINT ["/rabbitmq-admin-password-updater"]
