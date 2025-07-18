package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func testResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "timeout")
}

func timeoutMiddleware() gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(500*time.Millisecond),
		timeout.WithResponse(testResponse),
	)
}

func main() {
	r := gin.New()
	r.Use(timeoutMiddleware())
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(800 * time.Millisecond)
		c.Status(http.StatusOK)
	})
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
