module butterfly.orx.me/core

go 1.26.7

require (
	github.com/XSAM/otelsql v0.44.0
	github.com/aws/aws-sdk-go-v2/config v1.33.9
	github.com/aws/aws-sdk-go-v2/credentials v1.20.9
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.3
	github.com/aws/smithy-go/tracing/smithyoteltracing v1.0.33
	github.com/gin-gonic/gin v1.12.0
	github.com/go-sql-driver/mysql v1.10.1
	github.com/hashicorp/consul/api v1.34.5
	github.com/jackc/pgx/v5 v5.11.0
	github.com/prometheus/client_golang v1.25.0
	github.com/redis/go-redis/extra/redisotel/v9 v9.23.0
	github.com/redis/go-redis/v9 v9.23.0
	github.com/stretchr/testify v1.12.1
	github.com/testcontainers/testcontainers-go v0.44.0
	github.com/testcontainers/testcontainers-go/modules/mongodb v0.44.0
	github.com/testcontainers/testcontainers-go/modules/mysql v0.44.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.44.0
	github.com/testcontainers/testcontainers-go/modules/redis v0.44.0
	go.mongodb.org/mongo-driver/v2 v2.9.2
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.72.0
	go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo v0.0.0-20261016162345-d68bf0d1c4d3
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.opentelemetry.io/proto/otlp v1.11.1
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.2
	gorm.io/plugin/opentelemetry v0.1.16
)

require (
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/ClickHouse/ch-go v0.74.0 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.49.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/andybalholm/brotli v1.2.6 // indirect
	github.com/aws/aws-sdk-go-v2 v1.47.3 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.4 // indirect
	github.com/aws/smithy-go v1.28.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.4 // indirect
	github.com/bytedance/sonic/loader v0.5.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.15 // indirect
	github.com/gin-contrib/sse v1.1.2 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.8.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.2 // indirect
	github.com/go-playground/universal-translator v0.18.2 // indirect
	github.com/go-playground/validator/v10 v10.30.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/goccy/go-json v0.11.2 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.32.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.7.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/serf v0.11.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.1 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mdelapenya/tlscert v0.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.3.0 // indirect
	github.com/moby/moby/api v1.56.1 // indirect
	github.com/moby/moby/client v0.6.0 // indirect
	github.com/moby/patternmatcher v0.6.1 // indirect
	github.com/moby/sys/sequential v0.7.0 // indirect
	github.com/moby/sys/user v0.4.1 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.64.0 // indirect
	github.com/redis/go-redis/extra/rediscmd/v9 v9.23.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.6 // indirect
	github.com/shopspring/decimal v1.5.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/arch v0.32.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261012220534-4341edf3f941 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261012220534-4341edf3f941 // indirect
	gorm.io/driver/clickhouse v0.7.0 // indirect
	gorm.io/driver/postgres v1.6.3 // indirect
)
