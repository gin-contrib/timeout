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
	r.GET("/", New(WithTimeout(50*time.Microsecond), WithHandler(emptySuccessResponse)))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, http.StatusText(http.StatusRequestTimeout), w.Body.String())
}

func TestWithoutTimeout(t *testing.T) {
	r := gin.New()
	r.GET("/", New(WithTimeout(-1*time.Microsecond), WithHandler(emptySuccessResponse)))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func testResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "test response")
}

func TestCustomResponse(t *testing.T) {
	r := gin.New()
	r.GET("/", New(
		WithTimeout(100*time.Microsecond),
		WithHandler(emptySuccessResponse),
		WithResponse(testResponse),
	))

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
		WithHandler(emptySuccessResponse2),
		WithResponse(testResponse),
	))

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
		WithHandler(func(c *gin.Context) {
			c.Next()
		}),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, `{"error": "timeout error"}`)
		}),
	), func(c *gin.Context) {
		time.Sleep(2 * time.Second) // wait almost same as timeout
		c.String(http.StatusBadRequest, `{"error": "handler error"}`)
	})

	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
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
Test to ensure no further middleware is executed after timeout (covers c.Next() removal)
This test verifies that after a timeout occurs, no subsequent middleware is executed.
*/
func TestNoNextAfterTimeout(t *testing.T) {
	r := gin.New()
	called := false
	r.Use(New(WithTimeout(50*time.Millisecond), WithHandler(func(c *gin.Context) {
		time.Sleep(100 * time.Millisecond)
		c.String(http.StatusOK, "should not reach")
	})))
	r.Use(func(c *gin.Context) {
		called = true
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.False(t, called, "next middleware should not be called after timeout")
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
		WithHandler(func(c *gin.Context) {
			panic("timeout panic test")
		}),
	))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/panic", nil)
	r.ServeHTTP(w, req)

	// Verify the response status code and body.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "panic caught: timeout panic test")
}
