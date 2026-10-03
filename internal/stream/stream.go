// Package stream holds what the streaming parse entry points share: stopping
// on a handler error or a done context.
package stream

import (
	"context"
	"fmt"
	"io"
)

// pollEvery is how many Tick calls pass between context checks: ctx.Err takes
// a lock, and a parse emits millions of statements.
const pollEvery = 1024

// Stopper records why a parse must stop. The zero value never stops on its
// own. It is not safe for concurrent use; a parse runs on one goroutine.
type Stopper struct {
	ctx context.Context
	n   int
	err error
}

// New returns a Stopper polling ctx. A context that can never be done
// (nil, context.Background) is not polled at all.
func New(ctx context.Context) *Stopper {
	if ctx != nil && ctx.Done() == nil {
		ctx = nil
	}
	return &Stopper{ctx: ctx}
}

// Err is the reason to stop, or nil.
func (s *Stopper) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}

// Fail records err as the reason to stop, unless one is already recorded.
func (s *Stopper) Fail(err error) {
	if s != nil && s.err == nil && err != nil {
		s.err = err
	}
}

// Tick counts one emitted statement and polls the context every pollEvery
// calls. It returns the reason to stop, if any.
func (s *Stopper) Tick() error {
	if s == nil {
		return nil
	}
	if s.err == nil && s.ctx != nil {
		s.n++
		if s.n >= pollEvery {
			s.n = 0
			s.Poll()
		}
	}
	return s.err
}

// Poll checks the context now.
func (s *Stopper) Poll() error {
	if s == nil {
		return nil
	}
	if s.err == nil && s.ctx != nil {
		if err := s.ctx.Err(); err != nil {
			s.err = fmt.Errorf("parse stopped: %w", err)
		}
	}
	return s.err
}

// Reader returns r, made to fail with the recorded reason once there is one,
// so a decoder that reads ahead stops at its next read instead of consuming
// the rest of the input.
func (s *Stopper) Reader(r io.Reader) io.Reader {
	return &reader{r: r, s: s}
}

type reader struct {
	r io.Reader
	s *Stopper
}

func (r *reader) Read(b []byte) (int, error) {
	if err := r.s.Poll(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}
