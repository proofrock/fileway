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
	"sync/atomic"
	"time"

	"github.com/proofrock/fileway/utils"
	"github.com/proofrock/fileway/utils/latch"
)

const (
	chunkSizeInitial    = 4096 // initially 4k
	chunkSizeRampFactor = 2    // x2 every chunk, until it reaches chunkSize
)

/*
Conduit represents a single transfer conduit, which can be used to upload or download files or text.
It is a thread-safe structure that can be accessed concurrently by multiple goroutines.
*/
type Conduit struct {
	Id       string
	IsText   bool
	Filename string
	Size     int64

	ChunkPlan []int

	ChunkQueue chan []byte

	secret string

	lastAccessed    atomic.Int64
	downloadStarted atomic.Bool
	Latch           *latch.Latch
}

// Creates a new Conduit instance
func newConduit(
	isText bool,
	filename string,
	size int64,
	secret string,
	chunkSize, bufferQueueSize, idsLength int,
) *Conduit {
	ret := &Conduit{
		Id:         utils.GenRandomString(idsLength),
		IsText:     isText,
		Filename:   filename,
		Size:       size,
		secret:     secret,
		ChunkQueue: make(chan []byte, bufferQueueSize),
		Latch:      latch.NewLatch(),
	}

	if !ret.IsText {
		ret.ChunkPlan = buildChunkPlan(size, chunkSize)
	} else {
		ret.ChunkPlan = []int{int(size)}
	}

	ret.touch()
	return ret
}

func buildChunkPlan(size int64, chunkSize int) []int {
	if size < chunkSizeInitial {
		return []int{int(size)}
	}

	sum := int64(chunkSizeInitial)
	lastChunk := chunkSizeInitial
	ret := []int{chunkSizeInitial}
	for {
		nextChunk := min(lastChunk*chunkSizeRampFactor, chunkSize, int(size-sum))
		ret = append(ret, nextChunk)
		sum += int64(nextChunk)
		if sum == size {
			return ret
		}
		lastChunk = nextChunk
	}
}

// IsUploadSecretWrong checks if the provided secret is wrong
func (c *Conduit) IsUploadSecretWrong(candidate string) bool {
	return c.secret != candidate
}

// touch updates the lastAccessed timestamp to the current time
func (c *Conduit) touch() {
	c.lastAccessed.Store(time.Now().UnixMilli())
}

// WasAccessedBefore checks if the lastAccessed timestamp is before the provided cutoff time
func (c *Conduit) WasAccessedBefore(cutoffTime int64) bool {
	return c.lastAccessed.Load() < cutoffTime
}

// Download starts the download process
func (c *Conduit) Download() error {
	if c.downloadStarted.Load() {
		return ErrConduitAlreadyDownloading
	}

	c.touch()
	c.downloadStarted.Store(true)
	c.Latch.Unlock()

	return nil
}

// Offer offers a chunk of content to the Conduit (upload)
func (c *Conduit) Offer(content []byte) error {
	c.touch()
	select {
	case c.ChunkQueue <- content:
		return nil
	case <-time.After(30 * time.Second):
		return ErrUploadTimeout
	}
}

var (
	ErrConduitAlreadyDownloading = fmt.Errorf("conduit Already Downloading or Downloaded")
	ErrUploadTimeout             = fmt.Errorf("upload timed out. Conduit seems stuck")
)
