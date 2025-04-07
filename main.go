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

var (
	jobQueue = make(chan copyJob, 100)
)

type copyJob struct {
	Req      CompleteRequest
	SrcDir   string
	DstDir   string
	RespChan chan<- string
}

func main() {
	go processCopyJobs()

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

	srcDir := filepath.Join(sourceBase, req.Metainfo)
	dstDir := filepath.Join(destinationBase, req.Metainfo)

	respChan := make(chan string, 1)

	job := copyJob{
		Req:      req,
		SrcDir:   srcDir,
		DstDir:   dstDir,
		RespChan: respChan,
	}

	select {
	case jobQueue <- job:
		log.Printf("Queued copy job for %s", req.Metainfo)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("Job queued"))
	default:
		http.Error(w, "Server busy, try again later", http.StatusServiceUnavailable)
		log.Printf("Queue full, dropping job for %s", req.Metainfo)
	}
}

func processCopyJobs() {
	for job := range jobQueue {
		log.Printf("Starting copy job for %s", job.Req.Metainfo)

		// Skip if destination already exists
		if _, err := os.Stat(job.DstDir); err == nil {
			log.Printf("Destination already exists for hash %s, skipping", job.Req.Metainfo)
			job.RespChan <- "Already exists, skipping"
			continue
		} else if !os.IsNotExist(err) {
			log.Printf("Error checking destination: %v", err)
			job.RespChan <- fmt.Sprintf("Error: %v", err)
			continue
		}

		if err := copyDir(job.SrcDir, job.DstDir); err != nil {
			log.Printf("Copy failed for %s: %v", job.Req.Metainfo, err)
			job.RespChan <- fmt.Sprintf("Copy failed: %v", err)
			continue
		}

		log.Printf("Copy succeeded for hash %s", job.Req.Metainfo)
		job.RespChan <- "Copied successfully"
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[walk error] %v", err)
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			log.Printf("Creating directory: %s", targetPath)
			return os.MkdirAll(targetPath, info.Mode())
		}

		log.Printf("Copying file: %s -> %s", path, targetPath)

		srcFile, err := os.Open(path)
		if err != nil {
			log.Printf("Error opening source file %s: %v", path, err)
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			log.Printf("Error creating destination file %s: %v", targetPath, err)
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			log.Printf("Copy failed for %s: %v", path, err)
			return err
		}

		log.Printf("Copied: %s", targetPath)
		return nil
	})
}
