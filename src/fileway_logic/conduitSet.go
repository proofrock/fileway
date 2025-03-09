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

package fileway

import (
	"fmt"
	"sync"
	"time"
)

type ConduitSet struct {
	conduits     map[string]*Conduit
	expiryMillis int64
	mu           sync.RWMutex
}

func NewConduitSet(
	expirySeconds int,
) *ConduitSet {
	// Create a new ConduitSet instance
	ret := &ConduitSet{
		conduits:     make(map[string]*Conduit),
		expiryMillis: int64(expirySeconds) * 1000,
	}

	// Setup periodic cleanup
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			ret.cleanupStaleConduits()
		}
	}()

	return ret
}

func (cs *ConduitSet) cleanupStaleConduits() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cutoffTime := time.Now().UnixMilli() - cs.expiryMillis
	i := 0
	for id, conduit := range cs.conduits {
		if !conduit.WasAccessedAfter(cutoffTime) {
			i++
			delete(cs.conduits, id)
			conduit.Latch.Unlock() // Unlock the latch, so that any waiting upload can fail
		}
	}
	if i > 0 {
		fmt.Printf("%d sessions were garbage collected\n", i)
	}
}

func (cs *ConduitSet) NewConduit(isText bool,
	filename string,
	size int64,
	secret string,
	chunkSize, bufferQueueSize, idsLength int) string {
	// Create a new Conduit instance
	conduit := newConduit(isText, filename, size, secret, chunkSize, bufferQueueSize, idsLength)
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.conduits[conduit.Id] = conduit

	return conduit.Id
}

func (cs *ConduitSet) GetConduit(conduitId string) *Conduit {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.conduits[conduitId]
}

func (cs *ConduitSet) DelConduit(conduitId string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.conduits, conduitId)
}
