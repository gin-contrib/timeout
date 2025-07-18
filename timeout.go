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
		// Channel to signal handler completion.
		finish := make(chan struct{}, 1)
		// panicChan transmits both the panic value and the stack trace.
		type panicInfo struct {
			Value interface{}
			Stack []byte
		}
		panicChan := make(chan panicInfo, 1)

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
			// Use the copied context to avoid data race when running handler in a goroutine.
			c.Next()
			finish <- struct{}{}
		}()

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

		case <-time.After(t.timeout):
			tw.mu.Lock()
			// Handler timed out: set timeout flag and clean up
			tw.timeout = true
			tw.FreeBuffer()
			bufPool.Put(buffer)
			tw.mu.Unlock()

			// Create a fresh context for the timeout response
			// Important: check if headers were already written
			timeoutCtx := c.Copy()
			timeoutCtx.Writer = w

			// Only write timeout response if headers haven't been written to original writer
			if !w.Written() {
				t.response(timeoutCtx)
			}
			// Abort the context to prevent further middleware execution after timeout
			c.AbortWithStatus(http.StatusRequestTimeout)
		}
	}
}
