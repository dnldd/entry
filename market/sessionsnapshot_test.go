package market

import (
	"testing"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

func TestSessionSnapshot(t *testing.T) {
	// Ensure a session snapshot can be created.
	size := 3
	sessionSnapshot, err := NewSessionSnapshot(size)
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

	// Ensure adding a session at capacity advances the start index for the next addition.
	londonSession, err := shared.NewSession(shared.London, shared.LondonOpen, shared.LondonClose)
	assert.NoError(t, err)

	sessionSnapshot.Add(londonSession)
	assert.Equal(t, sessionSnapshot.count, size)
	assert.Equal(t, sessionSnapshot.size, size)
	assert.Equal(t, sessionSnapshot.start, 1)
	assert.Equal(t, len(sessionSnapshot.data), size)
}
