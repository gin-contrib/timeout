package timeout

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
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
