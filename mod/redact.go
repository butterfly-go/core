package mod

import (
	"fmt"
	"log/slog"

	"butterfly.orx.me/core/internal/redact"
)

// The config structs below hold credentials. They implement fmt.Stringer and
// slog.LogValuer so that printing one, with %v, %+v or as a log attribute,
// masks the secrets instead of leaking them. (%#v still prints every field.)

func (c RedisConfig) String() string {
	return fmt.Sprintf("{mode:%s addrs:%v master_name:%s username:%s password:%s db:%d tls:%t}",
		c.ResolvedMode(), c.Nodes(), c.MasterName, c.Username, redact.Secret(c.Password), c.DB, c.TLS.Enabled)
}

func (c RedisConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("mode", c.ResolvedMode()),
		slog.Any("addrs", c.Nodes()),
		slog.String("master_name", c.MasterName),
		slog.String("username", c.Username),
		slog.String("password", redact.Secret(c.Password)),
		slog.Int("db", c.DB),
		slog.Bool("tls", c.TLS.Enabled),
	)
}

func (c MongoConfig) String() string { return fmt.Sprintf("{uri:%s}", redact.URI(c.URI)) }

func (c MongoConfig) LogValue() slog.Value {
	return slog.GroupValue(slog.String("uri", redact.URI(c.URI)))
}

func (c DBConfig) String() string {
	return fmt.Sprintf("{driver:%s host:%s port:%d user:%s password:%s db_name:%s ssl_mode:%s}",
		c.Driver, c.Host, c.Port, c.User, redact.Secret(c.Password), c.DBName, c.SSLMode)
}

func (c DBConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("driver", c.Driver),
		slog.String("host", c.Host),
		slog.Int("port", c.Port),
		slog.String("user", c.User),
		slog.String("password", redact.Secret(c.Password)),
		slog.String("db_name", c.DBName),
		slog.String("ssl_mode", c.SSLMode),
	)
}

func (c S3Config) String() string {
	return fmt.Sprintf("{endpoint:%s region:%s bucket:%s access_key:%s secret_key:%s session_token:%s}",
		c.Endpoint, c.Region, c.Bucket,
		redact.Secret(c.AccessKeyID+c.AK), redact.Secret(c.SecretAccessKey+c.SK), redact.Secret(c.SessionToken))
}

func (c S3Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("endpoint", c.Endpoint),
		slog.String("region", c.Region),
		slog.String("bucket", c.Bucket),
		slog.String("access_key", redact.Secret(c.AccessKeyID+c.AK)),
		slog.String("secret_key", redact.Secret(c.SecretAccessKey+c.SK)),
		slog.String("session_token", redact.Secret(c.SessionToken)),
	)
}
