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

func extendedTimeoutMiddleware() gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(200*time.Millisecond),          // Default timeout on all routes
		timeout.WithExtendedTimeout(1000*time.Millisecond), // Extended timeout on pattern based routes
		timeout.WithExtendedPaths([]string{"/ext.*"}),      // List of patterns to allow extended timeouts
		timeout.WithHandler(func(c *gin.Context) {
			c.Next()
		}),
		timeout.WithResponse(testResponse),
	)
}

func main() {
	r := gin.New()
	r.Use(extendedTimeoutMiddleware())
	r.GET("/extended", func(c *gin.Context) {
		time.Sleep(800 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
