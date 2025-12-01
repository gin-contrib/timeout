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

// custom middleware straight from example
func timeoutMiddleware() gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(500*time.Millisecond),
		timeout.WithResponse(testResponse),
	)
}

// simple middleware to always throw a 401
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		debug := c.Query("debug")
		if debug != "true" {
			c.Next()
			return
		}
		c.AbortWithStatus(401)
	}
}

func main() {
	r := gin.New()

	// middleware
	r.Use(gin.Logger())
	r.Use(timeoutMiddleware()) // 1. timeout middleware
	r.Use(authMiddleware())    // 2. auth middleware
	r.Use(
		gin.Recovery(),
	) // recommend to use this middleware to recover from any panics in the handlers.

	r.GET("/", func(c *gin.Context) {
		time.Sleep(1000 * time.Millisecond)
		c.String(http.StatusOK, "Hello world!")
	})
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
