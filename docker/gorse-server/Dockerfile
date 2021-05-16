############################
# STEP 1 build executable binary
############################
FROM golang:1.16

ENV GOPROXY=https://goproxy.io,direct

RUN git clone --depth 1 https://github.com/zhenghaoz/gorse.git && \
    cd gorse && \
    go get -v -t -d ./...

RUN cd gorse/cmd/gorse-server && \
    CGO_ENABLED=0 go build . && \
    mv gorse-server /usr/bin

############################
# STEP 2 build a small image
############################
FROM scratch

COPY --from=0 /usr/bin/gorse-server /usr/bin/gorse-server

ENTRYPOINT ["/usr/bin/gorse-server"]
