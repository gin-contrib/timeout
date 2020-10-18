package timeout

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteHeader(t *testing.T) {
	code1 := 99
	errmsg1 := fmt.Sprintf("invalid http status code: %d", code1)
	code2 := 1000
	errmsg2 := fmt.Sprintf("invalid http status code: %d", code2)

	writer := Writer{}
	assert.PanicsWithValue(t, errmsg1, func() {
		writer.WriteHeader(code1)
	})
	assert.PanicsWithValue(t, errmsg2, func() {
		writer.WriteHeader(code2)
	})
}
