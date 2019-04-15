FROM scratch
WORKDIR /
COPY tmp/servicebroker servicebroker
COPY config.yaml config.yaml
ENTRYPOINT ["/servicebroker", "-configPath", "config.yaml"]
