package app

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

type testConfig struct {
}

func (t *testConfig) Print() {}

func setup() {
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
}

func teardown() {

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
