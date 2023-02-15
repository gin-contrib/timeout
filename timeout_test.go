package timeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func panicResponse(c *gin.Context) {
	panic("test")
}

func TestPanic(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/", New(
		WithTimeout(1*time.Second),
		WithHandler(panicResponse),
	))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func TestWriter_Status(t *testing.T) {
	r := gin.New()

	r.Use(New(
		WithTimeout(1*time.Second),
		WithHandler(func(c *gin.Context) {
			c.Next()
		}),
		WithResponse(testResponse),
	))

	r.Use(func(c *gin.Context) {
		c.Next()
		statusInMW := c.Writer.Status()
		c.Request.Header.Set("X-Status-Code-MW-Set", strconv.Itoa(statusInMW))
		t.Logf("[%s] %s %s %d\n", time.Now().Format(time.RFC3339), c.Request.Method, c.Request.URL, statusInMW)
	})

	r.GET("/test", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusInternalServerError)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, w.Result().StatusCode, http.StatusInternalServerError)
	assert.Equal(t, strconv.Itoa(http.StatusInternalServerError), req.Header.Get("X-Status-Code-MW-Set"))
}
