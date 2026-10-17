# Testing

Your `Router` and `GRPCRegister` functions are ordinary Go functions, so you can test handlers without starting the framework.


## Testing routes

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
    
    // The Router function is plain Go: apply it to a bare engine.
    config := &app.Config{
        Service: "test-service",
        Router: func(r *gin.Engine) {
            r.GET("/ping", func(c *gin.Context) {
                c.JSON(200, gin.H{"message": "pong"})
            })
        },
    }

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

A bare `gin.New()` engine does not have the framework's built-in middleware (request id, access log, panic recovery). If a test depends on them, add the pieces you need yourself. Handlers that call `log.FromContext(ctx)` work without it, because it falls back to `slog.Default()`.

Do not call `app.Run()` or `app.RunContext()` more than once per test process, because it registers process-wide state such as the metrics registry. See [Application](application.md#driving-the-app-from-a-context).


## Capturing logs

The framework provides a mock logger for capturing and asserting log output in unit tests:

```go
import (
    "testing"
    "log/slog"
    
    "butterfly.orx.me/core/testsuite"
)

func TestUserCreation(t *testing.T) {
    // Create mock logger and capture helper
    logger, mockLog := testsuite.NewMockLog()
    
    // Option 1: Pass logger directly to code under test
    service := NewUserService(logger)
    service.CreateUser("test@example.com")
    
    // Assert log output
    if !mockLog.ContainsMessage("user created") {
        t.Error("expected 'user created' log message")
    }
    
    // Check specific log levels
    if mockLog.CountLevel(slog.LevelError) > 0 {
        t.Error("unexpected error logs")
    }
    
    // Get all messages
    messages := mockLog.Messages()
    t.Logf("logged messages: %v", messages)
    
    // Get full entries with attributes
    entries := mockLog.Entries()
    for _, entry := range entries {
        t.Logf("level=%s msg=%s attrs=%v", entry.Level, entry.Message, entry.Attrs)
    }
    
    // Reset between test cases
    mockLog.Reset()
}

func TestWithDefaultLogger(t *testing.T) {
    _, mockLog := testsuite.NewMockLog()
    
    // Option 2: Set as the default slog logger (returns restore function)
    restore := mockLog.SetAsDefault()
    defer restore()
    
    // Any code using slog.Default() will now be captured
    slog.Info("this will be captured")
    
    if !mockLog.ContainsMessage("this will be captured") {
        t.Error("message not captured")
    }
}
```
