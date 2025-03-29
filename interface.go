package main

import "context"

// PositionStorer defines the requirements for storing positions.
type PositionStorer interface {
	PersistClosedPosition(ctx context.Context, position *Position) error
}
