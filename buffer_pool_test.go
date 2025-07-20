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
	assert.Same(t, buf, buf2)
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

	// This test demonstrates that it is the responsibility of the
	// caller to reset the buffer before putting it back into the pool.
	pool := &BufferPool{}

	// Get a buffer, write to it, and put it back without resetting.
	buf := pool.Get()
	buf.WriteString("hello")
	pool.Put(buf)

	// Get the buffer again and check if the old content is still there.
	buf2 := pool.Get()
	assert.Same(t, buf, buf2)
	assert.Equal(t, "hello", buf2.String())
}
