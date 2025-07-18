package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.Use(timeout.New(
		timeout.WithTimeout(100*time.Microsecond),
		timeout.WithHandler(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "timeout")
		}),
	), func(c *gin.Context) {
		c.Next()

		if c.Writer.Status() == http.StatusRequestTimeout {
			slog.Error("request timeout")
		}
	})

	r.GET("/long", func(c *gin.Context) {
		time.Sleep(10 * time.Second)
		c.String(http.StatusOK, "long time ago")
	})

	s := &http.Server{
		Addr:              ":8000",
		Handler:           r,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: time.Second * 5,
	}

	if err := s.ListenAndServe(); err != nil {
		slog.Error("ListenAndServe failed", "err", err)
	}
}
