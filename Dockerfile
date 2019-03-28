FROM scratch
WORKDIR /
COPY tmp/operator .
COPY templates/ templates/
ENTRYPOINT ["/operator"]
