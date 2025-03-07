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
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	conduits     = make(map[string]*Conduit)
	secretHashes = make([][]byte, 0)
	conduitsMu   sync.RWMutex
	passwords    sync.Map

	idsLength       = getIntEnv("RANDOM_IDS_LENGTH", 33)      // Length of ID random strings, amounts to 192 bit
	chunkSize       = getIntEnv("CHUNK_SIZE_KB", 4096) * 1024 // 4Mb
	bufferQueueSize = getIntEnv("BUFFER_QUEUE_SIZE", 4)       // 16Mb total
)

const (
	chunkSizeInitial    = 4096          // initially 4k
	chunkSizeRampFactor = 2             // x2 every chunk, until it reaches chunkSize
	expiryMillis        = 4 * 60 * 1000 // cleanup unused/stale sessions, not accessed for > 4 minutes
)

//go:embed static/upload.html
var uploadPage []byte

//go:embed static/download.html
var downloadPage []byte

//go:embed static/download_for_txt.html
var downloadPageForTxt []byte

//go:embed static/favicon.png
var favicon []byte

//go:embed static/fileway_ul.py
var cliUploader []byte

var version string   // Set at build time, var VERSION
var buildTime string // Set at build time, var SOURCE_DATE_EPOCH

func main() {
	// Replaces version in the web pages and cli uploader
	downloadPage = replace(downloadPage, "#VERSION#", version)
	downloadPageForTxt = replace(downloadPageForTxt, "#VERSION#", version)
	uploadPage = replace(uploadPage, "#VERSION#", version)
	cliUploader = replace(cliUploader, "#VERSION#", version)

	// https://manytools.org/hacker-tools/ascii-banner/, profile "Slant"
	fmt.Println("    _____ __")
	fmt.Println("   / __(_) /__ _      ______ ___  __")
	fmt.Println("  / /_/ / / _ \\ | /| / / __ `/ / / /")
	fmt.Println(" / __/ / /  __/ |/ |/ / /_/ / /_/ /")
	fmt.Println("/_/ /_/_/\\___/|__/|__/\\__,_/\\__, /")
	fmt.Println("                           /____/ " + version)
	fmt.Println()

	if _, isthere := os.LookupEnv("REPRODUCIBLE_BUILD_INFO"); isthere {
		fmt.Println("Variables used for this build:")
		fmt.Printf("- VERSION: '%s'\n", version)
		fmt.Printf("- SOURCE_DATE_EPOCH: '%s'\n", buildTime)
		fmt.Println()
		return
	}

	env := os.Getenv("FILEWAY_SECRET_HASHES")
	if env == "" {
		log.Fatal("FATAL: missing environment variable FILEWAY_SECRET_HASHES")
	}
	for _, s := range strings.Split(env, ",") {
		secretHashes = append(secretHashes, []byte(s))
	}

	fmt.Println("Parameters:")
	fmt.Printf("- Chunk size (Kb): %d\n", chunkSize)
	fmt.Printf("- Internal chunk queue size: %d Kb\n", bufferQueueSize)
	fmt.Printf("- Random IDs length: %d\n", idsLength)
	fmt.Println()

	// Setup periodic cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			cleanupStaleConduits()
		}
	}()

	// Routes
	http.HandleFunc("/dl/", dl)   // Shows a download page, if downloader "looks like" CLI redirects to ddl
	http.HandleFunc("/ddl/", ddl) // Direct download
	http.HandleFunc("/setup", setup)
	http.HandleFunc("/ping/", ping)
	http.HandleFunc("/ul/", ul)
	http.HandleFunc("/fileway_ul.py", serveCLIUploader)
	http.HandleFunc("/favicon.png", serveFile(favicon, "image/png"))
	http.HandleFunc("/", serveFile(uploadPage, "text/html"))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func serveFile(file []byte, mime string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", mime)
		w.Write(file)
	}
}

func cleanupStaleConduits() {
	conduitsMu.Lock()
	defer conduitsMu.Unlock()

	cutoffTime := time.Now().UnixMilli() - expiryMillis
	i := 0
	for id, conduit := range conduits {
		if conduit.WasAccessedBefore(cutoffTime) {
			i++
			delete(conduits, id)
		}
	}
	if i > 0 {
		fmt.Printf("%d sessions were garbage collected\n", i)
	}
}

func getConduit(r *string) *Conduit {
	parts := strings.Split(*r, "/")
	conduitId := parts[len(parts)-1]

	conduitsMu.RLock()
	defer conduitsMu.RUnlock()

	conduit := conduits[conduitId]
	return conduit
}

func authenticate(pwd string) bool {
	if _, ok := passwords.Load(pwd); ok {
		return true
	}
	for _, hash := range secretHashes {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(pwd)); err == nil {
			passwords.Store(pwd, true)
			return true
		}
	}
	return false
}

// This is the basic handler for downloads; it shows a download page
// unless the user agent "appears" to come from a CLI application.
// In this case, forwards control to ddl(w, r) that directly downloads
// the payload.
func dl(w http.ResponseWriter, r *http.Request) {
	switch strings.Split(r.UserAgent(), "/")[0] {
	case "curl", "Wget", "HTTPie", "aria2", "Axel":
		ddl(w, r)
	default:
		conduit := getConduit(&r.URL.Path)
		if conduit == nil {
			http.Error(w, "Conduit Not Found", http.StatusNotFound)
			return
		}

		var _downloadPage []byte
		if conduit.IsText {
			_downloadPage = downloadPageForTxt
		} else {
			fileString := fmt.Sprintf("%s (%s)", conduit.Filename, humanReadableSize(conduit.Size))
			_downloadPage = replace(downloadPage, "#FILE_INFO#", fileString)
		}

		serveFile(_downloadPage, "text/html")(w, r)
	}
}

// direct download of the payload
func ddl(w http.ResponseWriter, r *http.Request) {
	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	if err := conduit.Download(); err != nil {
		http.Error(w, err.Error(), http.StatusGone)
		return
	}

	var contentType string
	if conduit.IsText {
		contentType = "text/plain"
	} else {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", conduit.Filename))
	w.Header().Set("Content-Length", strconv.FormatInt(conduit.Size, 10))

	transferred := int64(0)
	for {
		chunk, ok := <-conduit.ChunkQueue
		if !ok || len(chunk) == 0 {
			break
		}

		_, err := w.Write(chunk)
		if err != nil {
			log.Printf("Error writing chunk: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			break
		}

		transferred += int64(len(chunk))
		if transferred >= conduit.Size {
			break
		}
	}

	conduitsMu.Lock()
	delete(conduits, conduit.Id)
	conduitsMu.Unlock()
}

func setup(w http.ResponseWriter, r *http.Request) {
	qry := r.URL.Query()

	passedSecret := r.Header.Get("x-fileway-secret")

	if !authenticate(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	var filename string
	sizeStr := qry.Get("size")
	isText := qry.Get("txt") == "1"
	if isText {
		filename = fmt.Sprintf("fileway_%s.txt", nowString())
	} else {
		filename = qry.Get("filename")
	}
	if sizeStr == "" || filename == "" {
		http.Error(w, "Missing required parameter", http.StatusBadRequest)
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		http.Error(w, "Non-numeric size", http.StatusBadRequest)
		return
	}

	conduit := NewConduit(isText, filename, size, passedSecret)

	conduitsMu.Lock()
	conduits[conduit.Id] = conduit
	conduitsMu.Unlock()

	_, _ = w.Write([]byte(conduit.Id))
}

func ping(w http.ResponseWriter, r *http.Request) {
	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	passedSecret := r.Header.Get("x-fileway-secret")
	if conduit.IsUploadSecretWrong(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	var ret []byte
	if conduit.latch.Wait(20 * time.Second) {
		if _ret, err := json.Marshal(conduit.ChunkPlan); err != nil {
			http.Error(w, "Marshaling issue", http.StatusInternalServerError)
			return
		} else {
			ret = _ret
		}
	} else { // timed out
		ret = []byte("[]")
	}

	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write(ret)
}

func ul(w http.ResponseWriter, r *http.Request) {
	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	passedSecret := r.Header.Get("x-fileway-secret")
	if conduit.IsUploadSecretWrong(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := conduit.Offer(content); err != nil {
		http.Error(w, err.Error(), http.StatusRequestTimeout)
		return
	}

	nextSize := min(len(content)*chunkSizeRampFactor, chunkSize)
	_, _ = w.Write([]byte(strconv.Itoa(nextSize)))
}

func serveCLIUploader(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	base_url := fmt.Sprintf("%s://%s", scheme, r.Host)
	ret := replace(cliUploader, "#BASE_URL#", base_url)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\"fileway_ul.py\"")
	w.Write(ret)
}
