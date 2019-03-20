FROM scratch
WORKDIR /
COPY tmp/manager .
COPY templates/ templates/
ENTRYPOINT ["/manager"]
