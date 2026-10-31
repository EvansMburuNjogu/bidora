// Package health is independent of HTTP and makes no business-readiness claim.
package health

import "sync/atomic"

type State struct{ ready atomic.Bool }

func (s *State) Ready() bool { return s.ready.Load() }
func (s *State) Start()      { s.ready.Store(true) }
func (s *State) Drain()      { s.ready.Store(false) }
