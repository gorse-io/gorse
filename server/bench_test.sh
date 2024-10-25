CACHE_ARG=redis
DATA_ARG=mysql

while [[ $# -gt 0 ]]; do
  case $1 in
    --cache)
      CACHE_ARG="$2"
      shift # past argument
      shift # past value
      ;;
    --data)
      DATA_ARG="$2"
      shift # past argument
      shift # past value
      ;;
    *)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

case $CACHE_ARG in
  redis)
    export BENCH_CACHE_STORE='redis://127.0.0.1:6379/'
    ;;
  mysql)
    export BENCH_CACHE_STORE='mysql://root:password@tcp(127.0.0.1:3306)/'
    ;;
  postgres)
    export BENCH_CACHE_STORE='postgres://gorse:gorse_pass@127.0.0.1/'
    ;;
  mongodb)
    export BENCH_CACHE_STORE='mongodb://root:password@127.0.0.1:27017/'
    ;;
  *)
    echo "Unknown database $1"
    exit 1
    ;;
esac

case $DATA_ARG in
  clickhouse)
    export BENCH_DATA_STORE='clickhouse://127.0.0.1:8123/'
    ;;
  mysql)
    export BENCH_DATA_STORE='mysql://root:password@tcp(127.0.0.1:3306)/'
    ;;
  postgres)
    export BENCH_DATA_STORE='postgres://gorse:gorse_pass@127.0.0.1/'
    ;;
  mongodb)
    export BENCH_DATA_STORE='mongodb://root:password@127.0.0.1:27017/'
    ;;
  *)
    echo "Unknown database $1"
    exit 1
    ;;
esac

echo cache: "$CACHE_ARG"
echo data: "$DATA_ARG"
go test -run Benchmark -bench .
