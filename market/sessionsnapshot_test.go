package market

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestSessionSnapshot(t *testing.T) {
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure session snapshot size cannot be negaitve or zero.
	sessionSnapshot, err := NewSessionSnapshot(-1, now)
	assert.Error(t, err)

	sessionSnapshot, err = NewSessionSnapshot(0, now)
	assert.Error(t, err)

	// Ensure a session snapshot can be created.
	size := 4
	sessionSnapshot, err = NewSessionSnapshot(size, now)
	assert.NoError(t, err)

	assert.Equal(t, sessionSnapshot.count, size)
	assert.Equal(t, sessionSnapshot.size, size)
	assert.Equal(t, sessionSnapshot.start, 0)
	assert.Equal(t, len(sessionSnapshot.data), size)

	// Ensure the current session can be fetched.
	current := sessionSnapshot.FetchCurrentSession()
	assert.NotEqual(t, current, nil)

	// Ensure the last session open can be fetched.
	lastOpen, err := sessionSnapshot.FetchLastSessionOpen()
	assert.NoError(t, err)
	assert.False(t, lastOpen.IsZero())

	// Ensure the last session high and low can be fetched.
	high, low, err := sessionSnapshot.FetchLastSessionHighLow()
	assert.NoError(t, err)
	assert.Equal(t, high, 0)
	assert.Equal(t, low, 0)

	tomorrow := now.AddDate(0, 0, 1)

	// Ensure adding a session at capacity advances the start index for the next addition.
	londonSession, err := shared.NewSession(shared.London, shared.LondonOpen, shared.LondonClose, tomorrow)
	assert.NoError(t, err)

	sessionSnapshot.Add(londonSession)
	assert.Equal(t, sessionSnapshot.count, size)
	assert.Equal(t, sessionSnapshot.size, size)
	assert.Equal(t, sessionSnapshot.start, 1)
	assert.Equal(t, len(sessionSnapshot.data), size)
}

func TestGenerateNewSessions(t *testing.T) {
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)
	tomorrowNext := tomorrow.AddDate(0, 0, 1)

	sessionSnapshot, err := NewSessionSnapshot(shared.SnapshotSize, now)
	assert.NoError(t, err)

	// Asia -> London -> New York -> Asia (today-tomorrow)
	assert.Equal(t, sessionSnapshot.count, 4)
	assert.Equal(t, sessionSnapshot.data[0].Open.Day(), yesterday.Day())
	assert.Equal(t, sessionSnapshot.data[0].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[1].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[1].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[2].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[2].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[3].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[3].Close.Day(), tomorrow.Day())

	err = sessionSnapshot.GenerateNewSessions(tomorrow)
	assert.NoError(t, err)

	// Asia -> London -> New York -> Asia (today-tomorrow) -> London (tomorrow) -> New York (tomorrow) -> Asia (tomorrow-nextday)
	assert.Equal(t, sessionSnapshot.count, 7)
	assert.Equal(t, sessionSnapshot.data[0].Open.Day(), yesterday.Day())
	assert.Equal(t, sessionSnapshot.data[0].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[1].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[1].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[2].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[2].Close.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[3].Open.Day(), now.Day())
	assert.Equal(t, sessionSnapshot.data[3].Close.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[4].Open.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[4].Close.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[5].Open.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[5].Close.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[6].Open.Day(), tomorrow.Day())
	assert.Equal(t, sessionSnapshot.data[6].Close.Day(), tomorrowNext.Day())
}
