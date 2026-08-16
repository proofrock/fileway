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
	"sync"
	"testing"
)

func TestBuildChunkPlan(t *testing.T) {
	cases := []struct {
		size      int64
		chunkSize int
	}{
		{4095, 4096},
		{4096, 4096},
		{4097, 4096},
		{12288, 4096},
		{1, 4096},
	}
	for _, c := range cases {
		plan := buildChunkPlan(c.size, c.chunkSize)
		// no zero chunks
		for i, ch := range plan {
			if ch == 0 {
				t.Errorf("size=%d chunkSize=%d: chunk[%d] is zero", c.size, c.chunkSize, i)
			}
		}
		// sum equals size
		sum := int64(0)
		for _, ch := range plan {
			sum += int64(ch)
		}
		if sum != c.size {
			t.Errorf("size=%d chunkSize=%d: plan sum=%d", c.size, c.chunkSize, sum)
		}
	}
}

func TestDownloadRace(t *testing.T) {
	const rounds = 20000
	const goroutines = 4

	for round := 0; round < rounds; round++ {
		c := newConduit(false, "f.bin", 4096, "s", 4096, 4, 16)

		var wg sync.WaitGroup
		admitted := 0
		var mu sync.Mutex

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if c.Download() == nil {
					mu.Lock()
					admitted++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		if admitted != 1 {
			t.Fatalf("round %d: %d goroutines passed Download(), I want exactly 1", round, admitted)
		}
	}
}

// Two concurrent uploads must never be handed the same entry of the plan:
// the second would be size-checked against the first chunk's budget.
func TestClaimNextChunkIsAtomic(t *testing.T) {
	const rounds = 5000
	for round := 0; round < rounds; round++ {
		// A large chunkSize keeps the ramp going, so the first claims return
		// distinct sizes (4096, 8192, 16384, 32768) and a double claim shows up.
		c := newConduit(false, "f.bin", 1000000, "s", 4096*1024, 4, 8)

		var wg sync.WaitGroup
		var mu sync.Mutex
		seen := map[int]int{}
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got := c.ClaimNextChunk()
				mu.Lock()
				seen[got]++
				mu.Unlock()
			}()
		}
		wg.Wait()

		for size, n := range seen {
			if n != 1 {
				t.Fatalf("round %d: chunk size %d claimed %d times", round, size, n)
			}
		}
	}
}
