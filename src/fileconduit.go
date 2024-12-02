package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	IdsLength      = 33       // Length of ID random strings, amounts to 192 bit
	ChunkSize      = 16777216 // 16Mb
	ChunkQueueSize = 4        // 64Mb total memory
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
		ChunkQueue: make(chan []byte, ChunkQueueSize),
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
