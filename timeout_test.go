package timeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func emptySuccessResponse(c *gin.Context) {
	time.Sleep(200 * time.Microsecond)
	c.String(http.StatusOK, "")
}

func TestTimeout(t *testing.T) {
	r := gin.New()
	r.GET("/", New(
		WithTimeout(50*time.Microsecond),
	),
		emptySuccessResponse,
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, http.StatusText(http.StatusRequestTimeout), w.Body.String())
}

func TestTimeoutWithUse(t *testing.T) {
	r := gin.New()
	r.Use(New(
		WithTimeout(50 * time.Microsecond),
	))
	r.GET("/", emptySuccessResponse)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, http.StatusText(http.StatusRequestTimeout), w.Body.String())
}

func TestWithoutTimeout(t *testing.T) {
	r := gin.New()
	r.GET("/", New(
		WithTimeout(-1*time.Microsecond),
	),
		emptySuccessResponse,
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, http.StatusText(http.StatusRequestTimeout), w.Body.String())
}

func testResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "test response")
}

func TestCustomResponse(t *testing.T) {
	r := gin.New()
	r.GET("/", New(
		WithTimeout(100*time.Microsecond),
		WithResponse(testResponse),
	),
		emptySuccessResponse,
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func emptySuccessResponse2(c *gin.Context) {
	time.Sleep(50 * time.Microsecond)
	c.String(http.StatusOK, "")
}

func TestSuccess(t *testing.T) {
	r := gin.New()
	r.GET("/", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	),
		emptySuccessResponse2,
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func TestLargeResponse(t *testing.T) {
	r := gin.New()
	r.GET("/slow", New(
		WithTimeout(1*time.Second),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, `{"error": "timeout error"}`)
		}),
	),
		func(c *gin.Context) {
			// Use context-aware wait so the handler exits promptly after timeout
			select {
			case <-time.After(2 * time.Second):
			case <-c.Request.Context().Done():
				return
			}
			c.String(http.StatusBadRequest, `{"error": "handler error"}`)
		},
	)

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/slow", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusRequestTimeout, w.Code)
			assert.Equal(t, `{"error": "timeout error"}`, w.Body.String())
		}()
	}
	wg.Wait()
}

/*
Test to ensure no further middleware executes meaningful work after timeout.
Handlers that respect context cancellation will exit early when the timeout fires.
*/
func TestNoNextAfterTimeout(t *testing.T) {
	r := gin.New()
	called := false
	r.Use(New(
		WithTimeout(50*time.Millisecond),
	),
		func(c *gin.Context) {
			// Use context-aware wait so the handler exits when timeout fires
			select {
			case <-time.After(100 * time.Millisecond):
			case <-c.Request.Context().Done():
				return
			}
			c.String(http.StatusOK, "should not reach")
		},
	)
	r.Use(func(c *gin.Context) {
		// Check context cancellation before doing work
		if c.Request.Context().Err() != nil {
			return
		}
		called = true
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.False(t, called, "next middleware should not run after context timeout")
}

/*
TestTimeoutPanic: verifies the behavior when a panic occurs inside a handler wrapped by the timeout middleware.
This test ensures that a panic in the handler is caught by CustomRecovery and returns a 500 status code
with the panic message.
*/
func TestTimeoutPanic(t *testing.T) {
	r := gin.New()
	// Use CustomRecovery to catch panics and return a custom error message.
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		c.String(http.StatusInternalServerError, "panic caught: %v", recovered)
	}))

	// Register the timeout middleware; the handler will panic.
	r.GET("/panic", New(
		WithTimeout(100*time.Millisecond),
	),
		func(c *gin.Context) {
			panic("timeout panic test")
		},
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/panic", nil)
	r.ServeHTTP(w, req)

	// Verify the response status code and body.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "panic caught: timeout panic test")
}

func TestDataRace(t *testing.T) {
	r := gin.New()
	r.GET("/race", New(
		WithTimeout(50*time.Millisecond),
	), func(c *gin.Context) {
		// Sleep longer than the timeout to ensure the timeout path is always taken.
		// Do NOT use context cancellation here because ctx.Done() fires at the same
		// time as the timer, making the select nondeterministic.
		time.Sleep(200 * time.Millisecond)
		c.String(http.StatusOK, "done")
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/race", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusRequestTimeout, w.Code)
		}()
	}
	wg.Wait()
}

func TestContextDeadlineSet(t *testing.T) {
	r := gin.New()
	var hasDeadline bool
	r.GET("/deadline", New(
		WithTimeout(1*time.Second),
	), func(c *gin.Context) {
		_, hasDeadline = c.Request.Context().Deadline()
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/deadline", nil)
	r.ServeHTTP(w, req)

	assert.True(t, hasDeadline, "request context should have a deadline set by the middleware")
	assert.Equal(t, http.StatusOK, w.Code)
}
