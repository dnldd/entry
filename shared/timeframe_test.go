package shared

import (
	"testing"

	"github.com/peterldowns/testy/assert"
)

func TestNewYorkTime(t *testing.T) {
	// Ensure new york locale times can be created.
	now, loc, err := NewYorkTime()
	assert.NoError(t, err)
	assert.Equal(t, now.Location().String(), "America/New_York")
	assert.Equal(t, now.Location().String(), loc.String())
}
