# Example: a user service

A small service with a MySQL table, a Redis cache and a REST API. It needs this configuration:

```yaml
store:
  db:
    main:
      driver: mysql
      host: localhost
      port: 3306
      user: root
      password: password
      db_name: users
  redis:
    cache:
      addr: localhost:6379

max_users: 1000
```

And this code:

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "strconv"
    "time"

    "butterfly.orx.me/core/app"
    "butterfly.orx.me/core/log"
    "butterfly.orx.me/core/store/gorm"
    "butterfly.orx.me/core/store/redis"
    "github.com/gin-gonic/gin"
    goredis "github.com/redis/go-redis/v9"
)

type User struct {
    gorm.Model
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

type Settings struct {
    MaxUsers int `yaml:"max_users"`
}

func (s *Settings) Print() {}

var settings = &Settings{}

func main() {
    app.New(&app.Config{
        Service: "user-service",
        Config:  settings,
        Router:  setupRoutes,
        InitFunc: []func() error{
            migrate,
        },
    }).Run()
}

// The stores are connected before InitFuncs run.
func migrate() error {
    return gorm.GetDB("main").AutoMigrate(&User{})
}

func setupRoutes(r *gin.Engine) {
    api := r.Group("/api/v1")
    api.GET("/users/:id", getUser)
    api.POST("/users", createUser)
}

func getUser(c *gin.Context) {
    ctx := c.Request.Context()
    logger := log.FromContext(ctx)

    // Parse the id: never pass raw request strings to gorm's First/Where.
    id, err := strconv.ParseUint(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }
    key := "user:" + strconv.FormatUint(id, 10)

    // Cache first. A Redis error is not fatal, so fall through to the database.
    cached, err := redis.GetClient("cache").Get(ctx, key).Bytes()
    switch {
    case err == nil:
        c.Data(http.StatusOK, "application/json", cached)
        return
    case !errors.Is(err, goredis.Nil):
        logger.Warn("cache read failed", "error", err)
    }

    var user User
    err = gorm.GetDB("main").WithContext(ctx).First(&user, id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    if err != nil {
        logger.Error("load user failed", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }

    cacheUser(ctx, key, &user)
    c.JSON(http.StatusOK, user)
}

func createUser(c *gin.Context) {
    ctx := c.Request.Context()
    logger := log.FromContext(ctx)

    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    db := gorm.GetDB("main").WithContext(ctx)
    var count int64
    if err := db.Model(&User{}).Count(&count).Error; err != nil {
        logger.Error("count users failed", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }
    if settings.MaxUsers > 0 && count >= int64(settings.MaxUsers) {
        c.JSON(http.StatusConflict, gin.H{"error": "user limit reached"})
        return
    }

    if err := db.Create(&user).Error; err != nil {
        logger.Error("create user failed", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }

    logger.Info("user created", "user_id", user.ID)
    c.JSON(http.StatusCreated, user)
}

func cacheUser(ctx context.Context, key string, user *User) {
    data, err := json.Marshal(user)
    if err != nil {
        return
    }
    if err := redis.GetClient("cache").Set(ctx, key, data, time.Hour).Err(); err != nil {
        log.FromContext(ctx).Warn("cache write failed", "key", key, "error", err)
    }
}
```

Every request already gets a request id, an access log line and a trace span from the framework. The log lines written with `log.FromContext(ctx)` carry the same `request_id` and `trace_id`, so one request can be followed across the log and the trace.
