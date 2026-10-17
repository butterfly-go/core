# Data storage

Every store entry in the `store` section of the [configuration](configuration.md#core-configuration-reference) is connected at startup, before your `InitFunc`s run. Fetch a client by its config key:

| Store | Config key | Getter | Returns |
|---|---|---|---|
| Redis | `store.redis.<name>` | `redis.GetClient(name)` | `*redis.Client` ([go-redis v9](https://github.com/redis/go-redis)) |
| SQL | `store.db.<name>` | `sqldb.GetDB(name)` | `*sql.DB` |
| GORM | `store.db.<name>` (MySQL only) | `gorm.GetDB(name)` | `*gorm.DB` |
| MongoDB | `store.mongo.<name>` | `mongo.GetClient(name)` | `*mongo.Client` ([driver v2](https://pkg.go.dev/go.mongodb.org/mongo-driver/v2)) |
| S3 | `store.s3.<name>` | `s3.GetClient(name)`, `s3.GetBucket(name)` | `*s3.Client` ([AWS SDK v2](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3)), bucket name |

Things that apply to all of them:

- **An unknown name returns `nil`** (`""` for `GetBucket`). The getters do not panic, so a typo in a name shows up as a nil pointer when you first use the client. Fetch clients once, at startup, to find mistakes early.
- **Tracing and metrics are built in** for Redis, SQL, GORM and Mongo: each query becomes a span, and the SQL pool reports connection stats on `/metrics`.
- **The framework closes the clients** (Redis, SQL, Mongo) during [graceful shutdown](application.md#graceful-shutdown), after your `TeardownFunc`s. Do not close them yourself.
- **Readiness checks** ping every Redis, SQL and Mongo client, so `/readyz` fails while one of them is unreachable. S3 is not checked. See [Health checks](observability.md#health-checks).

## Redis

```go
import (
    "context"
    "encoding/json"
    "errors"
    "time"

    "butterfly.orx.me/core/store/redis"
    goredis "github.com/redis/go-redis/v9"
)

func getUserFromCache(ctx context.Context, id string) (*User, error) {
    // "cache" is store.redis.cache in the configuration file.
    client := redis.GetClient("cache")

    val, err := client.Get(ctx, "user:"+id).Result()
    if errors.Is(err, goredis.Nil) {
        return nil, nil // cache miss
    }
    if err != nil {
        return nil, err
    }

    var user User
    if err := json.Unmarshal([]byte(val), &user); err != nil {
        return nil, err
    }
    return &user, nil
}

func setUserCache(ctx context.Context, user *User) error {
    data, err := json.Marshal(user)
    if err != nil {
        return err
    }
    return redis.GetClient("cache").Set(ctx, "user:"+user.ID, data, time.Hour).Err()
}
```

`redis.Nil` is not exported by the framework. Import go-redis for it, as above.

## SQL

`store.db.<name>` supports MySQL and PostgreSQL (`driver: mysql` or `postgres`). `sqldb.GetDB` returns the standard `*sql.DB`:

```go
import (
    "context"

    "butterfly.orx.me/core/store/sqldb"
)

func getUser(ctx context.Context, id int) (*User, error) {
    db := sqldb.GetDB("main") // store.db.main

    var user User
    err := db.QueryRowContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?", id, // use $1 on PostgreSQL
    ).Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err
    }
    return &user, nil
}
```

## GORM

For a `store.db` entry whose driver is MySQL, the framework also builds a GORM handle on the same connection pool, with the tracing plugin installed. Get it with `gorm.GetDB`. It is not created for PostgreSQL entries, so `GetDB` returns `nil` for them. Use `sqldb.GetDB` there.

```go
import (
    "context"
    "errors"

    "butterfly.orx.me/core/store/gorm"
)

func createUser(ctx context.Context, user *User) error {
    db := gorm.GetDB("main") // store.db.main
    return db.WithContext(ctx).Create(user).Error
}

func findUser(ctx context.Context, id uint) (*User, error) {
    var user User
    err := gorm.GetDB("main").WithContext(ctx).First(&user, id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    return &user, err
}
```

The `gorm` package re-exports the commonly used GORM types (`DB`, `Model`, `DeletedAt`, ...) and errors (`ErrRecordNotFound`, `ErrDuplicatedKey`, ...), so you usually do not need to import `gorm.io/gorm` yourself. Run migrations from an `InitFunc`:

```go
InitFunc: []func() error{
    func() error { return gorm.GetDB("main").AutoMigrate(&User{}) },
},
```

If you want a GORM connection that is not in the configuration file, `gorm.NewDB(dsn)` opens a MySQL connection with tracing and pool metrics. The framework does not close those. You own them.

## MongoDB

```go
import (
    "context"

    "butterfly.orx.me/core/store/mongo"
    "go.mongodb.org/mongo-driver/v2/bson"
)

func findUser(ctx context.Context, id string) (*User, error) {
    // "primary" is store.mongo.primary in the configuration file.
    client := mongo.GetClient("primary")
    collection := client.Database("myapp").Collection("users")

    var user User
    if err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
        return nil, err
    }
    return &user, nil
}
```

The framework's `mongo` package only provides `GetClient`. Types such as `*mongo.Collection` come from `go.mongodb.org/mongo-driver/v2/mongo`.

## S3-compatible object storage

AWS S3 and compatible services such as MinIO work. See the [configuration reference](configuration.md#core-configuration-reference) for the options. The short forms `ak` and `sk` are accepted for `access_key_id` and `secret_access_key`, and MinIO needs `use_path_style: true`.

```go
import (
    "context"
    "io"

    "butterfly.orx.me/core/store/s3"
    awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func uploadFile(ctx context.Context, key string, body io.Reader) error {
    // "assets" is store.s3.assets in the configuration file.
    client := s3.GetClient("assets")
    bucket := s3.GetBucket("assets")

    _, err := client.PutObject(ctx, &awss3.PutObjectInput{
        Bucket: &bucket,
        Key:    &key,
        Body:   body,
    })
    return err
}
```
