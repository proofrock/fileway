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
	"strings"
	"testing"
	"time"

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

// A chunk larger than the plan allows must be refused, not buffered.
func TestUploadRejectsOversizedChunk(t *testing.T) {
	setupTestServer()

	id := conduits.NewConduit(false, "a.bin", 5, "mysecret", 4096, 4, 16)
	body := strings.Repeat("X", 5000)

	r := httptest.NewRequest("PUT", "/ul/"+id, strings.NewReader(body))
	r.Header.Set("x-fileway-secret", "mysecret")
	w := httptest.NewRecorder()
	ul(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("oversized chunk -> HTTP %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// A conduit that expires while chunks are still buffered must still deliver
// them: the downloader was promised Content-Length bytes and silently getting
// fewer corrupts the file.
func TestDownloadDeliversBufferedChunksOnExpiry(t *testing.T) {
	setupTestServer()

	const rounds = 200
	truncated := 0
	for i := 0; i < rounds; i++ {
		id := conduits.NewConduit(false, "a.bin", 12, "mysecret", 4096, 4, 16)
		conduit := conduits.GetConduit(id)

		// The uploader delivered everything and went away; the chunks sit in
		// the buffer waiting for a slow downloader.
		conduit.ChunkQueue <- []byte("aaaa")
		conduit.ChunkQueue <- []byte("bbbb")
		conduit.ChunkQueue <- []byte("cccc")

		// The cleanup ticker fires right now.
		conduit.Expire()

		// ddl() claims the download itself, so it must not be claimed here.
		r := httptest.NewRequest("GET", "/ddl/"+id, nil)
		w := httptest.NewRecorder()
		ddl(w, r)

		if w.Body.Len() != 12 {
			truncated++
		}
	}

	if truncated > 0 {
		t.Errorf("%d/%d downloads were truncated despite the chunks being buffered", truncated, rounds)
	}
}

// Streaming a chunk must keep the conduit alive, otherwise a download slower
// than UPLOAD_TIMEOUT_SECS is killed by the cleanup ticker halfway through.
func TestDownloadKeepsConduitAlive(t *testing.T) {
	setupTestServer()

	id := conduits.NewConduit(false, "a.bin", 8, "mysecret", 4096, 4, 16)
	conduit := conduits.GetConduit(id)

	// ddl() claims the download itself, so it must not be claimed here.
	finished := make(chan struct{})
	go func() {
		r := httptest.NewRequest("GET", "/ddl/"+id, nil)
		ddl(httptest.NewRecorder(), r)
		close(finished)
	}()

	// Let ddl() get past Download(), which touches the conduit on its own: the
	// cutoff has to sit after that, or it would measure the wrong touch.
	time.Sleep(30 * time.Millisecond)
	cutoff := time.Now().UnixMilli()
	time.Sleep(5 * time.Millisecond)

	conduit.ChunkQueue <- []byte("aaaa")
	time.Sleep(30 * time.Millisecond)

	alive := conduit.WasAccessedAfter(cutoff)
	conduit.ChunkQueue <- []byte("bbbb")
	<-finished

	if !alive {
		t.Error("streaming a chunk did not touch the conduit; a slow transfer would expire mid-flight")
	}
}

// An expired conduit must be reported as 410 by /ul/ too, not as a 408 that
// clients would read as a transient stall.
func TestUploadOnExpiredConduitIsGone(t *testing.T) {
	setupTestServer()

	id := conduits.NewConduit(false, "a.bin", 8, "mysecret", 4096, 1, 16)
	conduit := conduits.GetConduit(id)
	conduit.ChunkQueue <- []byte("full") // fill the queue so Offer() must block
	conduit.Expire()

	r := httptest.NewRequest("PUT", "/ul/"+id, strings.NewReader("aaaa"))
	r.Header.Set("x-fileway-secret", "mysecret")
	w := httptest.NewRecorder()
	ul(w, r)

	if w.Code != http.StatusGone {
		t.Errorf("ul on expired conduit -> HTTP %d, want %d", w.Code, http.StatusGone)
	}
}
