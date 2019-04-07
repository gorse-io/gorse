###############
# Build Stage #
###############

FROM golang

ENV GO111MODULE=on

COPY .. /go/src/github.com/zhenghaoz/gorse

WORKDIR /go/src/github.com/zhenghaoz/gorse

##############
# Test Stage #
##############

FROM mysql

ENV MYSQL_USER=test_user

ENV MYSQL_PASSWORD=test_pass

ENV MYSQL_DATABASE=test_db

CMD ['bash', 'test_driver.sh', 'test_user', 'test_pass', 'test_db']