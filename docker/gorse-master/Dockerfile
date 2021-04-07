FROM golang:1.15

ENV GOPROXY=https://goproxy.io,direct

RUN mkdir -p $GOPATH/src/github.com/zhenghaoz && \
    cd $GOPATH/src/github.com/zhenghaoz && \
    git clone --depth 1 https://github.com/zhenghaoz/gorse.git && \
    cd gorse && \
    cd cmd/gorse-master && \
    go build . && \
    mv gorse-master $GOPATH/bin/

CMD $GOPATH/bin/gorse-master -c /etc/config.toml
