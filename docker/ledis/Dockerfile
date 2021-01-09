FROM golang:1.15

RUN apt-get update && apt-get install --no-install-recommends -y \
    git g++ libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev libbz2-dev libzstd-dev

# Install Rocksdb
RUN cd /tmp && git clone https://github.com/facebook/rocksdb.git && \
    cd rocksdb && \
    git checkout 5.1.fb && \
    make shared_lib && \
    mkdir -p /usr/local/rocksdb/lib && \
    mkdir /usr/local/rocksdb/include && \
    cp librocksdb.so* /usr/local/rocksdb/lib && \
    cp /usr/local/rocksdb/lib/librocksdb.so* /usr/lib/ && \
    cp -r include /usr/local/rocksdb/

RUN mkdir -p $GOPATH/src/github.com/siddontang && \
    cd $GOPATH/src/github.com/siddontang && \
    git clone https://github.com/ledisdb/ledisdb.git && \
    cd ledisdb && \
    git checkout tags/v0.6 && \
    bash dev.sh && make && \
    mv ./bin/ledis* $GOPATH/bin/

RUN cp $GOPATH/src/github.com/siddontang/etc/ledis.conf /etc/ledis.conf

EXPOSE 6380

CMD $GOPATH/bin/ledis-server -config=/etc/ledis.conf -db_name=rocksdb