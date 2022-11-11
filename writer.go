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

// Write will write data to response body
func (w *Writer) Write(data []byte) (int, error) {
	if w.timeout || w.body == nil {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.body.Write(data)
}

// WriteHeader will write http status code
func (w *Writer) WriteHeader(code int) {
	if w.timeout || w.wroteHeaders {
		return
	}

	// gin is using -1 to skip writing the status code
	// see https://github.com/gin-gonic/gin/blob/a0acf1df2814fcd828cb2d7128f2f4e2136d3fac/response_writer.go#L61
	if code == -1 {
		return
	}

	checkWriteHeaderCode(code)

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writeHeader(code)
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

func checkWriteHeaderCode(code int) {
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid http status code: %d", code))
	}
}
