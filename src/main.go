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
	"time"

	"github.com/proofrock/fileway/auth"
	fw "github.com/proofrock/fileway/fileway_logic"
	"github.com/proofrock/fileway/utils"
)

var (
	idsLength       = utils.GetIntEnv("RANDOM_IDS_LENGTH", 33)      // Length of ID random strings, amounts to 192 bit
	chunkSize       = utils.GetIntEnv("CHUNK_SIZE_KB", 4096) * 1024 // 4Mb
	bufferQueueSize = utils.GetIntEnv("BUFFER_QUEUE_SIZE", 4)       // 16Mb total
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
var conduits = fw.NewConduitSet()

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

	env := os.Getenv("FILEWAY_SECRET_HASHES")
	if env == "" {
		log.Fatal("FATAL: missing environment variable FILEWAY_SECRET_HASHES")
	}

	authenticator = auth.NewAuth(env)

	fmt.Println("Parameters:")
	fmt.Printf("- Chunk size (Kb): %d\n", chunkSize)
	fmt.Printf("- Internal chunk queue size: %d Kb\n", bufferQueueSize)
	fmt.Printf("- Random IDs length: %d\n", idsLength)
	fmt.Println()

	// Routes
	http.HandleFunc("/dl/", dl)   // Shows a download page, if downloader "looks like" CLI redirects to ddl
	http.HandleFunc("/ddl/", ddl) // Direct download
	http.HandleFunc("/setup", setup)
	http.HandleFunc("/cleanup/", cleanup)
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
			fileString := fmt.Sprintf("%s (%s)", conduit.Filename, utils.HumanReadableSize(conduit.Size))
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

	conduits.DelConduit(conduit.Id)
}

func setup(w http.ResponseWriter, r *http.Request) {
	qry := r.URL.Query()

	passedSecret := r.Header.Get("x-fileway-secret")

	if !authenticator.Authenticate(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	var filename string
	isText := qry.Get("txt") == "1"
	if isText {
		filename = fmt.Sprintf("fileway_%s.txt", utils.NowString())
	} else {
		filename = qry.Get("filename")
	}
	if filename == "" {
		http.Error(w, "Missing required parameter 'filename' (and 'txt' != 1)", http.StatusBadRequest)
		return
	}

	sizeStr := qry.Get("size")
	if sizeStr == "" {
		http.Error(w, "Missing required parameter 'size'", http.StatusBadRequest)
		return
	}
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		http.Error(w, "Non-numeric size", http.StatusBadRequest)
		return
	}

	forcedConduitId := qry.Get("forced_id")

	bqs := bufferQueueSize
	if isText {
		bqs = 1
	}

	conduitId, err := conduits.NewConduit(forcedConduitId, isText, filename, size, passedSecret, chunkSize, bqs, idsLength)
	if err != nil {
		// XXX for now, this conflict is the only possible error, but
		//     in the future the HTTP Response Code may be different
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	_, _ = w.Write([]byte(conduitId))
}

func cleanup(w http.ResponseWriter, r *http.Request) {
	passedSecret := r.Header.Get("x-fileway-secret")

	if !authenticator.Authenticate(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	conduits.DelConduit(conduit.Id)
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
