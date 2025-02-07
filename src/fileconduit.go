// Copyright (C) 2024- Germano Rizzo
//
// This file is part of fileway.
//
// fileway is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// fileway is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with fileway.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	IdsLength = 33      // Length of ID random strings, amounts to 192 bit
	ChunkSize = 4194304 // 16Mb
)

type Conduit struct {
	secret          string
	Id              string
	Filename        string
	Size            int64
	ChunkQueue      chan []byte
	lastAccessed    atomic.Int64
	downloadStarted atomic.Bool
	mu              sync.Mutex
}

func NewConduit(filename string, size int64, secret string) *Conduit {
	ret := &Conduit{
		Id:         genRandomString(IdsLength),
		Filename:   filename,
		Size:       size,
		secret:     secret,
		ChunkQueue: make(chan []byte, 1),
		mu:         sync.Mutex{},
	}
	ret.touch()
	return ret
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.downloadStarted.Load() {
		return ErrConduitAlreadyDownloading
	}

	c.touch()
	c.downloadStarted.Store(true)

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
