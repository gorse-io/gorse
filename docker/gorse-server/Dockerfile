############################
# STEP 1 build executable binary
############################
FROM golang:1.17

COPY . gorse

RUN cd gorse && \
    go get -v -t -d ./...

RUN cd gorse/cmd/gorse-server && \
    CGO_ENABLED=0 go build -ldflags=" \
       -X 'github.com/zhenghaoz/gorse/cmd/version.Version=$(git describe --tags $(git rev-parse HEAD))' \
       -X 'github.com/zhenghaoz/gorse/cmd/version.GitCommit=$(git rev-parse HEAD)' \
       -X 'github.com/zhenghaoz/gorse/cmd/version.BuildTime=$(date)'" . && \
    mv gorse-server /usr/bin

RUN /usr/bin/gorse-server --version

############################
# STEP 2 build a small image
############################
FROM scratch

ENV USER root

COPY --from=0 /usr/bin/gorse-server /usr/bin/gorse-server

ENTRYPOINT ["/usr/bin/gorse-server"]
