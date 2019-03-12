FROM golang

ENV GO111MODULE=on

COPY . /go/src/github.com/zhenghaoz/gorse

WORKDIR /go/src/github.com/zhenghaoz/gorse

CMD ["go", "test", "-v", "./..."]
