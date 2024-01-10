# syntax = docker/dockerfile:1-experimental

############################
# STEP 1 build executable binary
############################
FROM golang:1.21 as build

WORKDIR /src

COPY go.* ./

RUN go mod download

COPY . ./

RUN --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/gorse-worker && \
    CGO_ENABLED=0 go build -ldflags=" \
       -X 'github.com/Neura-Studios/gorse/cmd/version.Version=$(git describe --tags $(git rev-parse HEAD))' \
       -X 'github.com/Neura-Studios/gorse/cmd/version.GitCommit=$(git rev-parse HEAD)' \
       -X 'github.com/Neura-Studios/gorse/cmd/version.BuildTime=$(date)'" .

RUN /src/cmd/gorse-worker/gorse-worker --version

############################
# STEP 2 build a small image
############################
FROM alpine

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /src/cmd/gorse-worker /usr/bin
RUN /usr/bin/gorse-worker --version

ENV USER root

ENTRYPOINT ["/usr/bin/gorse-worker"]
