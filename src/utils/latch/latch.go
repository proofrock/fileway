// Copyright 2024 @proofrock
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package latch

import (
	"sync"
	"time"
)

/*
Package latch implements a thread-safe latch mechanism with timeout functionality.

The Latch type provides a synchronization primitive that allows one or more
goroutines to wait until it is unlocked by another goroutine. This is similar
to a "countdown latch" or "gate" in other concurrent programming models.

Key features:
- Thread-safe operation using mutex protection
- Blocking wait with configurable timeout
- One-time unlocking (subsequent unlock calls have no effect)
- Efficient channel-based signaling mechanism

Example usage:

	latch := NewLatch()

	// In one goroutine
	go func() {
	    if latch.Wait(5 * time.Second) {
	        // Latch was unlocked
	        doSomething()
	    } else {
	        // Wait timed out
	        handleTimeout()
	    }
	}()

	// In another goroutine
	go func() {
	    // Do some work
	    prepareData()

	    // Signal waiters to proceed
	    latch.Unlock()
	}()
*/
type Latch struct {
	mu       sync.Mutex
	ch       chan struct{}
	unlocked bool
}

// NewLatch creates a new Latch that starts locked.
func NewLatch() *Latch {
	return &Latch{
		ch: make(chan struct{}),
	}
}

// Unlock releases the latch, allowing waiters to proceed.
func (l *Latch) Unlock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.unlocked {
		close(l.ch)
		l.unlocked = true
	}
}

// Wait blocks until the latch is unlocked or the timeout elapses.
func (l *Latch) Wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-l.ch:
		return true // Unlocked
	case <-timer.C:
		return false // Timed out
	}
}
