package app

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

var configFilePath string

func TestMain(m *testing.M) {
	code := 1
	if err := setup(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
	} else {
		code = m.Run()
	}
	teardown()
	os.Exit(code)
}

type testConfig struct {
}

func (t *testConfig) Print() {}

func setup() error {
	configFile, err := os.CreateTemp("", "butterfly-app-test-*.yaml")
	if err != nil {
		return err
	}
	configFilePath = configFile.Name()
	_, err = configFile.WriteString("log:\n  level: error\n")
	if closeErr := configFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	if err := os.Setenv("BUTTERFLY_CONFIG_TYPE", "file"); err != nil {
		return err
	}
	if err := os.Setenv("BUTTERFLY_CONFIG_FILE_PATH", configFilePath); err != nil {
		return err
	}
	if err := os.Setenv("BUTTERFLY_TRACING_DISABLE", "true"); err != nil {
		return err
	}

	app := New(&Config{
		Config:  new(testConfig),
		Service: "test",
		Router: func(r *gin.Engine) {
			r.GET("/ping", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "pong",
				})
			})
		},
	})
	go app.Run()
	time.Sleep(time.Second * 5)
	return nil
}

func teardown() {
	if configFilePath != "" && filepath.IsAbs(configFilePath) {
		_ = os.Remove(configFilePath)
	}
}

func TestRouter(t *testing.T) {
	resp, err := http.DefaultClient.Get("http://localhost:8080/ping")
	if err != nil {
		t.Fatal(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("status code is not 200")
	}
}
