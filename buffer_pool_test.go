package timeout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBuffer(t *testing.T) {
	pool := &BufferPool{}
	buf := pool.Get()
	assert.NotEqual(t, nil, buf)
	pool.Put(buf)
	buf2 := pool.Get()
	assert.NotEqual(t, nil, buf2)
}
