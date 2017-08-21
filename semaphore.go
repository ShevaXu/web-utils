package utils

import (
	"context"
)

// Semaphore is bounded resources abstraction.
// Ref: https://github.com/golang/go/wiki/BoundingResourceUse
type Semaphore interface {
	// Obtain puts one into the semaphore, returns true if succeeds.
	// It blocks utils succeeds or the context cancelled.
	// Obtaining from a closed semaphore should return false.
	Obtain(context.Context) bool

	// Release takes one from the semaphore, returns true if succeeds.
	// It should never blocks.
	Release() bool

	// Capacity returns semaphore's max concurrent resources.
	Capacity() int

	// Count returns semaphore's current used resources.
	Count() int

	// Close stops obtaining resources from semaphore,
	// it makes Obtain() return false ever since.
	Close()

	// Closed tells if semaphore is closed.
	Closed() bool
}

// semaphore implements Semaphore using a buffered channel.
// It works like this:
// Release() <- Semaphore (buffered channel) <- Obtain()
type semaphore struct {
	sem    chan struct{}
	closed bool
}

func (s *semaphore) Obtain(ctx context.Context) bool {
	// never obtain from a closed semaphore
	if s.closed {
		return false
	}

	done := ctx.Done()
	select {
	case s.sem <- struct{}{}:
		return true
	case <-done:
		return false
	}
}

func (s *semaphore) Release() bool {
	select {
	case <-s.sem:
		return true
	default:
		// nothing queued
		return false
	}
}

func (s *semaphore) Capacity() int {
	return cap(s.sem)
}

func (s *semaphore) Count() int {
	return len(s.sem)
}

func (s *semaphore) Close() {
	// once closed, cannot be un-done
	s.closed = true
}

func (s *semaphore) Closed() bool {
	return s.closed
}

// NewSemaphore returns an internal semaphore.
// This is the exported interface for using semaphore.
func NewSemaphore(n int) Semaphore {
	return &semaphore{
		sem:    make(chan struct{}, n),
		closed: false,
	}
}
