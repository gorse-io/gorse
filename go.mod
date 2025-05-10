module github.com/zhenghaoz/gorse

go 1.24

require (
	github.com/XSAM/otelsql v0.35.0
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/benhoyt/goawk v1.20.0
	github.com/bits-and-blooms/bitset v1.2.1
	github.com/cenkalti/backoff/v5 v5.0.2
	github.com/chewxy/math32 v1.11.1
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/deckarep/golang-set/v2 v2.3.1
	github.com/emicklei/go-restful-openapi/v2 v2.9.0
	github.com/emicklei/go-restful/v3 v3.9.0
	github.com/expr-lang/expr v1.16.9
	github.com/fxtlabs/primes v0.0.0-20150821004651-dad82d10a449
	github.com/go-playground/locales v0.14.1
	github.com/go-playground/universal-translator v0.18.1
	github.com/go-playground/validator/v10 v10.22.1
	github.com/go-resty/resty/v2 v2.16.3
	github.com/go-sql-driver/mysql v1.6.0
	github.com/go-viper/mapstructure/v2 v2.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorse-io/dashboard v0.0.0-20250510125937-7845dd126256
	github.com/gorse-io/gorse-go v0.5.0-alpha.1
	github.com/haxii/go-swagger-ui v0.0.0-20210203093335-a63a6bbde946
	github.com/jaswdr/faker v1.16.0
	github.com/jellydator/ttlcache/v3 v3.3.0
	github.com/json-iterator/go v1.1.12
	github.com/juju/errors v1.0.0
	github.com/juju/ratelimit v1.0.2
	github.com/klauspost/cpuid/v2 v2.2.3
	github.com/lafikl/consistent v0.0.0-20220512074542-bdd3606bfc3e
	github.com/lib/pq v1.10.6
	github.com/madflojo/testcerts v1.3.0
	github.com/mailru/go-clickhouse/v2 v2.0.1-0.20221121001540-b259988ad8e5
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/nikolalohinski/gonja/v2 v2.3.3
	github.com/orcaman/concurrent-map v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.13.0
	github.com/rakyll/statik v0.1.7
	github.com/redis/go-redis/extra/redisotel/v9 v9.5.3
	github.com/redis/go-redis/v9 v9.7.0
	github.com/samber/lo v1.38.1
	github.com/sashabaranov/go-openai v1.36.1
	github.com/schollz/progressbar/v3 v3.17.1
	github.com/sclevine/yj v0.0.0-20210612025309-737bdf40a5d1
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.19.0
	github.com/steinfletcher/apitest v1.5.17
	github.com/stretchr/testify v1.10.0
	github.com/thoas/go-funk v0.9.2
	github.com/tiktoken-go/tokenizer v0.5.1
	github.com/yuin/goldmark v1.7.8
	go.mongodb.org/mongo-driver v1.16.1
	go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful v0.36.4
	go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo v0.55.0
	go.opentelemetry.io/otel v1.31.0
	go.opentelemetry.io/otel/exporters/jaeger v1.11.1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.31.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.31.0
	go.opentelemetry.io/otel/exporters/zipkin v1.11.1
	go.opentelemetry.io/otel/sdk v1.31.0
	go.opentelemetry.io/otel/trace v1.31.0
	go.uber.org/atomic v1.10.0
	go.uber.org/zap v1.24.0
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/oauth2 v0.22.0
	google.golang.org/grpc v1.67.1
	google.golang.org/grpc/security/advancedtls v1.0.0
	google.golang.org/protobuf v1.35.1
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/clickhouse v0.4.2
	gorm.io/driver/mysql v1.3.4
	gorm.io/driver/postgres v1.3.5
	gorm.io/driver/sqlite v1.3.4
	gorm.io/gorm v1.23.6
	modernc.org/mathutil v1.7.1
	modernc.org/sortutil v1.2.1
	modernc.org/sqlite v1.21.1
	modernc.org/strutil v1.2.1
	moul.io/zapgorm2 v1.1.3
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-jose/go-jose/v4 v4.0.2 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/spec v0.20.7 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.12.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.11.0 // indirect
	github.com/jackc/pgx/v4 v4.16.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/redis/go-redis/extra/rediscmd/v9 v9.5.3 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241007155032-5fefd90f89a9 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/uint128 v1.2.0 // indirect
	modernc.org/cc/v3 v3.40.0 // indirect
	modernc.org/ccgo/v3 v3.16.13 // indirect
	modernc.org/libc v1.22.3 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/opt v0.1.3 // indirect
	modernc.org/token v1.0.1 // indirect
)

replace (
	gorgonia.org/gorgonia v0.9.18-0.20230327110624-d1c17944ed22 => github.com/gorse-io/gorgonia v0.0.0-20230817132253-6dd1dbf95849
	gorgonia.org/tensor v0.9.23 => github.com/gorse-io/tensor v0.0.0-20230617102451-4c006ddc5162
	gorm.io/driver/clickhouse v0.4.2 => github.com/gorse-io/clickhouse v0.3.3-0.20220715124633-688011a495bb
	gorm.io/driver/sqlite v1.3.4 => github.com/gorse-io/sqlite v1.3.3-0.20220713123255-c322aec4e59e
)
