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
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Conduit struct {
	secret    string
	Id        string
	Filename  string
	Size      int64
	ChunkPlan []int

	ChunkQueue chan []byte

	lastAccessed    atomic.Int64
	downloadStarted atomic.Bool
	latch           *Latch
}

func NewConduit(filename string, size int64, secret string) *Conduit {
	ret := &Conduit{
		Id:         genRandomString(idsLength),
		Filename:   filename,
		Size:       size,
		secret:     secret,
		ChunkQueue: make(chan []byte, bufferQueueSize),
		latch:      NewLatch(),
	}

	ret.ChunkPlan = buildChunkPlan(size)

	ret.touch()
	return ret
}

func buildChunkPlan(size int64) []int {
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

func (c *Conduit) IsUploadSecretWrong(candidate string) bool {
	return c.secret != candidate
}

func (c *Conduit) touch() {
	c.lastAccessed.Store(time.Now().UnixMilli())
}

func (c *Conduit) WasAccessedBefore(cutoffTime int64) bool {
	return c.lastAccessed.Load() < cutoffTime
}

func (c *Conduit) Download() error {
	if c.downloadStarted.Load() {
		return ErrConduitAlreadyDownloading
	}

	c.touch()
	c.downloadStarted.Store(true)
	c.latch.Unlock()

	return nil
}

func (c *Conduit) IsDownloading() bool {
	c.touch()
	return c.downloadStarted.Load()
}

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
