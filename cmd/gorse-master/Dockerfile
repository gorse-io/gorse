# syntax = docker/dockerfile:1

############################
# STEP 1 build executable binary
############################
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder

WORKDIR /src

COPY go.* ./

RUN go mod download

COPY . ./

ARG TARGETOS

ARG TARGETARCH

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /out/ -ldflags=" \
    -X 'github.com/gorse-io/gorse/cmd/version.Version=${VERSION:-$(git describe --tags $(git rev-parse HEAD))}' \
    -X 'github.com/gorse-io/gorse/cmd/version.GitCommit=${GIT_COMMIT:-$(git rev-parse HEAD)}' \
    -X 'github.com/gorse-io/gorse/cmd/version.BuildTime=${BUILD_TIME:-$(date)}' \
    -X 'github.com/gorse-io/gorse/config.RootDir=/var/lib/gorse'" \
    ./cmd/gorse-master ./cmd/gorse-server ./cmd/gorse-worker

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /out/gorse-master /usr/bin/gorse-master

ENV USER=root

ENTRYPOINT ["/usr/bin/gorse-master", "-c", "/etc/gorse/config.toml"]
