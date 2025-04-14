package market

import (
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestSessionSnapshot(t *testing.T) {
	// Ensure session snapshot size cannot be negaitve or zero.
	sessionSnapshot, err := NewSessionSnapshot(-1)
	assert.Error(t, err)

	sessionSnapshot, err = NewSessionSnapshot(0)
	assert.Error(t, err)

	// Ensure a session snapshot can be created.
	size := 3
	sessionSnapshot, err = NewSessionSnapshot(size)
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

	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	tomorrow := now.Add(time.Hour * 24)

	// Ensure adding a session at capacity advances the start index for the next addition.
	londonSession, err := shared.NewSession(shared.London, shared.LondonOpen, shared.LondonClose, tomorrow)
	assert.NoError(t, err)

	sessionSnapshot.Add(londonSession)
	assert.Equal(t, sessionSnapshot.count, size)
	assert.Equal(t, sessionSnapshot.size, size)
	assert.Equal(t, sessionSnapshot.start, 1)
	assert.Equal(t, len(sessionSnapshot.data), size)
}
