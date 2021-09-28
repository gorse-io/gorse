############################
# STEP 1 build executable binary
############################
FROM golang:1.17

COPY . gorse

RUN cd gorse && \
    go get -v -t -d ./...

RUN cd gorse/cmd/gorse-worker && \
    CGO_ENABLED=0 go build . && \
    mv gorse-worker /usr/bin

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /usr/bin/gorse-worker /usr/bin/gorse-worker

ENV USER root

ENTRYPOINT ["/usr/bin/gorse-worker"]
