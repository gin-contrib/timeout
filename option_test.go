package timeout

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	t.Parallel()

	timeout := &Timeout{}
	customTimeout := 5 * time.Second
	customResponse := func(c *gin.Context) {
		c.String(http.StatusGatewayTimeout, "test response")
	}

	// Apply options
	WithTimeout(customTimeout)(timeout)
	WithResponse(customResponse)(timeout)

	// Assertions
	assert.Equal(t, customTimeout, timeout.timeout)
	assert.NotNil(t, timeout.response)

	// To fully verify the handler, we can execute it and check the response.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	timeout.response(c)
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestDefaultResponse(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	defaultResponse(c)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, http.StatusText(http.StatusRequestTimeout), w.Body.String())
}
