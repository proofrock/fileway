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

package main

import (
	"sync"
	"time"
)

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
	select {
	case <-l.ch:
		return true // Unlocked
	case <-time.After(timeout):
		return false // Timed out
	}
}
