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

func TestTimeframeString(t *testing.T) {
	tests := []struct {
		name      string
		timeframe Timeframe
		want      string
	}{
		{
			"One Hour",
			OneHour,
			"1H",
		},
		{
			"Five Minute",
			FiveMinute,
			"5m",
		},
	}

	for _, test := range tests {
		str := test.timeframe.String()
		if str != test.want {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, str)
		}
	}
}

func TestNextInterval(t *testing.T) {
	// Ensure the next time interval can be calculated.
	timeframe := FiveMinute
	now, _, err := NewYorkTime()
	assert.NoError(t, err)

	futureTime, err := NextInterval(timeframe, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, futureTime.Unix(), now.Unix())
	assert.LessThanOrEqual(t, futureTime.Unix()-now.Unix(), 300)
}
