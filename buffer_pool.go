package timeout

import (
	"bytes"
	"sync"
)

// BufferPool represents a pool of buffers with a fast-path cache for the last buffer.
type BufferPool struct {
	pool sync.Pool
	mu   sync.Mutex
	last *bytes.Buffer
}

// Get returns a buffer from the buffer pool. It first tries to return the most
// recently returned buffer without touching sync.Pool to minimize contention.
func (p *BufferPool) Get() *bytes.Buffer {
	p.mu.Lock()
	if p.last != nil {
		b := p.last
		p.last = nil
		p.mu.Unlock()
		return b
	}
	p.mu.Unlock()

	buf := p.pool.Get()
	if buf == nil {
		return &bytes.Buffer{}
	}
	return buf.(*bytes.Buffer)
}

// Put adds a buffer back to the pool without resetting it. The caller is
// responsible for calling Reset() when appropriate.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	p.mu.Lock()
	if p.last == nil {
		p.last = buf
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	p.pool.Put(buf)
}
