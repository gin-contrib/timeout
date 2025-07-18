# Timeout

[![Run Tests](https://github.com/gin-contrib/timeout/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gin-contrib/timeout/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/gin-contrib/timeout/branch/master/graph/badge.svg)](https://codecov.io/gh/gin-contrib/timeout)
[![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/timeout)](https://goreportcard.com/report/github.com/gin-contrib/timeout)
[![GoDoc](https://godoc.org/github.com/gin-contrib/timeout?status.svg)](https://pkg.go.dev/github.com/gin-contrib/timeout?tab=doc)
[![Join the chat at https://gitter.im/gin-gonic/gin](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gin-gonic/gin)

Timeout is a Gin middleware that wraps a handler and aborts its execution if a specified timeout is reached. This is useful for preventing slow handlers from blocking your server.

---

## Features

- Abort request processing if it exceeds a configurable timeout.
- Customizable timeout response.
- Can be used as route or global middleware.
- Compatible with other Gin middleware.

---

## Installation

```bash
go get github.com/gin-contrib/timeout
```

---

## Quick Start

A minimal example that times out a slow handler:

```go
// _example/example01/main.go
package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gin-contrib/timeout"
  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.New()

  // Apply timeout middleware to a single route
  r.GET("/", timeout.New(
    timeout.WithTimeout(100*time.Microsecond),
  ), func(c *gin.Context) {
    time.Sleep(200 * time.Microsecond)
    c.String(http.StatusOK, "")
  })

  if err := r.Run(":8080"); err != nil {
    log.Fatal(err)
  }
}
```

---

## Advanced Usage

### 1. Custom Timeout Response

You can define a custom response when a timeout occurs:

```go
// Custom timeout response for a single route
func testResponse(c *gin.Context) {
  c.String(http.StatusRequestTimeout, "custom timeout response")
}

r.GET("/", timeout.New(
  timeout.WithTimeout(100*time.Microsecond),
  timeout.WithResponse(testResponse),
), func(c *gin.Context) {
  time.Sleep(200 * time.Microsecond)
  c.String(http.StatusOK, "")
})
```

---

### 2. Global Middleware

Apply the timeout middleware to all routes:

```go
func testResponse(c *gin.Context) {
  c.String(http.StatusRequestTimeout, "timeout")
}

func timeoutMiddleware() gin.HandlerFunc {
  return timeout.New(
    timeout.WithTimeout(500*time.Millisecond),
    timeout.WithResponse(testResponse),
  )
}

func main() {
  r := gin.New()
  r.Use(timeoutMiddleware())
  r.GET("/slow", func(c *gin.Context) {
    time.Sleep(800 * time.Millisecond)
    c.Status(http.StatusOK)
  })
  if err := r.Run(":8080"); err != nil {
    log.Fatal(err)
  }
}
```

---

### 3. Logging Timeout Events

You can combine the timeout middleware with custom logging for timeout events:

```go
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
```

---

### 4. Combining with Other Middleware

You can stack the timeout middleware with other middleware, such as authentication or logging:

```go
func testResponse(c *gin.Context) {
  c.String(http.StatusRequestTimeout, "timeout")
}

// Custom timeout middleware
func timeoutMiddleware() gin.HandlerFunc {
  return timeout.New(
    timeout.WithTimeout(500*time.Millisecond),
    timeout.WithResponse(testResponse),
  )
}

// Example auth middleware
func authMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    debug := c.Query("debug")
    if debug != "true" {
      c.Next()
      return
    }
    c.AbortWithStatus(401)
  }
}

func main() {
  r := gin.New()
  r.Use(gin.Logger())
  r.Use(timeoutMiddleware())
  r.Use(authMiddleware())
  r.Use(gin.Recovery())

  r.GET("/", func(c *gin.Context) {
    time.Sleep(1 * time.Second)
    c.String(http.StatusOK, "Hello world!")
  })

  if err := r.Run(":8080"); err != nil {
    log.Fatal(err)
  }
}
```

---

## Real-World Example: Testing Timeout

Suppose your handler always takes longer than the timeout:

```go
// _example/example04/main.go (handler always times out)
r.GET("/", func(c *gin.Context) {
  time.Sleep(1 * time.Second)
  c.String(http.StatusOK, "Hello world!")
})
```

With a 500ms timeout, any request will return HTTP 408:

```bash
curl -i http://localhost:8080/
```

**Expected response:**

```bash
HTTP/1.1 408 Request Timeout
Content-Type: text/plain; charset=utf-8

timeout
```

---

## More Examples

The [`_example`](./_example) directory contains additional usage scenarios:

- **example01**: Minimal usage – how to apply a timeout to a single route.
- **example02**: Using timeout as a global middleware with a custom timeout response.
- **example03**: Demonstrates logging timeout events and includes a `concurrent_requests.sh` script for load testing.
- **example04**: Shows integration with custom authentication middleware and includes its own [README](./_example/example04/README.md) for detailed explanation.

Explore these examples for practical patterns and advanced integration tips.
