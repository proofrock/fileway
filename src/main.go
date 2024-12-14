package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	conduits     = make(map[string]*Conduit)
	secretHashes = make(map[string]bool)
	conduitsMu   sync.RWMutex
)

func main() {
	// https://manytools.org/hacker-tools/ascii-banner/, profile "Slant"
	fmt.Println("    _____ __                          __      _ __ ")
	fmt.Println("   / __(_) /__  _________  ____  ____/ /_  __(_) /_")
	fmt.Println("  / /_/ / / _ \\/ ___/ __ \\/ __ \\/ __  / / / / / __/")
	fmt.Println(" / __/ / /  __/ /__/ /_/ / / / / /_/ / /_/ / / /_  ")
	fmt.Println("/_/ /_/_/\\___/\\___/\\____/_/ /_/\\__,_/\\__,_/_/\\__/ v0.0.0")
	fmt.Println()

	env := os.Getenv("FILECONDUIT_SECRET_HASHES")
	if env == "" {
		log.Fatal("FATAL: missing environment variable FILECONDUIT_SECRET_HASHES")
	}
	for _, s := range strings.Split(env, ",") {
		secretHashes[strings.ToLower(s)] = true
	}

	// Setup periodic cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			cleanupStaleConduits()
		}
	}()

	// Routes
	http.HandleFunc("/dl/", dl)
	http.HandleFunc("/setup", setup)
	http.HandleFunc("/ping/", ping)
	http.HandleFunc("/ul/", ul)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func cleanupStaleConduits() {
	conduitsMu.Lock()
	defer conduitsMu.Unlock()

	cutoffTime := time.Now().Add(-15 * time.Minute).UnixMilli()
	for id, conduit := range conduits {
		if conduit.WasAccessedBefore(cutoffTime) {
			delete(conduits, id)
		}
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

func dl(w http.ResponseWriter, r *http.Request) {
	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	if err := conduit.Download(); err != nil {
		http.Error(w, err.Error(), http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
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
	passedSecret := r.Header.Get("x-fileconduit-secret")
	if !secretHashes[sha256Hex(passedSecret)] {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	sizeStr := r.URL.Query().Get("size")
	filename := r.URL.Query().Get("filename")
	if sizeStr == "" || filename == "" {
		http.Error(w, "Missing required parameter", http.StatusBadRequest)
		return
	}

	size, _ := strconv.ParseInt(sizeStr, 10, 64)
	conduit := NewConduit(filename, size, passedSecret)

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

	passedSecret := r.Header.Get("x-fileconduit-secret")
	if conduit.IsUploadSecretWrong(passedSecret) {
		http.Error(w, "Secret Mismatch", http.StatusUnauthorized)
		return
	}

	ret := ""
	if conduit.IsDownloading() {
		ret = strconv.Itoa(ChunkSize)
	}
	_, _ = w.Write([]byte(ret))
}

func ul(w http.ResponseWriter, r *http.Request) {
	conduit := getConduit(&r.URL.Path)
	if conduit == nil {
		http.Error(w, "Conduit Not Found", http.StatusNotFound)
		return
	}

	passedSecret := r.Header.Get("x-fileconduit-secret")
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
