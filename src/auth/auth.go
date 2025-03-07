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
	secretHashes [][]byte
	passwords    map[string]bool // cache
	mu           sync.Mutex
}

func NewAuth(envvar string) *Auth {
	ret := &Auth{
		secretHashes: make([][]byte, 0),
		passwords:    make(map[string]bool, 0),
	}

	for _, s := range strings.Split(envvar, ",") {
		ret.secretHashes = append(ret.secretHashes, []byte(s))
	}

	return ret
}

func (a *Auth) Authenticate(pwd string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.passwords[pwd]; ok {
		return true
	}
	for _, hash := range a.secretHashes {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(pwd)); err == nil {
			a.passwords[pwd] = true
			return true
		}
	}
	return false
}
