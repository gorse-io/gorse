############################
# STEP 1 build executable binary
############################
FROM golang:1.20

WORKDIR /src

COPY go.* .

RUN go mod download

COPY . .

RUN cd gorse/cmd/gorse-worker && \
    CGO_ENABLED=0 go build -ldflags=" \
       -X 'github.com/zhenghaoz/gorse/cmd/version.Version=$(git describe --tags $(git rev-parse HEAD))' \
       -X 'github.com/zhenghaoz/gorse/cmd/version.GitCommit=$(git rev-parse HEAD)' \
       -X 'github.com/zhenghaoz/gorse/cmd/version.BuildTime=$(date)'" .

RUN /src/gorse/cmd/gorse-worker/gorse-worker --version

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=0 /src/gorse/cmd/gorse-worker/gorse-worker /usr/bin/gorse-worker

ENV USER root

ENTRYPOINT ["/usr/bin/gorse-worker"]
