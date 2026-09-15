package endpoint

import (
	"context"
	"sync"
)

// rwLock is a readers-writer lock whose acquisition gives up when a context is
// done, so that a query waiting behind a long update still honours its
// timeout and its client's disconnect. sync.RWMutex cannot do that.
//
// Writers are preferred: once a writer waits, new readers wait too, so a
// steady stream of queries cannot starve an update. Readers already inside
// finish first.
//
// It is safe for concurrent use.
type rwLock struct {
	mu             sync.Mutex
	readers        int
	writer         bool
	writersWaiting int
	// changed is closed and replaced whenever the state changes in a way that
	// can let a waiter in.
	changed chan struct{}
}

func newRWLock() *rwLock { return &rwLock{changed: make(chan struct{})} }

// broadcast wakes every waiter. l.mu must be held.
func (l *rwLock) broadcast() {
	close(l.changed)
	l.changed = make(chan struct{})
}

// rlock takes a read lock, or returns ctx.Err() once ctx is done.
func (l *rwLock) rlock(ctx context.Context) error {
	for {
		l.mu.Lock()
		if !l.writer && l.writersWaiting == 0 {
			l.readers++
			l.mu.Unlock()
			return nil
		}
		ch := l.changed
		l.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (l *rwLock) runlock() {
	l.mu.Lock()
	l.readers--
	if l.readers < 0 {
		l.mu.Unlock()
		panic("endpoint: runlock of unlocked rwLock")
	}
	if l.readers == 0 {
		l.broadcast()
	}
	l.mu.Unlock()
}

// lock takes the write lock, or returns ctx.Err() once ctx is done.
func (l *rwLock) lock(ctx context.Context) error {
	l.mu.Lock()
	l.writersWaiting++
	for l.writer || l.readers > 0 {
		ch := l.changed
		l.mu.Unlock()
		select {
		case <-ch:
			l.mu.Lock()
		case <-ctx.Done():
			l.mu.Lock()
			l.writersWaiting--
			// Readers held back by this writer may go now.
			l.broadcast()
			l.mu.Unlock()
			return ctx.Err()
		}
	}
	l.writersWaiting--
	l.writer = true
	l.mu.Unlock()
	return nil
}

func (l *rwLock) unlock() {
	l.mu.Lock()
	if !l.writer {
		l.mu.Unlock()
		panic("endpoint: unlock of unlocked rwLock")
	}
	l.writer = false
	l.broadcast()
	l.mu.Unlock()
}
