package timeout

import (
	"bytes"
	"sync"
)

// BufferPool represents a pool of buffers.
type BufferPool struct {
	pool sync.Pool
}

// Get returns a buffer from the buffer pool.
// If the pool is empty, a new buffer is created and returned.
func (p *BufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get()
	if buf == nil {
		return &bytes.Buffer{}
	}
	return buf.(*bytes.Buffer)
}

// Put adds a buffer back to the pool.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	p.pool.Put(buf)
}
