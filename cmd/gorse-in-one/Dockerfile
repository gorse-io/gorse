############################
# STEP 1 build executable binary
############################
FROM golang:1.24

COPY . gorse

RUN cd gorse/cmd/gorse-in-one && \
    CGO_ENABLED=0 go build -ldflags=" \
    -X 'github.com/gorse-io/gorse/cmd/version.Version=$(git describe --tags $(git rev-parse HEAD))' \
    -X 'github.com/gorse-io/gorse/cmd/version.GitCommit=$(git rev-parse HEAD)' \
    -X 'github.com/gorse-io/gorse/cmd/version.BuildTime=$(date)'" . && \
    mv gorse-in-one /usr/bin

RUN /usr/bin/gorse-in-one --version

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=0 /usr/bin/gorse-in-one /usr/bin/gorse-in-one

ENV USER root

ENTRYPOINT ["/usr/bin/gorse-in-one", "-c", "/etc/gorse/config.toml"]
