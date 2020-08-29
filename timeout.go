package timeout

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout wraps a handler and aborts the process of the handler if the timeout is reached
func Timeout(handler gin.HandlerFunc, timeout time.Duration) gin.HandlerFunc {
	if timeout <= 0 {
		return handler
	}

	return func(c *gin.Context) {
		ch := make(chan struct{}, 1)

		go func() {
			defer func() {
				_ = gin.Recovery()
			}()
			handler(c)
			ch <- struct{}{}
		}()

		select {
		case <-ch:
		case <-time.After(timeout):
			c.AbortWithStatus(http.StatusRequestTimeout)
			c.String(http.StatusRequestTimeout, http.StatusText(http.StatusRequestTimeout))
			return
		}
	}
}
