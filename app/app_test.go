package app

import (
	"net/http"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	app := New(&Config{
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
}

func teardown() {

}

func TestRouter(t *testing.T) {
	resp, err := http.DefaultClient.Get("http://localhost:8080/ping")
	if err != nil {
		t.Fatal(err)
		return
	}
	if resp.StatusCode != 200 {
		t.Fatal("status code is not 200")
	}
}
