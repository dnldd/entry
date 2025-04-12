package market

import (
	"fmt"
	"time"

	"github.com/dnldd/entry/shared"
)

// CandlestickSnapshot represents a snapshot of session data.
type SessionSnapshot struct {
	data    []*shared.Session
	start   int
	current int
	count   int
	size    int
}

// NewSessionSnapshot initializes a new session snapshot.
func NewSessionSnapshot(size int) (*SessionSnapshot, error) {
	snapshot := &SessionSnapshot{
		data: make([]*shared.Session, size),
		size: size,
	}

	err := snapshot.AddSessions()
	if err != nil {
		return nil, fmt.Errorf("adding sessions to snapshot: %v", err)
	}

	_, err = snapshot.SetCurrentSession()
	if err != nil {
		return nil, fmt.Errorf("setting current session: %v", err)
	}

	return snapshot, nil
}

// Adds adds the provided session to the snapshot.
func (s *SessionSnapshot) Add(session *shared.Session) {
	end := (s.start + s.count) % s.size
	s.data[end] = session

	if s.count == s.size {
		// Overwrite the oldest entry when the snapshot is at capacity.
		s.start = (s.start + 1) % s.size
	} else {
		s.count++
	}
}

// Add sessions adds a new set of sessions (london, new york, asia - covering a day) to the snapshot.
func (s *SessionSnapshot) AddSessions() error {
	londonSession, err := shared.NewSession(shared.London, shared.LondonOpen, shared.LondonClose)
	if err != nil {
		return fmt.Errorf("creating london session: %w", err)
	}

	s.Add(londonSession)

	newYorkSession, err := shared.NewSession(shared.NewYork, shared.NewYorkOpen, shared.NewYorkClose)
	if err != nil {
		return fmt.Errorf("creating new york session: %w", err)
	}

	s.Add(newYorkSession)

	asianSession, err := shared.NewSession(shared.Asia, shared.AsiaOpen, shared.AsiaClose)
	if err != nil {
		return fmt.Errorf("creating asian session: %w", err)
	}

	s.Add(asianSession)

	return nil
}

// setCurrentSession sets the current session.
func (s *SessionSnapshot) SetCurrentSession() (bool, error) {
	now, _, err := shared.NewYorkTime()
	if err != nil {
		return false, err
	}

	// Set the current session.
	var set bool
	var changed bool
	prev := s.current
	for i := range s.count {
		idx := (s.start + i) % s.size
		session := s.data[idx]
		if session.IsCurrentSession(now) {
			set = true
			if prev != idx {
				// The changed flag indicates there has been a session change.
				changed = true
				s.current = idx
			}
			break
		}
	}

	// If the current session is not set then the market is closed and current time is
	// approaching the asian session. Preemptively set the asian session.
	if !set {
		for i := range s.count {
			idx := (s.start + s.count - 1 - i + s.size) % s.size
			session := s.data[idx]
			if session.Name == shared.Asia {
				s.current = idx
				break
			}
		}
	}

	return changed, nil
}

// FetchCurrentSession returns the current market session.
func (s *SessionSnapshot) FetchCurrentSession() *shared.Session {
	return s.data[s.current]
}

// FetchLastSessionOpen returns the last session open.
func (s *SessionSnapshot) FetchLastSessionOpen() (time.Time, error) {
	var open time.Time
	if s.count > 0 {
		if s.current == s.start {
			// There is no last session, set the open to the current one.
			open = s.data[s.current].Open
			return open, nil
		}

		previous := (s.current - 1 + s.size) % s.size
		open = s.data[previous].Open
		return open, nil
	}

	return open, fmt.Errorf("session snapshot has no elements")
}

// fetchLastSessionHighLow fetches newly generated levels from the previously completed session.
func (s *SessionSnapshot) FetchLastSessionHighLow() (float64, float64, error) {
	var high, low float64
	if s.count > 0 {
		if s.current == s.start {
			// There is no previous completed session.
			return 0, 0, fmt.Errorf("no completed previous session available")
		}

		previous := (s.current - 1 + s.size) % s.size
		high = s.data[previous].High
		low = s.data[previous].Low
		return high, low, nil
	}

	return 0, 0, fmt.Errorf("session snapshot has no elements")
}
