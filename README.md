# Timeout

[![Run Tests](https://github.com/gin-contrib/timeout/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gin-contrib/timeout/actions/workflows/go.yml)
[![Trivy Security Scan](https://github.com/gin-contrib/timeout/actions/workflows/trivy-scan.yml/badge.svg)](https://github.com/gin-contrib/timeout/actions/workflows/trivy-scan.yml)
[![codecov](https://codecov.io/gh/gin-contrib/timeout/branch/master/graph/badge.svg)](https://codecov.io/gh/gin-contrib/timeout)
[![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/timeout)](https://goreportcard.com/report/github.com/gin-contrib/timeout)
[![GoDoc](https://godoc.org/github.com/gin-contrib/timeout?status.svg)](https://pkg.go.dev/github.com/gin-contrib/timeout?tab=doc)

Timeout is a Gin middleware that wraps a handler and aborts its execution if a specified timeout is reached. This is useful for preventing slow handlers from blocking your server.

---

## Table of Contents

- [Timeout](#timeout)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
  - [How It Works](#how-it-works)
  - [API Reference](#api-reference)
    - [Configuration Options](#configuration-options)
      - [`timeout.New(opts ...Option) gin.HandlerFunc`](#timeoutnewopts-option-ginhandlerfunc)
      - [Available Options](#available-options)
    - [Example](#example)
  - [Advanced Usage](#advanced-usage)
    - [1. Custom Timeout Response](#1-custom-timeout-response)
    - [2. Global Middleware](#2-global-middleware)
    - [3. Logging Timeout Events](#3-logging-timeout-events)
    - [4. Combining with Other Middleware](#4-combining-with-other-middleware)
  - [Real-World Example: Testing Timeout](#real-world-example-testing-timeout)
  - [More Examples](#more-examples)
  - [Troubleshooting](#troubleshooting)
    - [Why is my handler still running after timeout?](#why-is-my-handler-still-running-after-timeout)
    - [Why am I getting partial responses?](#why-am-i-getting-partial-responses)
    - [What timeout value should I use?](#what-timeout-value-should-i-use)
    - [Can I use this with streaming responses?](#can-i-use-this-with-streaming-responses)
    - [Does this work with panic recovery?](#does-this-work-with-panic-recovery)
  - [Contributing](#contributing)
  - [License](#license)

---

## Features

- Abort request processing if it exceeds a configurable timeout.
- Customizable timeout response.
- Can be used as route or global middleware.
- Compatible with other Gin middleware.
- Buffered response writer to prevent partial responses.
- Panic recovery within timeout handlers.

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

func emptySuccessResponse(c *gin.Context) {
  time.Sleep(200 * time.Microsecond)
  c.String(http.StatusOK, "")
}

func main() {
  r := gin.New()

  r.GET("/", timeout.New(
    timeout.WithTimeout(100*time.Microsecond),
  ),
    emptySuccessResponse,
  )

  // Listen and Server in 0.0.0.0:8080
  if err := r.Run(":8080"); err != nil {
    log.Fatal(err)
  }
}
```

In this example, the handler will timeout because it sleeps for 200 microseconds while the timeout is set to 100 microseconds.

---

## How It Works

The timeout middleware operates by:

1. **Buffering responses**: It wraps the response writer with a buffered writer to prevent partial responses from being sent to the client.

2. **Running handlers in goroutines**: Your handler executes in a separate goroutine with a context that can be cancelled.

3. **Race against time**: The middleware waits for either:

   - Handler completion (writes buffered response to client)
   - Timeout expiry (writes timeout response instead)
   - Panic in handler (properly recovers and reports)

4. **Important limitation**: Once response headers are written to the client, the timeout response cannot be sent. The middleware can only prevent responses if it catches the timeout before headers are flushed.

**Default timeout**: If not specified, the default timeout is `5 seconds`.

---

## API Reference

### Configuration Options

#### `timeout.New(opts ...Option) gin.HandlerFunc`

Creates a new timeout middleware with the specified options.

#### Available Options

| Option                                  | Description                            | Default                                      |
| --------------------------------------- | -------------------------------------- | -------------------------------------------- |
| `WithTimeout(duration time.Duration)`   | Sets the timeout duration              | `5 * time.Second`                            |
| `WithResponse(handler gin.HandlerFunc)` | Sets a custom timeout response handler | Returns HTTP 408 with "Request Timeout" text |

### Example

```go
timeout.New(
  timeout.WithTimeout(3 * time.Second),
  timeout.WithResponse(func(c *gin.Context) {
    c.JSON(http.StatusRequestTimeout, gin.H{
      "error": "Request took too long",
      "code": "TIMEOUT",
    })
  }),
)
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

| Example                                   | Description                            | Use Case                                                                              |
| ----------------------------------------- | -------------------------------------- | ------------------------------------------------------------------------------------- |
| [example01](./_example/example01/main.go) | Minimal route-level timeout            | Quick start - applying timeout to a single route                                      |
| [example02](./_example/example02/main.go) | Global middleware with custom response | Production setup - protecting all endpoints with consistent timeout handling          |
| [example03](./_example/example03/main.go) | Logging timeout events + load testing  | Monitoring - tracking timeout occurrences with structured logging                     |
| [example04](./_example/example04/main.go) | Integration with auth middleware       | Complex middleware chains - see the [detailed README](./_example/example04/README.md) |

Explore these examples for practical patterns and advanced integration tips.

---

## Troubleshooting

### Why is my handler still running after timeout?

**Answer**: The timeout middleware can only prevent the **response** from being sent to the client. It cannot forcefully terminate the goroutine running your handler. However, the client will receive a timeout response and the connection will be closed.

**Best practice**: Check `c.Request.Context().Done()` in long-running handlers to gracefully exit:

```go
r.GET("/long", timeout.New(
  timeout.WithTimeout(2*time.Second),
), func(c *gin.Context) {
  for i := 0; i < 10; i++ {
    select {
    case <-c.Request.Context().Done():
      // Context cancelled, stop processing
      return
    default:
      // Do some work
      time.Sleep(500 * time.Millisecond)
    }
  }
  c.String(http.StatusOK, "done")
})
```

### Why am I getting partial responses?

**Answer**: If your handler writes response headers before the timeout occurs, those headers cannot be recalled. The middleware uses a buffered writer to prevent this, but streaming responses or explicit header flushes can bypass the buffer.

**Solution**: Avoid calling `c.Writer.Flush()` or using streaming responses with timeout middleware.

### What timeout value should I use?

**Guidelines**:

- **API endpoints**: 5-30 seconds (default is 5s)
- **Database queries**: 3-10 seconds
- **External API calls**: 10-30 seconds
- **Long-running jobs**: Consider using a job queue instead of HTTP with timeout

**Tip**: Set your timeout slightly lower than your load balancer or reverse proxy timeout to ensure your application responds first.

### Can I use this with streaming responses?

**Not recommended**: The middleware buffers responses to prevent partial writes. Streaming responses (SSE, chunked encoding) are incompatible with this approach.

**Alternative**: For streaming endpoints, implement timeout logic within your handler using `context.WithTimeout()`.

### Does this work with panic recovery?

**Yes**: The middleware includes panic recovery. In debug mode (`gin.SetMode(gin.DebugMode)`), it will return detailed panic information. In release mode, it re-throws the panic to be handled by upstream middleware like `gin.Recovery()`.

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
