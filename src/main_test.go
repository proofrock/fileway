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
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/proofrock/fileway/auth"
	fw "github.com/proofrock/fileway/fileway_logic"
)

// A bcrypt hash of "mysecret", same as the one used by the bats suite.
const testSecretHash = `$2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa`

func setupTestServer() {
	authenticator = auth.NewAuth(testSecretHash)
	conduits = fw.NewConduitSet(3600)
}

const maxSize = 4 * 1024 * 1024 * 1024 * 1024

// Sizes outside the supported range are refused at setup time.
func TestSetupRejectsInvalidSizes(t *testing.T) {
	setupTestServer()

	cases := []struct {
		size string
		want int
	}{
		{"0", http.StatusBadRequest},
		{"-1", http.StatusBadRequest},
		{"abc", http.StatusBadRequest},
		{"1", http.StatusOK},
		{strconv.FormatInt(maxSize, 10), http.StatusOK},
		{strconv.FormatInt(maxSize+1, 10), http.StatusBadRequest},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/setup?filename=a.bin&txt=0&size="+c.size, nil)
		r.Header.Set("x-fileway-secret", "mysecret")
		w := httptest.NewRecorder()
		setup(w, r)
		if w.Code != c.want {
			t.Errorf("size=%s -> HTTP %d, want %d", c.size, w.Code, c.want)
		}
	}
}
