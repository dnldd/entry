package market

import (
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
	"github.com/rs/zerolog"
)

func TestSessionSnapshot(t *testing.T) {
	now, loc, err := shared.NewYorkTime()
	assert.NoError(t, err)

	// Ensure session snapshot size cannot be negaitve or zero.
	sessionSnapshot, err := NewSessionSnapshot(-1, now)
	assert.Error(t, err)

	sessionSnapshot, err = NewSessionSnapshot(0, now)
	assert.Error(t, err)

	// Ensure a session snapshot can be created.
	size := int32(4)
	sessionSnapshot, err = NewSessionSnapshot(size, now)
	assert.NoError(t, err)

	assert.Equal(t, sessionSnapshot.count.Load(), size)
	assert.Equal(t, sessionSnapshot.size.Load(), size)
	assert.Equal(t, sessionSnapshot.start.Load(), 0)
	assert.Equal(t, len(sessionSnapshot.data), int(size))

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
	assert.Equal(t, sessionSnapshot.count.Load(), size)
	assert.Equal(t, sessionSnapshot.size.Load(), size)
	assert.Equal(t, sessionSnapshot.start.Load(), 1)
	assert.Equal(t, len(sessionSnapshot.data), int(size))

	// Ensure current session preempts to the next asia session when the time is within the
	// no session time range which is the hour between new york close and asia open.
	noSessionStr := "17:30"

	noSession, err := time.Parse(shared.SessionTimeLayout, noSessionStr)
	assert.NoError(t, err)

	noSessionTime := time.Date(now.Year(), now.Month(), now.Day(), noSession.Hour(), noSession.Minute(), 0, 0, loc)

	_, err = sessionSnapshot.SetCurrentSession(noSessionTime)
	assert.NoError(t, err)
	assert.Equal(t, sessionSnapshot.FetchCurrentSession().Name, shared.Asia)

	// Ensure sessions jobs can be executed.
	sessionSnapshot.GenerateNewSessionsJob(&zerolog.Logger{})

	// Fake the current session being the session beginning the snapshot.
	sessionSnapshot.current.Store(sessionSnapshot.start.Load())

	// Ensure fetching the last session open defaults to the current session open if there are no
	// past sessions.
	lastOpen, err = sessionSnapshot.FetchLastSessionOpen()
	assert.NoError(t, err)
	assert.Equal(t, lastOpen, sessionSnapshot.FetchCurrentSession().Open)

	// Ensure fetching the last session high and low returns an error if there are no past sessions.
	_, _, err = sessionSnapshot.FetchLastSessionHighLow()
	assert.Error(t, err)
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
	assert.Equal(t, sessionSnapshot.count.Load(), 4)
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
	assert.Equal(t, sessionSnapshot.count.Load(), 7)
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
