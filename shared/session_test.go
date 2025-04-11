package shared

import (
	"testing"
	"time"

	"github.com/peterldowns/testy/assert"
)

func TestSession(t *testing.T) {
	// Ensure asia, london and new york sessions can be created.
	asia, err := NewSession(Asia, AsiaOpen, AsiaClose)
	assert.NoError(t, err)
	assert.GreaterThan(t, asia.Close.Unix(), asia.Open.Unix())

	london, err := NewSession(London, LondonOpen, LondonClose)
	assert.NoError(t, err)
	assert.GreaterThan(t, london.Close.Unix(), london.Open.Unix())

	newYork, err := NewSession(NewYork, NewYorkOpen, NewYorkClose)
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
	assert.Equal(t, asia.Low, firstCandle.Low)
	assert.Equal(t, asia.High, firstCandle.High)

	secondCandle := &Candlestick{
		Open:  12,
		Close: 20,
		High:  25,
		Low:   1,
	}

	asia.Update(secondCandle)
	assert.Equal(t, asia.Low, secondCandle.Low)
	assert.Equal(t, asia.High, secondCandle.High)

	// Ensure session can be checked if they are the current session.
	now := asia.Close.Add(time.Hour * 4)

	current := london.IsCurrentSession(now)
	assert.NotNil(t, current)

	// Ensure it can be checked if the market is open.
	open, err := IsMarketOpen(now)
	assert.NoError(t, err)
	assert.True(t, open)
}
