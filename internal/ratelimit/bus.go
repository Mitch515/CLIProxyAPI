package ratelimit

import (
	"sync/atomic"
	"time"
)

// Sink receives an observation. The function should return quickly; the
// service implementation buffers via a channel.
type Sink func(Observation)

var sinkValue atomic.Pointer[Sink]

// SetSink registers (or replaces) the global sink. Pass nil to detach.
//
// Called once at process startup by the service coordinator. The publisher
// helper (in internal/runtime/executor) calls Submit on every parsed
// observation and we rely on this sink to forward into the persistent store
// and the live event bus.
func SetSink(s Sink) {
	if s == nil {
		sinkValue.Store(nil)
		return
	}
	sinkValue.Store(&s)
}

// Submit pushes one observation to the registered sink. It is safe to call
// before SetSink has been invoked: in that case the observation is dropped.
//
// ObservedAt is back-filled to time.Now() if zero, so callers can leave it
// unset for convenience.
func Submit(o Observation) {
	ptr := sinkValue.Load()
	if ptr == nil {
		return
	}
	if o.ObservedAt.IsZero() {
		o.ObservedAt = time.Now().UTC()
	}
	(*ptr)(o)
}

// HasSink reports whether a sink is currently registered.
func HasSink() bool {
	return sinkValue.Load() != nil
}
