FROM scratch
WORKDIR /
COPY manager .
COPY templates/ templates/
ENTRYPOINT ["/manager"]
