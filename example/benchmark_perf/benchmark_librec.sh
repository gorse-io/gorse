#!/usr/bin/env bash

LIBREC_ROOT=~/.gorse/librec-3.0.0

ml100k() {
    echo 'test performance on ml-100k'

    START_TIME=$SECONDS

    ${LIBREC_ROOT}/bin/librec rec -exec -D rec.recommender.class=biasedmf \
        -D dfs.data.dir=${LIBREC_ROOT}/data \
        -D data.input.path=movielens/ml-100k \
        -D data.column.format=UIRT \
        -D data.model.splitter=kcv \
        -D data.splitter.cv.number=5 \
        -D rec.factor.number=50 \
        -D rec.iterator.maximum=100 \
        -D rec.iterator.learnrate=0.01 \
        -D rec.iterator.learnrate.maximum=0.01 \
        -D rec.learnrate.decay=1.0 \
        -D rec.learningrate.bolddriver=false \
        -D rec.user.regularization=0.1 \
        -D rec.item.regularization=0.1 \
        -D rec.bias.regularization=0.1 \
        -libjars ${LIBREC_ROOT}/lib/log4j-1.2.17.jar

    ELAPSED_TIME=$(($SECONDS - $START_TIME))

    echo time=${ELAPSED_TIME}s
}

ml1m() {
    echo 'test performance on ml-1m'

    START_TIME=$SECONDS

    ${LIBREC_ROOT}/bin/librec rec -exec -D rec.recommender.class=biasedmf \
        -D dfs.data.dir=${LIBREC_ROOT}/data \
        -D data.input.path=movielens/ml-1m \
        -D data.column.format=UIRT \
        -D data.model.splitter=kcv \
        -D data.splitter.cv.number=5 \
        -D rec.factor.number=80 \
        -D rec.iterator.maximum=100 \
        -D rec.iterator.learnrate=0.005 \
        -D rec.iterator.learnrate.maximum=0.005 \
        -D rec.learnrate.decay=1.0 \
        -D rec.learningrate.bolddriver=false \
        -D rec.user.regularization=0.05 \
        -D rec.item.regularization=0.05 \
        -D rec.bias.regularization=0.05 \
        -libjars ${LIBREC_ROOT}/lib/log4j-1.2.17.jar

    ELAPSED_TIME=$(($SECONDS - $START_TIME))

    echo time=${ELAPSED_TIME}s
}

case $1 in
    ml-100k)
        ml100k
        ;;
    ml-1m)
        ml1m
        ;;
    *)
        # Display usage
        echo "usage: $0 [ml-100k|ml-1m]"
        exit 1
        ;;
esac
