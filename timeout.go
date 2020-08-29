package timeout

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Option for timeout
type Option func(*Timeout)

// WithTimeout set timeout
func WithTimeout(timeout time.Duration) Option {
	return func(t *Timeout) {
		t.timeout = timeout
	}
}

// WithHandler add gin handler
func WithHandler(h gin.HandlerFunc) Option {
	return func(t *Timeout) {
		t.handler = h
	}
}

// Timeout struct
type Timeout struct {
	timeout  time.Duration
	handler  gin.HandlerFunc
	response gin.HandlerFunc
}

// New wraps a handler and aborts the process of the handler if the timeout is reached
func New(opts ...Option) gin.HandlerFunc {
	const (
		defaultTimeout = 5 * time.Second
	)

	t := &Timeout{
		timeout: defaultTimeout,
		handler: nil,
	}

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated
		opt(t)
	}

	if t.timeout <= 0 {
		return t.handler
	}

	return func(c *gin.Context) {
		ch := make(chan struct{}, 1)

		go func() {
			defer func() {
				_ = gin.Recovery()
			}()
			t.handler(c)
			ch <- struct{}{}
		}()

		select {
		case <-ch:
		case <-time.After(t.timeout):
			c.AbortWithStatus(http.StatusRequestTimeout)
			c.String(http.StatusRequestTimeout, http.StatusText(http.StatusRequestTimeout))
			return
		}
	}
}
