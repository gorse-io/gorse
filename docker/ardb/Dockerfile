FROM ubuntu:18.04

RUN apt-get update && apt-get install --no-install-recommends -y \
    git build-essential ca-certificates wget

RUN git clone https://github.com/yinqiwen/ardb.git && \
    cd ardb && \
    git checkout tags/v0.9.7 && \
    storage_engine=rocksdb make && \
    cp src/ardb-server /usr/bin

RUN mkdir -p /etc/ardb && \
    cp ardb/ardb.conf /etc/ardb && \
    sed -e 's@home.*@home /var/lib/ardb@' -e 's/loglevel.*/loglevel info/' -i /etc/ardb/ardb.conf && \
    sed -i 's/16379/6379/g' /etc/ardb/ardb.conf && \
    echo 'trusted-ip *.*.*.*' >> /etc/ardb/ardb.conf

EXPOSE 6379

CMD /usr/bin/ardb-server /etc/ardb/ardb.conf
