package timeout

import (
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

		// Create a copy of the context before starting the goroutine to avoid data race
		cCopy := c.Copy()
		// Set the copied context's writer to our timeout writer to ensure proper buffering
		cCopy.Writer = tw

		// Channel to signal handler completion.
		finish := make(chan struct{}, 1)
		panicChan := make(chan panicInfo, 1)

		// Run the handler in a separate goroutine to enforce timeout and catch panics.
		// We use cCopy.Next() instead of c.Next() to avoid data races on c.index
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
			cCopy.Next()
			finish <- struct{}{}
		}()

		// Block until handler finishes, panics, or times out.
		// This prevents the middleware from returning and gin continuing the handler chain
		// while the goroutine is still executing.

		select {
		case pi := <-panicChan:
			// Handler panicked: free buffer, restore writer, and print stack trace if in debug mode.
			tw.FreeBuffer()
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
			// Handler finished successfully: flush buffer to response.
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
			// Restore the original writer
			c.Writer = w
			// Prevent further middleware execution
			c.Abort()

		case <-time.After(t.timeout):
			tw.mu.Lock()
			// Handler timed out: set timeout flag and clean up
			tw.timeout = true
			tw.FreeBuffer()
			bufPool.Put(buffer)
			tw.mu.Unlock()

			// Only write timeout response if headers haven't been written to original writer
			// We write directly to w to avoid touching c while the handler goroutine may still be executing
			if !w.Written() {
				w.WriteHeader(http.StatusRequestTimeout)
				_, _ = w.Write([]byte(http.StatusText(http.StatusRequestTimeout)))
			}
			// Restore the original writer so gin knows the response was written
			// This is safe because tw.timeout is set, so any writes from the handler goroutine
			// to tw will be ignored
			c.Writer = w
		}
	}
}
