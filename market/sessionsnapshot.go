package market

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/rs/zerolog"
)

const (
	// SessionGenerationTime is the time new sessions are generated to cover the day.
	SessionGenerationTime = "00:01"
)

// CandlestickSnapshot represents a snapshot of session data.
type SessionSnapshot struct {
	data    []*shared.Session
	start   atomic.Int32
	current atomic.Int32
	count   atomic.Int32
	size    atomic.Int32
}

// NewSessionSnapshot initializes a new session snapshot.
func NewSessionSnapshot(size int32, now time.Time) (*SessionSnapshot, error) {
	if size < 0 {
		return nil, errors.New("snapshot size cannot be negative")
	}
	if size == 0 {
		return nil, errors.New("snapshot size cannot be zero")
	}

	snapshot := &SessionSnapshot{
		data: make([]*shared.Session, size),
	}

	snapshot.size.Store(size)

	err := snapshot.GenerateNewSessions(now)
	if err != nil {
		return nil, fmt.Errorf("adding sessions to snapshot: %v", err)
	}

	_, err = snapshot.SetCurrentSession(now)
	if err != nil {
		return nil, fmt.Errorf("setting current session: %v", err)
	}

	return snapshot, nil
}

// Adds adds the provided session to the snapshot.
func (s *SessionSnapshot) Add(session *shared.Session) {
	end := (s.start.Load() + s.count.Load()) % s.size.Load()
	s.data[end] = session

	if s.count.Load() == s.size.Load() {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((s.start.Load() + 1) % s.size.Load())
	} else {
		s.count.Add(1)
	}
}

// Exists checks whether the snapshot has an existing session corresponding to
// the provided name and opening day.
func (s *SessionSnapshot) Exists(name string, open time.Time) bool {
	for i := s.count.Load() - 1; i >= 0; i-- {
		idx := (s.start.Load() + i) % s.size.Load()
		session := s.data[idx]
		if session.Name == name && session.Open.Equal(open) {
			return true
		}
	}

	return false
}

// GenerateNewSessions generate a new set of sessions for the snapshot.
func (s *SessionSnapshot) GenerateNewSessions(now time.Time) error {
	yesterday := now.AddDate(0, 0, -1)

	sessions := []struct {
		name  string
		open  string
		close string
		time  time.Time
	}{
		{shared.Asia, shared.AsiaOpen, shared.AsiaClose, yesterday},
		{shared.London, shared.LondonOpen, shared.LondonClose, now},
		{shared.NewYork, shared.NewYorkOpen, shared.NewYorkClose, now},
		{shared.Asia, shared.AsiaOpen, shared.AsiaClose, now},
	}

	for _, sess := range sessions {
		session, err := shared.NewSession(sess.name, sess.open, sess.close, sess.time)
		if err != nil {
			return fmt.Errorf("creating %s session: %w", sess.name, err)
		}

		if !s.Exists(session.Name, session.Open) {
			s.Add(session)
		}
	}

	return nil
}

// GenerateNewSessionJob is a job used to generate new sessions.
//
// This job should be scheduled for periodic execution.
func (s *SessionSnapshot) GenerateNewSessionsJob(logger *zerolog.Logger) {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		logger.Error().Msgf("fetching new york time: %v", err)
		return
	}

	err = s.GenerateNewSessions(now)
	if err != nil {
		logger.Error().Msgf("generating new sessions: %v", err)
		return
	}
}

// setCurrentSession sets the current session.
func (s *SessionSnapshot) SetCurrentSession(now time.Time) (bool, error) {
	// Set the current session.
	var set bool
	var changed bool
	prev := s.current.Load()
	for i := range s.count.Load() {
		idx := (s.start.Load() + i) % s.size.Load()
		session := s.data[idx]
		if session.IsCurrentSession(now) {
			set = true
			if prev != idx {
				// The changed flag indicates there has been a session change.
				changed = true
				s.current.Store(idx)
			}
			break
		}
	}

	// If the current session is not set then the market is closed and current time is
	// approaching the asian session. Preemptively set the asian session.
	if !set {
		for i := range s.count.Load() {
			idx := (s.start.Load() + s.count.Load() - 1 - i + s.size.Load()) % s.size.Load()
			session := s.data[idx]
			if session.Name == shared.Asia && now.Before(session.Open) {
				if prev != idx {
					// The changed flag indicates there has been a session change.
					changed = true
					s.current.Store(idx)
				}
				break
			}
		}
	}

	return changed, nil
}

// FetchCurrentSession returns the current market session.
func (s *SessionSnapshot) FetchCurrentSession() *shared.Session {
	return s.data[s.current.Load()]
}

// FetchLastSessionOpen returns the last session open.
func (s *SessionSnapshot) FetchLastSessionOpen() (time.Time, error) {
	var open time.Time
	if s.count.Load() > 0 {
		if s.current.Load() == s.start.Load() {
			// There is no last session, set the open to the current one.
			open = s.data[s.current.Load()].Open
			return open, nil
		}

		previous := (s.current.Load() - 1 + s.size.Load()) % s.size.Load()
		open = s.data[previous].Open
		return open, nil
	}

	return open, fmt.Errorf("session snapshot has no elements")
}

// fetchLastSessionHighLow fetches newly generated levels from the previously completed session.
func (s *SessionSnapshot) FetchLastSessionHighLow() (float64, float64, error) {
	var high, low float64
	if s.count.Load() > 0 {
		if s.current.Load() == s.start.Load() {
			// There is no previous completed session.
			return 0, 0, fmt.Errorf("no completed previous session available")
		}

		previous := (s.current.Load() - 1 + s.size.Load()) % s.size.Load()
		high = s.data[previous].High
		low = s.data[previous].Low
		return high, low, nil
	}

	return 0, 0, fmt.Errorf("session snapshot has no elements")
}
