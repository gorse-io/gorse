FROM golang:1.15

ENV GOPROXY=https://goproxy.io,direct

RUN mkdir -p $GOPATH/src/github.com/zhenghaoz && \
    cd $GOPATH/src/github.com/zhenghaoz && \
    git clone https://github.com/zhenghaoz/gorse.git && \
    cd gorse && \
    cd cmd/gorse-worker && \
    go build . && \
    mv gorse-worker $GOPATH/bin/

CMD $GOPATH/bin/gorse-worker -c /etc/worker.toml
