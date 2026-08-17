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

package auth

import (
	"sync"
	"testing"
	"time"
)

// bcrypt hashes of "mysecret" and "other", cost 10.
const (
	hashMysecret = `$2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa`
	hashOther    = `$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy`
)

func TestAuthenticate(t *testing.T) {
	a := NewAuth(hashMysecret)
	if !a.Authenticate("mysecret") {
		t.Error("correct secret rejected")
	}
	if !a.Authenticate("mysecret") {
		t.Error("correct secret rejected on the cached path")
	}
	if a.Authenticate("wrong") {
		t.Error("wrong secret accepted")
	}
	if a.Authenticate("") {
		t.Error("empty secret accepted")
	}
}

// Hashes are comma separated; surrounding whitespace is easy to introduce in a
// docker-compose file and used to make the hash silently unusable.
func TestNewAuthTrimsWhitespace(t *testing.T) {
	a := NewAuth("  " + hashMysecret + " ,\t" + hashOther + "\n")
	if len(a.secretHashes) != 2 {
		t.Fatalf("got %d hashes, want 2", len(a.secretHashes))
	}
	if !a.Authenticate("mysecret") {
		t.Error("secret rejected because its hash carried whitespace")
	}
}

func TestNewAuthSkipsEmptyEntries(t *testing.T) {
	a := NewAuth(hashMysecret + ",,  ,")
	if len(a.secretHashes) != 1 {
		t.Errorf("got %d hashes, want 1", len(a.secretHashes))
	}
}

// A wrong secret costs a full bcrypt round and is never cached. Holding the
// lock across that computation lets a burst of bad attempts stall a legitimate
// user whose secret is already cached and needs only a map lookup.
func TestWrongSecretsDoNotStallCachedUser(t *testing.T) {
	a := NewAuth(hashMysecret)
	a.Authenticate("mysecret") // warm the cache

	const attackers = 30
	var wg sync.WaitGroup
	for i := 0; i < attackers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			a.Authenticate("wrong-secret")
		}(i)
	}

	time.Sleep(5 * time.Millisecond) // let the burst pile up
	start := time.Now()
	a.Authenticate("mysecret")
	elapsed := time.Since(start)
	wg.Wait()

	// One bcrypt round at cost 10 is tens of milliseconds; a cached lookup that
	// has to queue behind 30 of them takes well over a second.
	if elapsed > 100*time.Millisecond {
		t.Errorf("cached authentication took %v while %d wrong secrets were verified", elapsed, attackers)
	}
}
