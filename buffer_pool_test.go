package timeout

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferPool(t *testing.T) {
	t.Parallel()

	pool := &BufferPool{}
	buf := pool.Get()
	assert.NotNil(t, buf)

	pool.Put(buf)
	buf2 := pool.Get()
	assert.NotNil(t, buf2)
	// Note: sync.Pool does not guarantee that Get returns the same object
	// that was Put, as the GC may collect pool entries at any time.
}

func TestBufferPool_Concurrent(t *testing.T) {
	t.Parallel()

	pool := &BufferPool{}
	numGoroutines := 50
	numGetsPerGoRoutine := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numGetsPerGoRoutine; j++ {
				buf := pool.Get()
				assert.NotNil(t, buf)
				assert.Equal(t, 0, buf.Len(), "buffer should be empty")

				buf.WriteString("test")
				buf.Reset()
				pool.Put(buf)
			}
		}()
	}

	wg.Wait()
}

func TestBufferPool_NoReset(t *testing.T) {
	t.Parallel()

	// This test demonstrates that buffers are not automatically reset
	// by the pool. The caller is responsible for resetting them.
	pool := &BufferPool{}

	buf := pool.Get()
	buf.WriteString("hello")
	assert.Equal(t, "hello", buf.String())

	// After reset, buffer should be empty
	buf.Reset()
	assert.Equal(t, "", buf.String())

	pool.Put(buf)
	buf2 := pool.Get()
	assert.NotNil(t, buf2)
}
