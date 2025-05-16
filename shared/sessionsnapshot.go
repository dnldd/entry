package shared

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

const (
	// SessionGenerationTime is the time new sessions are generated to cover the day.
	SessionGenerationTime = "00:01"
	// SessionSnapshotSize is the maximum number of entries for a session snapshot.
	SessionSnapshotSize = 28
)

// CandlestickSnapshot represents a snapshot of session data.
type SessionSnapshot struct {
	data    []*Session
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
		data: make([]*Session, size),
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
func (s *SessionSnapshot) Add(session *Session) {
	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	end := (start + count) % size
	s.data[end] = session

	if count == size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start.Store((start + 1) % size)
	} else {
		s.count.Add(1)
	}
}

// Exists checks whether the snapshot has an existing session corresponding to
// the provided name and opening day.
func (s *SessionSnapshot) Exists(name string, open time.Time) bool {
	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	for i := count - 1; i >= 0; i-- {
		idx := (start + i) % size
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
		{Asia, AsiaOpen, AsiaClose, yesterday},
		{London, LondonOpen, LondonClose, now},
		{NewYork, NewYorkOpen, NewYorkClose, now},
		{Asia, AsiaOpen, AsiaClose, now},
	}

	for _, sess := range sessions {
		session, err := NewSession(sess.name, sess.open, sess.close, sess.time)
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
	now, _, err := NewYorkTime()
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
	start := s.start.Load()
	count := s.count.Load()
	size := s.size.Load()
	for i := range count {
		idx := (start + i) % size
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
		start := s.start.Load()
		count := s.count.Load()
		size := s.size.Load()
		for i := range count {
			idx := (start + count - 1 - i + size) % size
			session := s.data[idx]
			if session.Name == Asia && now.Before(session.Open) {
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
func (s *SessionSnapshot) FetchCurrentSession() *Session {
	return s.data[s.current.Load()]
}

// FetchLastSessionOpen returns the last session open.
func (s *SessionSnapshot) FetchLastSessionOpen() (time.Time, error) {
	var open time.Time
	count := s.count.Load()
	if count > 0 {
		start := s.start.Load()
		size := s.size.Load()
		current := s.current.Load()
		if current == start {
			// There is no last session, set the open to the current one.
			open = s.data[current].Open
			return open, nil
		}

		previous := (current - 1 + size) % size
		open = s.data[previous].Open
		return open, nil
	}

	return open, fmt.Errorf("session snapshot has no elements")
}

// fetchLastSessionHighLow fetches newly generated levels from the previously completed session.
func (s *SessionSnapshot) FetchLastSessionHighLow() (float64, float64, error) {
	var high, low float64
	count := s.count.Load()
	if count > 0 {
		current := s.current.Load()
		start := s.start.Load()
		size := s.size.Load()
		if current == start {
			// There is no previous completed session.
			return 0, 0, fmt.Errorf("no completed previous session available")
		}

		previous := (current - 1 + size) % size
		high = s.data[previous].High.Load()
		low = s.data[previous].Low.Load()

		if high == 29 {
			fmt.Println("high is 29")
		}

		return high, low, nil
	}

	return 0, 0, fmt.Errorf("session snapshot has no elements")
}
