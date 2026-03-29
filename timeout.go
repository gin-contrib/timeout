package timeout

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

var bufPool *BufferPool

const (
	defaultTimeout = 5 * time.Second
)

// panicChan transmits both the panic value and the stack trace.
type panicInfo struct {
	Value interface{}
	Stack []byte
}

// New wraps a handler and aborts the process of the handler if the timeout is reached
func New(opts ...Option) gin.HandlerFunc {
	t := &Timeout{
		timeout:  defaultTimeout,
		response: defaultResponse,
	}

	// Apply each option to the Timeout instance
	for _, opt := range opts {
		if opt == nil {
			panic("timeout Option must not be nil")
		}

		// Call the option to configure the Timeout instance
		opt(t)
	}

	// Initialize the buffer pool for response writers.
	bufPool = &BufferPool{}

	return func(c *gin.Context) {
		// Swap the response writer with a buffered writer.
		w := c.Writer
		buffer := bufPool.Get()
		tw := NewWriter(w, buffer)
		c.Writer = tw
		buffer.Reset()

		// Set a deadline on the request context so handlers can detect
		// the timeout via c.Request.Context().Done() and exit promptly.
		ctx, cancel := context.WithTimeout(c.Request.Context(), t.timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		// Channel to signal handler completion.
		finish := make(chan struct{}, 1)
		panicChan := make(chan panicInfo, 1)

		// Run the handler in a separate goroutine to enforce timeout and catch panics.
		go func() {
			defer func() {
				if p := recover(); p != nil {
					// Capture both the panic value and the stack trace.
					panicChan <- panicInfo{
						Value: p,
						Stack: debug.Stack(),
					}
				}
			}()
			c.Next()
			finish <- struct{}{}
		}()

		// Use time.NewTimer for the select trigger — it has lower latency than
		// ctx.Done() which runs AfterFunc in a separate goroutine.
		timer := time.NewTimer(t.timeout)
		defer timer.Stop()

		select {
		case pi := <-panicChan:
			// Goroutine is done (deferred recover ran), safe to touch c.
			tw.FreeBuffer()
			bufPool.Put(buffer)
			c.Writer = w
			// If in debug mode, write error and stack trace to response for easier debugging.
			if gin.IsDebugging() {
				// Add the panic error to Gin's error list and write 500 status and stack trace to response.
				// Check the error return value of c.Error to satisfy errcheck linter.
				_ = c.Error(fmt.Errorf("%v", pi.Value))
				c.Writer.WriteHeader(http.StatusInternalServerError)
				// Use fmt.Fprintf instead of Write([]byte(fmt.Sprintf(...))) to satisfy staticcheck.
				_, _ = fmt.Fprintf(c.Writer, "panic caught: %v\n", pi.Value)
				_, _ = c.Writer.Write([]byte("Panic stack trace:\n"))
				_, _ = c.Writer.Write(pi.Stack)
				return
			}
			// In non-debug mode, re-throw the original panic value to be handled by the upper middleware.
			panic(pi.Value)
		case <-finish:
			// Goroutine is done, safe to touch c. Flush buffer to response.
			tw.mu.Lock()
			defer tw.mu.Unlock()
			dst := tw.ResponseWriter.Header()
			for k, vv := range tw.Header() {
				dst[k] = vv
			}

			// Write the status code if it was set, otherwise use 200
			if tw.code != 0 {
				tw.ResponseWriter.WriteHeader(tw.code)
			}

			// Only write content if there's any
			if buffer.Len() > 0 {
				if _, err := tw.ResponseWriter.Write(buffer.Bytes()); err != nil {
					panic(err)
				}
			}
			tw.FreeBuffer()
			bufPool.Put(buffer)

		case <-timer.C:
			tw.mu.Lock()
			tw.timeout = true
			tw.FreeBuffer()
			bufPool.Put(buffer)
			tw.mu.Unlock()

			timeoutCtx := c.Copy()
			timeoutCtx.Writer = w
			if !w.Written() {
				t.response(timeoutCtx)
			}

			// Wait for the goroutine to finish to avoid data race on c.index.
			// The context deadline has already fired, so well-behaved handlers
			// that check c.Request.Context().Done() will exit promptly.
			// The tw.timeout flag causes all handler writes to be discarded.
			select {
			case <-finish:
			case <-panicChan:
			}

			// Goroutine is done. Safe to modify c.index now.
			c.Abort()
		}
	}
}
