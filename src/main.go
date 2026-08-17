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
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/proofrock/fileway/auth"
	fw "github.com/proofrock/fileway/fileway_logic"
	"github.com/proofrock/fileway/utils"
)

var (
	idsLength       = utils.GetIntEnv("RANDOM_IDS_LENGTH", 33)      // Length of ID random strings, amounts to 192 bit
	chunkSize       = utils.GetIntEnv("CHUNK_SIZE_KB", 4096) * 1024 // 4Mb
	bufferQueueSize = utils.GetIntEnv("BUFFER_QUEUE_SIZE", 4)       // 16Mb total
	port            = utils.GetIntEnv("PORT", 8080)
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

var authenticator *auth.Auth
var conduits *fw.ConduitSet

func main() {
	// Replaces version in the web pages and cli uploader
	downloadPage = utils.Replace(downloadPage, "#VERSION#", version)
	downloadPageForTxt = utils.Replace(downloadPageForTxt, "#VERSION#", version)
	uploadPage = utils.Replace(uploadPage, "#VERSION#", version)
	cliUploader = utils.Replace(cliUploader, "#VERSION#", version)

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

	secretHashes := os.Getenv("FILEWAY_SECRET_HASHES")
	if secretHashes == "" {
		log.Fatal("FATAL: missing environment variable FILEWAY_SECRET_HASHES")
	}
	if idsLength <= 0 {
		log.Fatal("FATAL: RANDOM_IDS_LENGTH must be > 0")
	}
	if chunkSize <= 0 {
		log.Fatal("FATAL: CHUNK_SIZE_KB must be > 0")
	}
	if bufferQueueSize <= 0 {
		log.Fatal("FATAL: BUFFER_QUEUE_SIZE must be > 0")
	}
	if port <= 0 || port > 65535 {
		log.Fatal("FATAL: PORT must be between 1 and 65535")
	}

	authenticator = auth.NewAuth(secretHashes)

	uploadTimeout := utils.GetIntEnv("UPLOAD_TIMEOUT_SECS", 240)

	conduits = fw.NewConduitSet(uploadTimeout)

	fmt.Println("Parameters:")
	fmt.Printf("- Port: %d\n", port)
	fmt.Printf("- Chunk size: %d Kb\n", chunkSize/1024)
	fmt.Printf("- Internal chunk queue size: %d\n", bufferQueueSize)
	fmt.Printf("- Random IDs length: %d chars\n", idsLength)
	fmt.Printf("- Upload timeout: %d secs\n", uploadTimeout)
	fmt.Println()

	// Routes
	http.HandleFunc("/dl/", dl)   // Shows a download page, if downloader "looks like" CLI redirects to ddl
	http.HandleFunc("/ddl/", ddl) // Direct download
	http.HandleFunc("/setup", setup)
	http.HandleFunc("/ping/", ping)
	http.HandleFunc("/ul/", ul)
	http.HandleFunc("/fileway_ul.py", serveCLIUploader)
	http.HandleFunc("/favicon.png", serveFile(favicon, "image/png"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveFile(uploadPage, "text/html")(w, r)
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func serveFile(file []byte, contentType string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write(file)
	}
}

func getConduit(r *string) *fw.Conduit {
	parts := strings.Split(*r, "/")
	conduitId := parts[len(parts)-1]

	return conduits.GetConduit(conduitId)
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
			fileString := fmt.Sprintf("%s (%s)", html.EscapeString(conduit.Filename), utils.HumanReadableSize(conduit.Size))
			_downloadPage = utils.Replace(downloadPage, "#FILE_INFO#", fileString)
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
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": conduit.Filename}))
	w.Header().Set("Content-Length", strconv.FormatInt(conduit.Size, 10))

	transferred := int64(0)
	ctx := r.Context()
loop:
	for transferred < conduit.Size {
		select {
		case <-ctx.Done():
			log.Printf("Downloader disconnected for conduit %s", conduit.Id)
			break loop
		case chunk, ok := <-conduit.ChunkQueue:
			if !ok || len(chunk) == 0 {
				break loop
			}
			if _, err := w.Write(chunk); err != nil {
				log.Printf("Error writing chunk: %v", err)
				break loop
			}
			conduit.Touch() // a slow but progressing transfer must not expire
			transferred += int64(len(chunk))
		case <-conduit.Done:
			// The conduit expired. Whatever the uploader already handed over is
			// still owed to the downloader, so drain the buffer before giving up:
			// select picks a ready case at random, so without this the buffered
			// chunks would be dropped and the body would silently fall short of
			// the Content-Length we announced.
			for transferred < conduit.Size {
				var chunk []byte
				select {
				case chunk = <-conduit.ChunkQueue:
				default:
					log.Printf("Conduit %s expired during download", conduit.Id)
					break loop
				}
				if len(chunk) == 0 {
					break loop
				}
				if _, err := w.Write(chunk); err != nil {
					log.Printf("Error writing chunk: %v", err)
					break loop
				}
				transferred += int64(len(chunk))
			}
		}
	}

	conduits.DelConduit(conduit.Id)
}

const maxSizeBytes = 4 * 1024 * 1024 * 1024 * 1024 // 4 TiB

func setup(w http.ResponseWriter, r *http.Request) {
	qry := r.URL.Query()

	passedSecret := r.Header.Get("x-fileway-secret")

	if !authenticator.Authenticate(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	var filename string
	sizeStr := qry.Get("size")
	isText := qry.Get("txt") == "1"
	if isText {
		filename = fmt.Sprintf("fileway_%s.txt", utils.NowString())
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

	if size <= 0 || size > maxSizeBytes {
		http.Error(w, "Invalid size: must be between 1 byte and 4 TiB", http.StatusBadRequest)
		return
	}

	bqs := bufferQueueSize
	if isText {
		bqs = 1
	}

	conduitId := conduits.NewConduit(isText, filename, size, passedSecret, chunkSize, bqs, idsLength)

	_, _ = w.Write([]byte(conduitId))
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
	if conduit.Latch.Wait(20 * time.Second) {
		if conduit.IsExpired() {
			http.Error(w, "Transfer expired", http.StatusGone)
			return
		}
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

	expectedSize := conduit.ClaimNextChunk()
	if expectedSize < 0 {
		http.Error(w, "No chunk expected", http.StatusBadRequest)
		return
	}
	// Read one byte past the plan so an oversized body is detected rather than
	// silently truncated.
	content, err := io.ReadAll(io.LimitReader(r.Body, int64(expectedSize)+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(content) > expectedSize {
		http.Error(w, "Chunk exceeds declared size", http.StatusBadRequest)
		return
	}

	if err := conduit.Offer(content); err != nil {
		// An expired conduit is reported as 410 everywhere, matching ping, so
		// clients can tell "this transfer is over" from "this chunk stalled".
		if errors.Is(err, fw.ErrConduitExpired) {
			http.Error(w, err.Error(), http.StatusGone)
			return
		}
		http.Error(w, err.Error(), http.StatusRequestTimeout)
		return
	}
}

func serveCLIUploader(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	base_url := fmt.Sprintf("%s://%s", scheme, r.Host)
	ret := utils.Replace(cliUploader, "#BASE_URL#", base_url)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\"fileway_ul.py\"")
	w.Write(ret)
}
