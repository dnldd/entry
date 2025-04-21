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
	now, _, err := NewYorkTime()
	assert.NoError(t, err)

	futureTimeFiveMinuteInterval, err := NextInterval(FiveMinute, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, futureTimeFiveMinuteInterval.Unix(), now.Unix())
	assert.LessThanOrEqual(t, futureTimeFiveMinuteInterval.Unix()-now.Unix(), 300)

	futureTimeOneHourInterval, err := NextInterval(OneHour, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, futureTimeOneHourInterval.Unix(), now.Unix())
	assert.LessThanOrEqual(t, futureTimeOneHourInterval.Unix()-now.Unix(), 3600)

	// Ensure an error is returned if the timeframe is unknown
	_, err = NextInterval(Timeframe(999), now)
	assert.Error(t, err)
}
