package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestSession(t *testing.T) {
	now, loc, err := NewYorkTime()
	assert.NoError(t, err)

	// Ensure asia, london and new york sessions can be created.
	asia, err := NewSession(Asia, AsiaOpen, AsiaClose, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, asia.Close.Unix(), asia.Open.Unix())

	london, err := NewSession(London, LondonOpen, LondonClose, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, london.Close.Unix(), london.Open.Unix())

	newYork, err := NewSession(NewYork, NewYorkOpen, NewYorkClose, now)
	assert.NoError(t, err)
	assert.GreaterThan(t, newYork.Close.Unix(), newYork.Open.Unix())

	// Ensure a session can be updated.
	firstCandle := &Candlestick{
		Open:  5,
		Close: 10,
		High:  12,
		Low:   2,
	}

	asia.Update(firstCandle)
	assert.Equal(t, asia.Low.Load(), firstCandle.Low)
	assert.Equal(t, asia.High.Load(), firstCandle.High)

	secondCandle := &Candlestick{
		Open:  12,
		Close: 20,
		High:  25,
		Low:   1,
	}

	asia.Update(secondCandle)
	assert.Equal(t, asia.Low.Load(), secondCandle.Low)
	assert.Equal(t, asia.High.Load(), secondCandle.High)

	// Ensure sessions can be checked to assert if they are the current session.
	futureTime := asia.Close.Add(time.Hour * 4)

	current := london.IsCurrentSession(futureTime)
	assert.NotNil(t, current)

	// Ensure it can be checked if the market is open.
	open, _, err := IsMarketOpen(now)
	assert.NoError(t, err)
	assert.True(t, open)

	// Ensure an error is returned if the provided time is not new york localized.
	utcNow := now.UTC()
	_, err = NewSession("unknown", AsiaOpen, NewYorkClose, utcNow)
	assert.Error(t, err)

	// Ensure current session returns no session for the hour between new york close and asia open
	// where the market is closed.
	noSessionStr := "17:30"

	noSession, err := time.Parse(SessionTimeLayout, noSessionStr)
	assert.NoError(t, err)

	noSessionTime := time.Date(now.Year(), now.Month(), now.Day(), noSession.Hour(), noSession.Minute(), 0, 0, loc)

	// Ensure the any provided time can be checked to be within the high volume window.
	hwv, err := InHighVolumeWindow(noSessionTime)
	assert.NoError(t, err)
	assert.False(t, hwv)

	highVolumeWindowStr := "9:00"
	highVolumeWindow, err := time.Parse(SessionTimeLayout, highVolumeWindowStr)
	assert.NoError(t, err)

	highVolumeWindowTime := time.Date(now.Year(), now.Month(), now.Day(), highVolumeWindow.Hour(), highVolumeWindow.Minute(), 0, 0, loc)

	hwv, err = InHighVolumeWindow(highVolumeWindowTime)
	assert.NoError(t, err)
	assert.True(t, hwv)

	name, session, err := CurrentSession(noSessionTime)
	assert.NoError(t, err)
	assert.Nil(t, session)
	assert.Equal(t, name, "")
}
