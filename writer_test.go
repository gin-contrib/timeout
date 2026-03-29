package timeout

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWriteHeader(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{
			name: "code less than 100",
			code: 99,
		},
		{
			name: "code greater than 999",
			code: 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := Writer{}
			errmsg := fmt.Sprintf("invalid http status code: %d", tt.code)
			assert.PanicsWithValue(t, errmsg, func() {
				writer.WriteHeader(tt.code)
			})
		})
	}
}

func TestWriteHeader_SkipMinusOne(t *testing.T) {
	code := -1

	writer := Writer{}
	assert.NotPanics(t, func() {
		writer.WriteHeader(code)
		assert.False(t, writer.wroteHeaders)
	})
}

func TestWriter_Status(t *testing.T) {
	r := gin.New()

	r.Use(New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	))

	r.Use(func(c *gin.Context) {
		c.Next()
		statusInMW := c.Writer.Status()
		c.Request.Header.Set("X-Status-Code-MW-Set", strconv.Itoa(statusInMW))
		t.Logf(
			"[%s] %s %s %d\n",
			time.Now().Format(time.RFC3339),
			c.Request.Method,
			c.Request.URL,
			statusInMW,
		)
	})

	r.GET("/test", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusInternalServerError)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(
		t,
		strconv.Itoa(http.StatusInternalServerError),
		req.Header.Get("X-Status-Code-MW-Set"),
	)
}

// testNew is a copy of New() with a small change to the timeoutHandler() function.
// ref: https://github.com/gin-contrib/timeout/issues/31
func testNew(duration time.Duration) gin.HandlerFunc {
	return New(
		WithTimeout(duration),
		WithResponse(timeoutHandler()),
	)
}

// timeoutHandler returns a handler that returns a 504 Gateway Timeout error.
func timeoutHandler() gin.HandlerFunc {
	gatewayTimeoutErr := struct {
		Error string `json:"error"`
	}{
		Error: "Timed out.",
	}

	return func(c *gin.Context) {
		log.Printf("request timed out: [method=%s,path=%s]",
			c.Request.Method, c.Request.URL.Path)
		c.JSON(http.StatusGatewayTimeout, gatewayTimeoutErr)
	}
}

// TestHTTPStatusCode tests the HTTP status code of the response.
func TestHTTPStatusCode(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	type testCase struct {
		Name          string
		Method        string
		Path          string
		ExpStatusCode int
		Handler       gin.HandlerFunc
	}

	var (
		cases = []testCase{
			{
				Name:          "Plain text (200)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusOK,
				Handler: func(ctx *gin.Context) {
					ctx.String(http.StatusOK, "I'm text!")
				},
			},
			{
				Name:          "Plain text (201)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusCreated,
				Handler: func(ctx *gin.Context) {
					ctx.String(http.StatusCreated, "I'm created!")
				},
			},
			{
				Name:          "Plain text (204)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusNoContent,
				Handler: func(ctx *gin.Context) {
					ctx.String(http.StatusNoContent, "")
				},
			},
			{
				Name:          "Plain text (400)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusBadRequest,
				Handler: func(ctx *gin.Context) {
					ctx.String(http.StatusBadRequest, "")
				},
			},
			{
				Name:          "JSON (200)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusOK,
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, gin.H{"field": "value"})
				},
			},
			{
				Name:          "JSON (201)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusCreated,
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusCreated, gin.H{"field": "value"})
				},
			},
			{
				Name:          "JSON (204)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusNoContent,
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusNoContent, nil)
				},
			},
			{
				Name:          "JSON (400)",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusBadRequest,
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusBadRequest, nil)
				},
			},
			{
				Name:          "No reply",
				Method:        http.MethodGet,
				Path:          "/me",
				ExpStatusCode: http.StatusOK,
				Handler:       func(ctx *gin.Context) {},
			},
		}

		initCase = func(c testCase) (*http.Request, *httptest.ResponseRecorder) {
			return httptest.NewRequest(c.Method, c.Path, nil), httptest.NewRecorder()
		}
	)

	for i := range cases {
		t.Run(cases[i].Name, func(tt *testing.T) {
			router := gin.Default()

			router.Use(testNew(1 * time.Second))
			router.GET("/*root", cases[i].Handler)

			req, resp := initCase(cases[i])
			router.ServeHTTP(resp, req)

			assert.Equal(tt, cases[i].ExpStatusCode, resp.Code)
		})
	}
}

func TestWriter_WriteHeaderNow(t *testing.T) {
	const (
		testOrigin  = "*"
		testMethods = "GET,HEAD,POST,PUT,OPTIONS"
	)

	g := gin.New()
	g.Use(testNew(time.Second * 3))
	g.Use(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Origin", testOrigin)
			c.Header("Access-Control-Allow-Methods", testMethods)

			// Below 3 lines can be replaced with `c.AbortWithStatus(http.StatusNoContent)`
			c.Status(http.StatusNoContent)
			c.Writer.WriteHeaderNow()
			c.Abort()

			return
		}
		c.Next()
	})
	g.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "It's works!")
	})

	serv := httptest.NewServer(g)
	defer serv.Close()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodOptions,
		serv.URL+"/test",
		nil,
	)
	if err != nil {
		t.Fatal("NewRequest:", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("Do request:", err)
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, testOrigin, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, testMethods, resp.Header.Get("Access-Control-Allow-Methods"))
}

// TestWriteHeader_MultipleCallsLastWins verifies that WriteHeader can be
// called multiple times and the last value wins, matching gin's native
// responseWriter behavior. This is required for r.Static() to work correctly
// because gin's createStaticHandler calls WriteHeader(404) preemptively,
// then http.FileServer overrides it with WriteHeader(200).
func TestWriteHeader_MultipleCallsLastWins(t *testing.T) {
	writer := Writer{}
	writer.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, writer.code)

	writer.WriteHeader(http.StatusOK)
	assert.Equal(t, http.StatusOK, writer.code)
	assert.False(t, writer.wroteHeaders, "wroteHeaders should not be set by WriteHeader")
}

// TestWriteHeader_AfterWriteHeaderNow verifies that WriteHeader is a no-op
// after WriteHeaderNow has flushed headers.
func TestWriteHeader_AfterWriteHeaderNow(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
		c.Writer.WriteHeaderNow()
		// This call should be ignored since headers are already flushed
		c.Writer.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestWriteHeader_AfterTimeout verifies that WriteHeader is a no-op
// after a timeout has occurred.
func TestWriteHeader_AfterTimeout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(10*time.Millisecond),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "timeout")
		}),
	), func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		// These should be silently ignored after timeout
		c.Writer.WriteHeader(http.StatusOK)
		c.String(http.StatusOK, "should not appear")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "timeout", w.Body.String())
}

// TestWriteHeaderNow_DefaultStatus verifies that WriteHeaderNow defaults
// to 200 if no status code has been set.
func TestWriteHeaderNow_DefaultStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		c.Header("X-Custom", "value")
		c.Writer.WriteHeaderNow()
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestWriteHeaderNow_Idempotent verifies that calling WriteHeaderNow
// multiple times only flushes once.
func TestWriteHeaderNow_Idempotent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		c.Status(http.StatusCreated)
		c.Writer.WriteHeaderNow()
		c.Writer.WriteHeaderNow() // second call should be no-op
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestWriteHeader_StatusVisibleInMiddleware verifies that after multiple
// WriteHeader calls, the correct (last) status is visible to downstream middleware.
func TestWriteHeader_StatusVisibleInMiddleware(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	var statusInMW int
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		statusInMW = c.Writer.Status()
	})
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		// Mimic gin's static file pattern: set 404 then override to 200
		c.Writer.WriteHeader(http.StatusNotFound)
		c.Writer.WriteHeader(http.StatusOK)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, http.StatusOK, statusInMW)
	assert.Equal(t, "ok", w.Body.String())
}

// TestWriteHeader_VariousOverrides verifies WriteHeader override works
// with different status code combinations using route-level middleware.
func TestWriteHeader_VariousOverrides(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	tests := []struct {
		name     string
		first    int
		second   int
		expected int
	}{
		{"404 to 200", http.StatusNotFound, http.StatusOK, http.StatusOK},
		{"500 to 200", http.StatusInternalServerError, http.StatusOK, http.StatusOK},
		{"200 to 301", http.StatusOK, http.StatusMovedPermanently, http.StatusMovedPermanently},
		{"403 to 401", http.StatusForbidden, http.StatusUnauthorized, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/test", New(
				WithTimeout(1*time.Second),
				WithResponse(testResponse),
			), func(c *gin.Context) {
				c.Writer.WriteHeader(tt.first)
				c.Writer.WriteHeader(tt.second)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expected, w.Code)
		})
	}
}

// TestWriteHeader_OverrideWithBody verifies that WriteHeader override
// works correctly when the handler also writes a response body.
func TestWriteHeader_OverrideWithBody(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		// Mimic gin's static file pattern: pre-set 404, then serve with 200
		c.Writer.WriteHeader(http.StatusNotFound)
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Header().Set("Content-Type", "text/plain")
		_, _ = c.Writer.Write([]byte("file content here"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "file content here", w.Body.String())
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
}

// TestWriteHeader_JSONResponse verifies that WriteHeader works correctly
// with JSON responses using route-level middleware.
func TestWriteHeader_JSONResponse(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	tests := []struct {
		name     string
		code     int
		body     interface{}
		expected string
	}{
		{
			"200 JSON",
			http.StatusOK,
			gin.H{"status": "ok"},
			`{"status":"ok"}`,
		},
		{
			"201 JSON",
			http.StatusCreated,
			gin.H{"id": 1},
			`{"id":1}`,
		},
		{
			"400 JSON error",
			http.StatusBadRequest,
			gin.H{"error": "bad request"},
			`{"error":"bad request"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/test", New(
				WithTimeout(1*time.Second),
				WithResponse(testResponse),
			), func(c *gin.Context) {
				c.JSON(tt.code, tt.body)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.code, w.Code)
			assert.JSONEq(t, tt.expected, w.Body.String())
		})
	}
}

// TestWriteHeader_NoResponse verifies that a handler with no explicit
// response returns 200 using route-level middleware.
func TestWriteHeader_NoResponse(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		// handler does nothing
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestWriteHeader_AbortWithStatus verifies that AbortWithStatus works
// correctly with route-level timeout middleware.
func TestWriteHeader_AbortWithStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

// TestStaticFileServing verifies that static files served via r.Static()
// return the correct HTTP status code (200) along with the file content.
// Reproduces https://github.com/gin-contrib/timeout/issues/68
func TestStaticFileServing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	dir := t.TempDir()
	testContent := "hello static file"
	err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(testContent), 0o600)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(New(
		WithTimeout(5*time.Second),
		WithResponse(testResponse),
	))
	r.Static("/static", dir)

	// existing file should return 200 with correct body
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/test.txt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testContent, w.Body.String())

	// non-existent file should return 404
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/static/nonexistent.txt", nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

// TestStaticFileServing_GroupLevel verifies that static files work correctly
// when the timeout middleware is applied at the group level.
func TestStaticFileServing_GroupLevel(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	dir := t.TempDir()
	testContent := "group level static"
	err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(testContent), 0o600)
	assert.NoError(t, err)

	r := gin.New()
	g := r.Group("/files", New(
		WithTimeout(5*time.Second),
		WithResponse(testResponse),
	))
	g.Static("/static", dir)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/files/static/test.txt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testContent, w.Body.String())
}

// TestStaticFileServing_ContentTypeHeader verifies that Content-Type header
// is correctly propagated when serving static files through the timeout middleware.
func TestStaticFileServing_ContentTypeHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"key":"value"}`), 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "page.html"), []byte(`<html><body>hello</body></html>`), 0o600)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(New(
		WithTimeout(5*time.Second),
		WithResponse(testResponse),
	))
	r.Static("/static", dir)

	// JSON file
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/data.json", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, `{"key":"value"}`, w.Body.String())

	// HTML file
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/static/page.html", nil)
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Header().Get("Content-Type"), "text/html")
}

// TestStaticFileServing_Concurrent verifies that concurrent static file
// requests work correctly with the timeout middleware and race detector.
func TestStaticFileServing_Concurrent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	dir := t.TempDir()
	files := map[string]string{
		"a.txt": "content-a",
		"b.txt": "content-b",
		"c.txt": "content-c",
	}
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
		assert.NoError(t, err)
	}

	r := gin.New()
	r.Use(New(
		WithTimeout(5*time.Second),
		WithResponse(testResponse),
	))
	r.Static("/static", dir)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		for name, expectedContent := range files {
			wg.Add(1)
			go func(name, expectedContent string) {
				defer wg.Done()
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/static/"+name, nil)
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code,
					"file %s should return 200", name)
				assert.Equal(t, expectedContent, w.Body.String(),
					"file %s should return correct content", name)
			}(name, expectedContent)
		}
	}
	wg.Wait()
}

// TestStaticFileServing_WithTimeout verifies that static file serving
// correctly returns a timeout response when the request takes too long.
func TestStaticFileServing_WithTimeout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o600)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(New(
		WithTimeout(10*time.Millisecond),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "timeout")
		}),
	))
	// Add a slow middleware before static to force a timeout
	r.Use(func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		c.Next()
	})
	r.Static("/static", dir)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/test.txt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "timeout", w.Body.String())
}

// TestRouteLevel_TimeoutFires verifies that route-level timeout middleware
// returns the timeout response when the handler exceeds the deadline.
func TestRouteLevel_TimeoutFires(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/slow", New(
		WithTimeout(50*time.Microsecond),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "too slow")
		}),
	), func(c *gin.Context) {
		time.Sleep(200 * time.Microsecond)
		c.String(http.StatusOK, "done")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "too slow", w.Body.String())
}

// TestRouteLevel_SuccessBeforeTimeout verifies that route-level timeout
// middleware returns the handler's response when it completes in time.
func TestRouteLevel_SuccessBeforeTimeout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/fast", New(
		WithTimeout(1*time.Second),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "too slow")
		}),
	), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"result": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"result":"success"}`, w.Body.String())
}

// TestRouteLevel_MultipleRoutes verifies that different routes can have
// different timeout configurations applied at the route level.
func TestRouteLevel_MultipleRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Fast route with short timeout — handler completes in time
	r.GET("/fast", New(
		WithTimeout(1*time.Second),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "fast timeout")
		}),
	), func(c *gin.Context) {
		c.String(http.StatusOK, "fast ok")
	})

	// Slow route with short timeout — handler exceeds deadline
	r.GET("/slow", New(
		WithTimeout(10*time.Millisecond),
		WithResponse(func(c *gin.Context) {
			c.String(http.StatusGatewayTimeout, "slow timeout")
		}),
	), func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond)
		c.String(http.StatusOK, "slow ok")
	})

	// Fast route succeeds
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/fast", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "fast ok", w1.Body.String())

	// Slow route times out
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/slow", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusGatewayTimeout, w2.Code)
	assert.Equal(t, "slow timeout", w2.Body.String())
}

// TestRouteLevel_ConcurrentRequests verifies that route-level timeout
// handles concurrent requests without data races.
func TestRouteLevel_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.GET("/test", New(
		WithTimeout(1*time.Second),
		WithResponse(testResponse),
	), func(c *gin.Context) {
		c.String(http.StatusOK, "hello")
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "hello", w.Body.String())
		}()
	}
	wg.Wait()
}
