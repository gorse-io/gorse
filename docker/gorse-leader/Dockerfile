FROM golang:1.15

ENV GOPROXY=https://goproxy.io,direct

RUN mkdir -p $GOPATH/src/github.com/zhenghaoz && \
    cd $GOPATH/src/github.com/zhenghaoz && \
    git clone https://github.com/zhenghaoz/gorse.git && \
    cd gorse && \
    git checkout release-0.2 && \
    cd cmd/gorse-leader && \
    go build . && \
    mv gorse-leader $GOPATH/bin/

CMD $GOPATH/bin/gorse-leader -c /etc/leader.toml
