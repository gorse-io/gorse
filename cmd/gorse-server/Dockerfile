# syntax = docker/dockerfile:1

############################
# STEP 1 build executable binary
############################
FROM --platform=$BUILDPLATFORM golang:1.25

WORKDIR /src

COPY go.* ./

RUN go mod download

COPY . ./

ARG TARGETOS

ARG TARGETARCH

RUN --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/gorse-server && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags=" \
    -X 'github.com/gorse-io/gorse/cmd/version.Version=$(git describe --tags $(git rev-parse HEAD))' \
    -X 'github.com/gorse-io/gorse/cmd/version.GitCommit=$(git rev-parse HEAD)' \
    -X 'github.com/gorse-io/gorse/cmd/version.BuildTime=$(date)' \
    -X 'github.com/gorse-io/gorse/config.RootDir=/var/lib/gorse'" .

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=0 /src/cmd/gorse-server/gorse-server /usr/bin/gorse-server

ENV USER=root

ENTRYPOINT ["/usr/bin/gorse-server"]
