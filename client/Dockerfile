# syntax = docker/dockerfile:1

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

FROM scratch AS runtime
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV USER=root

FROM runtime AS master
COPY --from=builder /out/gorse-master /usr/bin/gorse-master
ENTRYPOINT ["/usr/bin/gorse-master", "-c", "/etc/gorse/config.toml"]

FROM runtime AS server
COPY --from=builder /out/gorse-server /usr/bin/gorse-server
ENTRYPOINT ["/usr/bin/gorse-server"]

FROM runtime AS worker
COPY --from=builder /out/gorse-worker /usr/bin/gorse-worker
ENTRYPOINT ["/usr/bin/gorse-worker"]
