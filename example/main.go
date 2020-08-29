package main

import (
	"net/http"
	"time"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func emptySuccessResponse(c *gin.Context) {
	time.Sleep(200 * time.Microsecond)
	c.String(http.StatusOK, "")
}

func main() {
	r := gin.New()

	r.GET("/", timeout.Timeout(emptySuccessResponse, 100*time.Microsecond))

	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")
}
