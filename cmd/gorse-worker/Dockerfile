# syntax = docker/dockerfile:1

############################
# STEP 1 build executable binary
############################
FROM --platform=$BUILDPLATFORM golang:1.27

WORKDIR /src

COPY go.* ./

RUN go mod download

COPY . ./

ARG TARGETOS

ARG TARGETARCH

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build ./...

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

RUN cd cmd/gorse-worker && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags=" \
    -X 'github.com/gorse-io/gorse/cmd/version.Version=${VERSION}' \
    -X 'github.com/gorse-io/gorse/cmd/version.GitCommit=${GIT_COMMIT}' \
    -X 'github.com/gorse-io/gorse/cmd/version.BuildTime=${BUILD_TIME}' \
    -X 'github.com/gorse-io/gorse/config.RootDir=/var/lib/gorse'" \
    .

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=0 /src/cmd/gorse-worker/gorse-worker /usr/bin/gorse-worker

ENV USER=root

ENTRYPOINT ["/usr/bin/gorse-worker"]
