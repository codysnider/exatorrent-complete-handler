package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	sourceBase      = "/data/torrents"
	destinationBase = "/data/complete"
)

type CompleteRequest struct {
	Metainfo string    `json:"metainfo"`
	Name     string    `json:"name"`
	State    string    `json:"state"`
	Time     time.Time `json:"time"`
}

func main() {
	http.HandleFunc("/complete", completeHandler)
	fmt.Println("Listening on :8080")
	_ = http.ListenAndServe(":8080", nil)
}

func completeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request from %s", r.Method, r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Rejected non-POST request")
		return
	}

	var req CompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Printf("Failed to decode JSON: %v", err)
		return
	}
	if req.Metainfo == "" {
		http.Error(w, "Missing metainfo (infohash)", http.StatusBadRequest)
		log.Println("Missing 'metainfo' field")
		return
	}

	log.Printf("Torrent completed: %s (%s)", req.Name, req.Metainfo)
	log.Printf("Completion time: %s", req.Time.Format(time.RFC3339))
	log.Printf("State: %s", req.State)

	srcDir := filepath.Join(sourceBase, req.Metainfo)
	dstDir := filepath.Join(destinationBase, req.Metainfo)

	log.Printf("Source path: %s", srcDir)
	log.Printf("Destination path: %s", dstDir)

	if err := copyDir(srcDir, dstDir); err != nil {
		http.Error(w, fmt.Sprintf("Copy error: %v", err), http.StatusInternalServerError)
		log.Printf("Copy failed: %v", err)
		return
	}

	log.Printf("Copy succeeded for hash %s", req.Metainfo)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Copied successfully"))
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func(srcFile *os.File) {
			_ = srcFile.Close()
		}(srcFile)

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func(dstFile *os.File) {
			_ = dstFile.Close()
		}(dstFile)

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
