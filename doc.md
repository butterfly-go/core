# Butterfly Microservice Framework Documentation

## Introduction

Butterfly is a lightweight microservice framework designed for the Go language, aimed at simplifying the development and deployment of microservices. The framework provides core functionalities such as configuration management, service runtime, HTTP/gRPC support, data storage, and observability, allowing developers to focus on implementing business logic.

## Core Features

- **Configuration Management**: Supports file configuration and Consul configuration center, flexibly controlled through environment variables
- **Service Runtime**: Provides application lifecycle management, supports initialization function chain and graceful shutdown
- **Transport Layer Support**: 
  - HTTP server (based on Gin framework)
  - gRPC server support
  - Twirp RPC support
- **Data Storage**: 
  - GORM (MySQL and other relational databases)
  - MongoDB v2 driver
  - Redis client
  - Native SQL database connections
- **Observability**:
  - Prometheus metrics collection and exposure
  - OpenTelemetry distributed tracing
  - Structured logging system
- **Middleware Integration**: Automatic integration of OpenTelemetry middleware for request tracing

## Installation

```bash
go get butterfly.orx.me/core
```

## Quick Start

### Basic Application Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "butterfly.orx.me/core/app"
)

func main() {
    // Create application configuration
    config := &app.Config{
        Service: "my-service",
        Router: func(r *gin.Engine) {
            r.GET("/ping", func(c *gin.Context) {
                c.JSON(200, gin.H{"message": "pong"})
            })
        },
    }
    
    // Create and run application
    application := app.New(config)
    application.Run()
}
```

## Configuration Management

### 1. Environment Variable Configuration

The framework uses environment variables for configuration, all configuration items are prefixed with `BUTTERFLY_`:

```bash
# Configuration type: file or consul
export BUTTERFLY_CONFIG_TYPE=file

# File configuration path
export BUTTERFLY_CONFIG_FILE_PATH=/path/to/config.yaml

# Consul configuration
export BUTTERFLY_CONFIG_CONSUL_ADDRESS=consul:8500

# Tracing configuration
export BUTTERFLY_TRACING_ENDPOINT=localhost:4318
export BUTTERFLY_TRACING_PROVIDER=http  # or grpc

# Prometheus push configuration
export BUTTERFLY_PROMETHEUS_PUSH_ENDPOINT=http://pushgateway:9091
```

### 2. Configuration File Format

Configuration files support YAML format:

```yaml
# Core configuration
store:
  # MongoDB configuration
  mongo:
    primary:
      uri: "mongodb://localhost:27017"
    secondary:
      uri: "mongodb://localhost:27018"
  
  # Redis configuration
  redis:
    cache:
      addr: "localhost:6379"
      password: ""
      db: 0
    session:
      addr: "localhost:6380"
      password: ""
      db: 1
  
  # Database configuration
  db:
    main:
      host: "localhost"
      port: 3306
      user: "root"
      password: "password"
      db_name: "myapp"

# OpenTelemetry configuration
otel:
  # Configuration items to be extended
```

### 3. Consul Configuration Center

When using Consul as the configuration center, configurations are stored with the service name as the key:

```go
// Set to use Consul
export BUTTERFLY_CONFIG_TYPE=consul
export BUTTERFLY_CONFIG_CONSUL_ADDRESS=consul:8500

// Configuration will be read from Consul KV with service name as key
// For example: service name is "user-service", then read configuration from key "user-service"
```

## Application Structure

### Creating a Complete Application

```go
package main

import (
    "context"
    "net/http"
    
    "butterfly.orx.me/core/app"
    "butterfly.orx.me/core/store/mongo"
    "butterfly.orx.me/core/store/redis"
    "butterfly.orx.me/core/store/gorm"
    "github.com/gin-gonic/gin"
    "google.golang.org/grpc"
    pb "your-service/proto"
)

// Custom configuration structure
type MyConfig struct {
    APIKey     string `yaml:"api_key"`
    MaxRetries int    `yaml:"max_retries"`
}

func (c *MyConfig) Print() {
    // Implement configuration printing logic
}

func main() {
    config := &app.Config{
        Service: "user-service",
        Config:  &MyConfig{},
        
        // HTTP route registration
        Router: setupHTTPRoutes,
        
        // gRPC service registration
        GRPCRegister: setupGRPCServer,
        
        // Initialization functions
        InitFunc: []func() error{
            initDatabase,
            initCache,
            initMessageQueue,
        },
        
        // Teardown functions
        TeardownFunc: []func() error{
            closeDatabase,
            closeCache,
        },
    }
    
    app := app.New(config)
    app.Run()
}

func setupHTTPRoutes(r *gin.Engine) {
    // API route group
    api := r.Group("/api/v1")
    {
        api.GET("/users", listUsers)
        api.GET("/users/:id", getUser)
        api.POST("/users", createUser)
        api.PUT("/users/:id", updateUser)
        api.DELETE("/users/:id", deleteUser)
    }
    
    // Health checks
    r.GET("/health", healthCheck)
    r.GET("/ready", readinessCheck)
}

func setupGRPCServer(s *grpc.Server) {
    // Register gRPC services
    pb.RegisterUserServiceServer(s, &userServiceServer{})
}
```

### Initialization Function Chain

The framework uses a function chain pattern to manage the initialization process:

```go
// Built-in initialization order
1. config.Init()        // Initialize configuration system
2. app.InitAppConfig()  // Load application configuration
3. config.CoreConfigInit() // Initialize core configuration
4. metric.Init()        // Initialize Prometheus metrics
5. tracing.Init()       // Initialize OpenTelemetry tracing
6. store.Init()         // Initialize storage connections
7. Custom InitFunc      // User-defined initialization
```

## HTTP Service

### Gin Framework Integration

The framework integrates the Gin Web framework by default and automatically configures the following features:

```go
func setupHTTPRoutes(r *gin.Engine) {
    // Framework has automatically configured:
    // - Disabled default logging (using framework logging system)
    // - Recovery middleware
    // - OpenTelemetry tracing middleware
    
    // Add custom middleware
    r.Use(customAuthMiddleware())
    
    // Register routes
    r.GET("/", homeHandler)
    
    // API versioning
    v1 := r.Group("/api/v1")
    v1.Use(rateLimitMiddleware())
    {
        v1.GET("/resources", listResources)
        v1.POST("/resources", createResource)
    }
}

func homeHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "service": "user-service",
        "version": "1.0.0",
    })
}
```

### Twirp RPC Support

The framework provides convenient registration methods for Twirp RPC:

```go
import (
    "butterfly.orx.me/core/utils/httputils"
    "your-service/rpc/userservice"
)

func setupHTTPRoutes(r *gin.Engine) {
    // Create Twirp service
    twirpServer := userservice.NewUserServiceServer(
        &userServiceImpl{},
        nil, // hooks
    )
    
    // Register Twirp handler
    httputils.RegisterTwirpHandler(r, "/twirp/", twirpServer)
}
```

## gRPC Service

The framework automatically starts a gRPC server on port 9090:

```go
func setupGRPCServer(s *grpc.Server) {
    // Register multiple gRPC services
    pb.RegisterUserServiceServer(s, &userServer{})
    pb.RegisterAuthServiceServer(s, &authServer{})
    
    // Register gRPC reflection service (for debugging)
    reflection.Register(s)
}

// Implement gRPC service
type userServer struct {
    pb.UnimplementedUserServiceServer
}

func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // Implement business logic
    return &pb.User{
        Id:   req.Id,
        Name: "John Doe",
    }, nil
}
```

## Data Storage

### GORM (MySQL)

```go
import (
    "butterfly.orx.me/core/store/gorm"
)

// Create database connection
func initDatabase() error {
    db, err := gorm.NewDB("user:password@tcp(localhost:3306)/dbname?charset=utf8mb4")
    if err != nil {
        return err
    }
    
    // Auto migrate
    db.AutoMigrate(&User{}, &Order{})
    
    // Store to global variable or dependency injection container
    database = db
    return nil
}

// Usage example
func createUser(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if err := database.Create(&user).Error; err != nil {
        c.JSON(500, gin.H{"error": "failed to create user"})
        return
    }
    
    c.JSON(201, user)
}
```

### MongoDB

```go
import (
    "butterfly.orx.me/core/store/mongo"
    "go.mongodb.org/mongo-driver/bson"
)

// Get MongoDB client through configuration key
func getUserCollection() *mongo.Collection {
    // "primary" corresponds to store.mongo.primary in configuration file
    client := mongo.GetClient("primary")
    return client.Database("myapp").Collection("users")
}

// Usage example
func findUser(id string) (*User, error) {
    collection := getUserCollection()
    
    var user User
    err := collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user)
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

### Redis

```go
import (
    "butterfly.orx.me/core/store/redis"
    "encoding/json"
)

// Get Redis client
func getCacheClient() *redis.Client {
    // "cache" corresponds to store.redis.cache in configuration file
    return redis.GetClient("cache")
}

// Cache example
func getUserFromCache(userId string) (*User, error) {
    client := getCacheClient()
    
    // Try to get from cache
    val, err := client.Get(context.Background(), "user:"+userId).Result()
    if err == redis.Nil {
        return nil, nil // Cache miss
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

// Set cache
func setUserCache(user *User) error {
    client := getCacheClient()
    
    data, err := json.Marshal(user)
    if err != nil {
        return err
    }
    
    return client.Set(context.Background(), 
        "user:"+user.ID, 
        data, 
        time.Hour,
    ).Err()
}
```

### Native SQL Database

```go
import (
    "butterfly.orx.me/core/store/sqldb"
    "database/sql"
)

// Get native SQL connection
func getDB() *sql.DB {
    // "main" corresponds to store.db.main in configuration file
    return sqldb.GetDB("main")
}

// Use native SQL
func getUserBySQL(id int) (*User, error) {
    db := getDB()
    
    var user User
    err := db.QueryRow(
        "SELECT id, name, email FROM users WHERE id = ?", 
        id,
    ).Scan(&user.ID, &user.Name, &user.Email)
    
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

## Observability

### Prometheus Metrics

The framework automatically exposes the `/metrics` endpoint on port 2223:

```go
import (
    "butterfly.orx.me/core/observe/otel"
    "github.com/prometheus/client_golang/prometheus"
)

// Get Prometheus registry
registry := otel.PrometheusRegistry()

// Register custom metrics
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request latencies in seconds.",
        },
        []string{"method", "endpoint", "status"},
    )
    
    activeUsers = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_users_total",
            Help: "Number of active users.",
        },
    )
)

func init() {
    registry.MustRegister(requestDuration)
    registry.MustRegister(activeUsers)
}

// Use metrics
func measureRequest(c *gin.Context) {
    start := time.Now()
    
    c.Next()
    
    duration := time.Since(start).Seconds()
    requestDuration.WithLabelValues(
        c.Request.Method,
        c.FullPath(),
        fmt.Sprintf("%d", c.Writer.Status()),
    ).Observe(duration)
}
```

Access metrics:
```bash
curl http://localhost:2223/metrics
```

### OpenTelemetry Tracing

The framework automatically configures OpenTelemetry tracing:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func processOrder(ctx context.Context, orderID string) error {
    // Create new span
    tracer := otel.Tracer("order-service")
    ctx, span := tracer.Start(ctx, "processOrder")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.String("order.id", orderID),
        attribute.String("order.status", "processing"),
    )
    
    // Call other services
    if err := validateOrder(ctx, orderID); err != nil {
        span.RecordError(err)
        return err
    }
    
    // Add event
    span.AddEvent("order validated")
    
    return nil
}
```

Configure trace export:
```bash
# HTTP export
export BUTTERFLY_TRACING_PROVIDER=http
export BUTTERFLY_TRACING_ENDPOINT=localhost:4318

# gRPC export
export BUTTERFLY_TRACING_PROVIDER=grpc
export BUTTERFLY_TRACING_ENDPOINT=localhost:4317
```

### Logging System

The framework provides a simple wrapper around Go standard library's `slog` for structured logging with context support:

```go
import (
    "butterfly.orx.me/core/log"
    "log/slog"
)

// Get logger from context (returns default logger if none exists)
func handler(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Get logger from context - always returns a valid logger
    logger := log.FromContext(ctx)
    logger.Info("handling request", "path", c.Request.URL.Path)
    
    // Create a logger with additional context
    contextLogger := slog.With("request_id", "123", "user_id", "456")
    
    // Store logger in context for downstream use
    ctx = log.WithLogger(ctx, contextLogger)
    
    // Pass context to other functions
    processRequest(ctx)
}

func processRequest(ctx context.Context) {
    // Retrieve logger from context
    logger := log.FromContext(ctx)
    
    // Use structured logging
    logger.Info("processing request",
        "step", "validation",
        "timestamp", time.Now(),
    )
    
    // Different log levels
    logger.Debug("debug info", "key", "value")
    logger.Info("info message", "count", 42)
    logger.Warn("warning", "retry", 3)
    logger.Error("error occurred", "error", err)
}

// Direct usage of slog (without context)
func simpleLogging() {
    // Use default logger
    slog.Info("simple log message", "key", "value")
    
    // Create custom logger with attributes
    logger := slog.With("service", "user-service", "version", "1.0.0")
    logger.Info("service started")
}
```

## Practical Examples

### Complete User Service Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    "log/slog"
    
    "butterfly.orx.me/core/app"
    "butterfly.orx.me/core/log"
    "butterfly.orx.me/core/store/gorm"
    "butterfly.orx.me/core/store/redis"
    "github.com/gin-gonic/gin"
    gormDriver "gorm.io/gorm"
)

var (
    db     *gormDriver.DB
    cache  *redis.Client
    logger = slog.With("service", "user-service")
)

type User struct {
    gormDriver.Model
    Name     string `json:"name" binding:"required"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"-"`
}

type Config struct {
    JWTSecret string `yaml:"jwt_secret"`
    MaxUsers  int    `yaml:"max_users"`
}

func (c *Config) Print() {
    logger.Info("config loaded", 
        "max_users", c.MaxUsers,
    )
}

func main() {
    config := &app.Config{
        Service: "user-service",
        Config:  &Config{},
        Router:  setupRoutes,
        InitFunc: []func() error{
            initDB,
            initCache,
        },
    }
    
    app := app.New(config)
    app.Run()
}

func initDB() error {
    var err error
    db, err = gorm.NewDB("root:password@tcp(localhost:3306)/users?charset=utf8mb4")
    if err != nil {
        return fmt.Errorf("failed to connect database: %w", err)
    }
    
    // Auto migrate
    return db.AutoMigrate(&User{})
}

func initCache() error {
    cache = redis.GetClient("cache")
    if cache == nil {
        logger.Warn("cache not configured")
    }
    return nil
}

func setupRoutes(r *gin.Engine) {
    // Health check
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })
    
    // API routes
    api := r.Group("/api/v1")
    api.Use(errorHandler())
    {
        api.GET("/users", listUsers)
        api.GET("/users/:id", getUser)
        api.POST("/users", createUser)
        api.PUT("/users/:id", updateUser)
        api.DELETE("/users/:id", deleteUser)
    }
}

func errorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        if len(c.Errors) > 0 {
            err := c.Errors.Last()
            
            // Get logger from context or use default
            logger := log.FromContext(c.Request.Context())
            logger.Error("request failed", 
                "path", c.Request.URL.Path,
                "method", c.Request.Method,
                "error", err.Error(),
            )
            
            c.JSON(500, gin.H{
                "error": "internal server error",
            })
        }
    }
}

func listUsers(c *gin.Context) {
    var users []User
    
    // Try to get from cache
    if cache != nil {
        if cached, _ := cache.Get(c, "users:all").Result(); cached != "" {
            c.Data(200, "application/json", []byte(cached))
            return
        }
    }
    
    // Query from database
    if err := db.Find(&users).Error; err != nil {
        c.Error(err)
        return
    }
    
    c.JSON(200, users)
}

func getUser(c *gin.Context) {
    id := c.Param("id")
    
    // Check cache first
    if cache != nil {
        if cached, _ := cache.Get(c, "user:"+id).Result(); cached != "" {
            c.Data(200, "application/json", []byte(cached))
            return
        }
    }
    
    var user User
    if err := db.First(&user, id).Error; err != nil {
        c.JSON(404, gin.H{"error": "user not found"})
        return
    }
    
    c.JSON(200, user)
}

func createUser(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if err := db.Create(&user).Error; err != nil {
        c.Error(err)
        return
    }
    
    // Use context-based logging
    logger := log.FromContext(c.Request.Context())
    logger.Info("user created", "user_id", user.ID, "email", user.Email)
    
    // Clear cache
    if cache != nil {
        cache.Del(c, "users:all")
    }
    
    c.JSON(201, user)
}

func updateUser(c *gin.Context) {
    id := c.Param("id")
    
    var user User
    if err := db.First(&user, id).Error; err != nil {
        c.JSON(404, gin.H{"error": "user not found"})
        return
    }
    
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    if err := db.Save(&user).Error; err != nil {
        c.Error(err)
        return
    }
    
    // Clear cache
    if cache != nil {
        cache.Del(c, "user:"+id, "users:all")
    }
    
    c.JSON(200, user)
}

func deleteUser(c *gin.Context) {
    id := c.Param("id")
    
    if err := db.Delete(&User{}, id).Error; err != nil {
        c.Error(err)
        return
    }
    
    // Clear cache
    if cache != nil {
        cache.Del(c, "user:"+id, "users:all")
    }
    
    c.JSON(204, nil)
}
```

## Deployment Recommendations

### Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/service .
COPY config.yaml .

CMD ["./service"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
      - name: user-service
        image: your-registry/user-service:latest
        ports:
        - containerPort: 8080  # HTTP
        - containerPort: 9090  # gRPC
        - containerPort: 2223  # Metrics
        env:
        - name: BUTTERFLY_CONFIG_TYPE
          value: "consul"
        - name: BUTTERFLY_CONFIG_CONSUL_ADDRESS
          value: "consul:8500"
        - name: BUTTERFLY_TRACING_ENDPOINT
          value: "otel-collector:4318"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Testing

### Unit Test Example

```go
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
    
    "butterfly.orx.me/core/app"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestPingRoute(t *testing.T) {
    // Set test mode
    gin.SetMode(gin.TestMode)
    
    // Create test application
    config := &app.Config{
        Service: "test-service",
        Router: func(r *gin.Engine) {
            r.GET("/ping", func(c *gin.Context) {
                c.JSON(200, gin.H{"message": "pong"})
            })
        },
    }
    
    // Create router
    router := gin.New()
    config.Router(router)
    
    // Create test request
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/ping", nil)
    
    // Execute request
    router.ServeHTTP(w, req)
    
    // Verify response
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), "pong")
}
```

## Frequently Asked Questions

### 1. How to configure multiple databases?

Define multiple database connections in the configuration file:

```yaml
store:
  db:
    primary:
      host: "primary.db.com"
      port: 3306
      user: "app"
      password: "secret"
      db_name: "main"
    analytics:
      host: "analytics.db.com"
      port: 3306
      user: "reader"
      password: "secret"
      db_name: "analytics"
```

Use by key when accessing:
```go
primaryDB := sqldb.GetDB("primary")
analyticsDB := sqldb.GetDB("analytics")
```

### 2. How to customize log format?

The framework uses `slog`, which can be customized by setting different Handlers:

```go
import (
    "log/slog"
    "os"
)

// JSON format
jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})
slog.SetDefault(slog.New(jsonHandler))

// Text format
textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})
slog.SetDefault(slog.New(textHandler))
```

### 3. How to implement graceful shutdown?

The framework automatically handles graceful shutdown, but you can register cleanup functions:

```go
config := &app.Config{
    Service: "my-service",
    TeardownFunc: []func() error{
        func() error {
            // Close database connection
            sqlDB, _ := db.DB()
            return sqlDB.Close()
        },
        func() error {
            // Close message queue connection
            return messageQueue.Close()
        },
    },
}
```

### 4. How to add rate limiting?

Implement rate limiting using middleware:

```go
import (
    "golang.org/x/time/rate"
)

func rateLimitMiddleware(rps int) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Limit(rps), rps)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatusJSON(429, gin.H{
                "error": "too many requests",
            })
            return
        }
        c.Next()
    }
}

// Usage
r.Use(rateLimitMiddleware(100)) // 100 RPS
```

## Performance Optimization Recommendations

1. **Database Connection Pool Configuration**
   ```go
   sqlDB, _ := db.DB()
   sqlDB.SetMaxIdleConns(10)
   sqlDB.SetMaxOpenConns(100)
   sqlDB.SetConnMaxLifetime(time.Hour)
   ```

2. **Use Caching to Reduce Database Load**
   - Implement multi-level caching strategy
   - Use Redis as distributed cache
   - Set reasonable cache expiration times

3. **Enable gzip Compression**
   ```go
   import "github.com/gin-contrib/gzip"
   
   r.Use(gzip.Gzip(gzip.DefaultCompression))
   ```

4. **Use Connection Pools for Connection Reuse**
   - HTTP client connection pool
   - Database connection pool
   - Redis connection pool