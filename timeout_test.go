package timeout

import (
	"context"
	"crypto/tls"
	"io"
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

func TestDeadlineExceeded(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	r := gin.New()
	r.GET("/", New(
		WithTimeout(50*time.Millisecond),
		WithHandler(func(c *gin.Context) {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			assert.Equal(t, "value", c.Request.Header.Get("X-Test"))
			assert.Equal(t, context.DeadlineExceeded, c.Request.Context().Err())
			select {
			case <-c.Request.Context().Done():
				// OK
			case <-time.After(1 * time.Second):
				assert.Fail(t, "context is not done")
			}
		}),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "timeout")
		}),
	))

	srv := &http.Server{
		Addr:              ":3124",
		Handler:           r,
		ReadHeaderTimeout: 1 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			assert.Error(t, err)
		}
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	testRequest(
		t,
		"http://localhost:3124",
		"408 Request Timeout",
		"timeout",
	)

	wg.Wait()

	if err := srv.Close(); err != nil {
		assert.Fail(t, "Server Close: "+err.Error())
	}
}

// params[0]=url example:http://127.0.0.1:8080/index (cannot be empty)
// params[1]=response status (custom compare status) default:"200 OK"
// params[2]=response body (custom compare content)  default:"it worked"
func testRequest(t *testing.T, params ...string) {
	if len(params) == 0 {
		t.Fatal("url cannot be empty")
	}

	req, err := http.NewRequest(
		"GET",
		params[0],
		nil,
	)
	assert.NoError(t, err)

	header := http.Header{}
	header.Set("X-Test", "value")
	req.Header = header

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	body, ioerr := io.ReadAll(resp.Body)
	assert.NoError(t, ioerr)

	responseStatus := "200 OK"
	if len(params) > 1 && params[1] != "" {
		responseStatus = params[1]
	}

	responseBody := "it worked"
	if len(params) > 2 && params[2] != "" {
		responseBody = params[2]
	}

	assert.Equal(t, responseStatus, resp.Status, "should get a "+responseStatus)
	assert.Equal(t, responseBody, string(body), "resp body should match")
}
