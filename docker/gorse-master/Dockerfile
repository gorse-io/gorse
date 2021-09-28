############################
# STEP 1 build executable binary
############################
FROM golang:1.17

COPY . gorse

RUN cd gorse && \
    go get -v -t -d ./...

RUN cd gorse/cmd/gorse-master && \
    CGO_ENABLED=0 go build . && \
    mv gorse-master /usr/bin

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /usr/bin/gorse-master /usr/bin/gorse-master

ENV USER root

ENTRYPOINT ["/usr/bin/gorse-master", "-c", "/etc/config.toml"]
