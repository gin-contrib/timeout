package timeout

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Writer is a writer with memory buffer
type Writer struct {
	gin.ResponseWriter
	body         *bytes.Buffer
	headers      http.Header
	mu           sync.Mutex
	timeout      bool
	wroteHeaders bool
	code         int
}

// NewWriter will return a timeout.Writer pointer
func NewWriter(w gin.ResponseWriter, buf *bytes.Buffer) *Writer {
	return &Writer{ResponseWriter: w, body: buf, headers: make(http.Header)}
}

// WriteHeaderNow the reason why we override this func is:
// once calling the func WriteHeaderNow() of based gin.ResponseWriter,
// this Writer can no longer apply the cached headers to the based
// gin.ResponseWriter. see test case `TestWriter_WriteHeaderNow` for details.
func (w *Writer) WriteHeaderNow() {
	if !w.wroteHeaders {
		if w.code == 0 {
			w.code = http.StatusOK
		}

		// Copy headers from our cache to the underlying ResponseWriter
		dst := w.ResponseWriter.Header()
		for k, vv := range w.headers {
			dst[k] = vv
		}

		w.WriteHeader(w.code)
	}
}

// Write will write data to response body
func (w *Writer) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.timeout || w.body == nil {
		return 0, nil
	}

	return w.body.Write(data)
}

// WriteHeader sends an HTTP response header with the provided status code.
// If the response writer has already written headers or if a timeout has occurred,
// this method does nothing.
func (w *Writer) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.timeout || w.wroteHeaders {
		return
	}

	// gin is using -1 to skip writing the status code
	// see https://github.com/gin-gonic/gin/blob/a0acf1df2814fcd828cb2d7128f2f4e2136d3fac/response_writer.go#L61
	if code == -1 {
		return
	}

	checkWriteHeaderCode(code)

	// Copy headers from our cache to the underlying ResponseWriter
	dst := w.ResponseWriter.Header()
	for k, vv := range w.headers {
		dst[k] = vv
	}

	w.writeHeader(code)
	w.ResponseWriter.WriteHeader(code)
}

func (w *Writer) writeHeader(code int) {
	w.wroteHeaders = true
	w.code = code
}

// Header will get response headers
func (w *Writer) Header() http.Header {
	return w.headers
}

// WriteString will write string to response body
func (w *Writer) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// FreeBuffer will release buffer pointer
func (w *Writer) FreeBuffer() {
	// if not reset body,old bytes will put in bufPool
	w.body.Reset()
	w.body = nil
}

// Status we must override Status func here,
// or the http status code returned by gin.Context.Writer.Status()
// will always be 200 in other custom gin middlewares.
func (w *Writer) Status() int {
	if w.code == 0 || w.timeout {
		return w.ResponseWriter.Status()
	}
	return w.code
}

func checkWriteHeaderCode(code int) {
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid http status code: %d", code))
	}
}
