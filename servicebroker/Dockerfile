FROM scratch
WORKDIR /
COPY servicebroker servicebroker
COPY config.yaml config.yaml
ENTRYPOINT ["/servicebroker", "-configPath", "config.yaml"]
