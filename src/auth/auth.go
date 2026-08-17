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
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	// Written once by NewAuth before any concurrent use, then read-only.
	secretHashes [][]byte

	// Cache of secrets already verified against a hash. Only successes are
	// stored, so it is bounded by the number of configured secrets.
	passwords map[string]bool
	mu        sync.RWMutex
}

func NewAuth(envvar string) *Auth {
	ret := &Auth{
		secretHashes: make([][]byte, 0),
		passwords:    make(map[string]bool),
	}

	for _, s := range strings.Split(envvar, ",") {
		// Whitespace around a comma-separated hash is easy to introduce in a
		// compose file and would otherwise make the hash silently unusable.
		if s = strings.TrimSpace(s); s != "" {
			ret.secretHashes = append(ret.secretHashes, []byte(s))
		}
	}

	return ret
}

func (a *Auth) Authenticate(pwd string) bool {
	a.mu.RLock()
	cached := a.passwords[pwd]
	a.mu.RUnlock()
	if cached {
		return true
	}

	// bcrypt is deliberately expensive, so it runs outside the lock: holding it
	// here would serialize every authentication behind the slowest one, and a
	// burst of wrong secrets (never cached, so always paying full price) would
	// stall users whose secret is already cached.
	for _, hash := range a.secretHashes {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(pwd)); err == nil {
			a.mu.Lock()
			a.passwords[pwd] = true
			a.mu.Unlock()
			return true
		}
	}
	return false
}
