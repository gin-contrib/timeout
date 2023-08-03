package timeout

import (
	"net/http"
	"regexp"
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

// WithExtendedTimeout set extended paths timeout
func WithExtendedTimeout(extendedTimeout time.Duration) Option {
	return func(t *Timeout) {
		t.extendedTimeout = extendedTimeout
	}
}

// WithExtendedPaths set extended paths
func WithExtendedPaths(extendedPaths []string) Option {
	return func(t *Timeout) {
		t.extendedPaths = extendedPaths
	}
}

// WithHandler add gin handler
func WithHandler(h gin.HandlerFunc) Option {
	return func(t *Timeout) {
		t.handler = h
	}
}

// WithResponse add gin handler
func WithResponse(h gin.HandlerFunc) Option {
	return func(t *Timeout) {
		t.response = h
	}
}

func defaultResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, http.StatusText(http.StatusRequestTimeout))
}

// shouldExtendPathTimeout receiver matches the current path against the list of extensible timeouts.
// Not providing extended paths will not override the normal timeout duration.
func (t *Timeout) shouldExtendPathTimeout(c *gin.Context) bool {
	for _, b := range t.extendedPaths {
		matched, err := regexp.MatchString(b, c.Request.URL.Path)
		if err != nil {
			return false
		}

		if matched {
			return true
		}
	}

	return false
}

// Timeout struct
type Timeout struct {
	timeout         time.Duration
	extendedTimeout time.Duration
	extendedPaths   []string
	handler         gin.HandlerFunc
	response        gin.HandlerFunc
}
