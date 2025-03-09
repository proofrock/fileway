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

	"github.com/proofrock/fileway/utils"
)

const (
	expiryMillis = 4 * 60 * 1000 // cleanup unused/stale sessions, not accessed for > 4 minutes
)

type ConduitSet struct {
	conduits map[string]*Conduit
	mu       sync.RWMutex
}

func (cs *ConduitSet) cleanup(id string) {
	panic("unimplemented")
}

func NewConduitSet() *ConduitSet {
	// Create a new ConduitSet instance
	ret := &ConduitSet{
		conduits: make(map[string]*Conduit),
	}

	// Setup periodic cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			ret.cleanupStaleConduits()
		}
	}()

	return ret
}

func (cs *ConduitSet) cleanupStaleConduits() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cutoffTime := time.Now().UnixMilli() - expiryMillis
	i := 0
	for id, conduit := range cs.conduits {
		if conduit.WasAccessedBefore(cutoffTime) {
			i++
			delete(cs.conduits, id)
		}
	}
	if i > 0 {
		fmt.Printf("%d sessions were garbage collected\n", i)
	}
}

func (cs *ConduitSet) NewConduit(
	forcedId string,
	isText bool,
	filename string,
	size int64,
	secret string,
	chunkSize, bufferQueueSize, idsLength int,
) (string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// an id can be provided, and if it's not, it's random
	var id string
	if forcedId != "" {
		// check if the forced id already exists
		if _, exists := cs.conduits[forcedId]; exists {
			return "", fmt.Errorf("conduit with id %s already exists", forcedId)
		}
		id = forcedId
	} else {
		// generate a random id that doesn't already exist
		for {
			id = utils.GenRandomString(idsLength)
			if _, exists := cs.conduits[id]; !exists {
				break
			}
		}
	}

	// Create a new Conduit instance
	conduit := newConduit(id, isText, filename, size, secret, chunkSize, bufferQueueSize)

	cs.conduits[conduit.Id] = conduit

	return id, nil
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
