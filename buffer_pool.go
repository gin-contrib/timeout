package timeout

import (
	"bytes"
	"sync"
)

// BufferPool represents a pool of buffers.
// It uses sync.Pool to manage the reuse of buffers, reducing memory allocation and garbage collection overhead.
type BufferPool struct {
	pool sync.Pool
}

// Get returns a buffer from the buffer pool.
// If the pool is empty, a new buffer is created and returned.
// This method ensures the reuse of buffers, improving performance.
func (p *BufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get()
	if buf == nil {
		// If there are no available buffers in the pool, create a new one
		return &bytes.Buffer{}
	}
	// Convert the retrieved buffer to *bytes.Buffer type and return it
	return buf.(*bytes.Buffer)
}

// Put adds a buffer back to the pool.
// This method allows the buffer to be reused in the future, reducing the number of memory allocations.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	p.pool.Put(buf)
}
