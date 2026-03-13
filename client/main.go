package main

import (
	"bytes"
	"client/models"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultServerUrl = "http://localhost:8080"
	maxRetries       = 3
	batchSize        = 50
)

// This function will be returning true when the file is compiled binary executable
// We need to check whether any execute permission bit is set -> owner/group/other
// Read the first 4 bytes and compare against known binary magic numbers
func isBinaryExecutable(path string, info os.FileInfo) bool {

	//Must have at least one execute bit set
	if info.Mode()&0o111 == 0 {
		return false
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 4)
	n, err := f.Read(buf)
	if err != nil || n < 2 {
		return false
	}

	return isBinaryMagic(buf[:n])
}

// This function will checks the first bytes against known executable magic numbers
// Supported formats:
// -ELF (Linux/Unix) 0x7f 'E' 'L' 'F'
// -PE (Windows) 'M' 'Z'
// -Mach-0 32 bit (macOS): 0xfeedface / 0xcefaedfe
// -Mach-0 64 bit (macOS): 0xfeedfacf / 0xcffaedfe
// -Mach-0 fat binary (macOS): 0xcafebabe
func isBinaryMagic(buf []byte) bool {
	//ELF
	if len(buf) >= 4 && buf[0] == 0x7f && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'F' {
		return true
	}

	//PE (MZ Header)
	if len(buf) >= 2 && buf[0] == 'M' && buf[1] == 'Z' {
		return true
	}

	if len(buf) >= 4 {
		magic := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])

		switch magic {
		case 0xfeedface, 0xfeedfacf, 0xcefaedfe, 0xcffaedfe, 0xcafebabe:
			return true
		}
	}

	return false
}

// This function will walsk rootDir recursively and collects metadata for every single file
// Errors on individual files are logged and skipped rather than aborting the scan
func scanDirectory(rootDir string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Warning: Skipping %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path // This will fall back to absolute path
		}

		meta := models.FileMetadata{
			FilePath: relPath,
			FileSize: info.Size(),
		}

		if isBinaryExecutable(path, info) {
			t := info.ModTime()
			meta.LastModifiedTime = &t
		}

		files = append(files, meta)
		return nil
	})

	return files, err
}

// SendBatch to posts a batch of file metadata to the server with exponential backoff
func sendBatch(client *http.Client, serverURL string, files []models.FileMetadata) error {
	data, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Post(
			serverURL+"/files",
			"application/json",
			bytes.NewReader(data),
		)
		if err != nil {
			lastErr = err
			delay := time.Duration(attempt) * time.Second
			log.Printf("attempt %d/%d failed (%v), retrying in %s", attempt, maxRetries, err, delay)
			time.Sleep(delay)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("server returned HTTP %d", resp.StatusCode)

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
		delay := time.Duration(attempt) * time.Second
		log.Printf("attempt %d/%d failed (%v), retrying in %s", attempt, maxRetries, lastErr, delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("all %d attempts failed and last error: %w", maxRetries, lastErr)
}

func main() {
	serverURL := flag.String("server", defaultServerUrl, "file metadata server url")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: binary scan [--server URL] <directory>")
		os.Exit(1)
	}

	rootDir := args[0]
	info, err := os.Stat(rootDir)
	if err != nil {
		log.Fatalf("cannot access %q: %v", rootDir, err)
	}

	if !info.IsDir() {
		log.Fatalf("%q is not a directory", rootDir)
	}

	log.Printf("scanning %s", rootDir)
	files, err := scanDirectory(rootDir)
	if err != nil {
		log.Fatalf("scan error: %v", err)
	}
	log.Printf("Found %d file", len(files))
	if len(files) == 0 {
		log.Println("nothing to upload")
		return
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	uploaded, failed := 0, 0
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]

		if err := sendBatch(httpClient, *serverURL, batch); err != nil {
			log.Printf("batch [%d - %d] failed: %v", i, end-i, err)
			failed += len(batch)
			continue
		}
		uploaded += len(batch)
		log.Printf("progress: %d/%d uploaded", uploaded, len(files))
	}

	log.Printf("done uploaded: %d, failed: %d", uploaded, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
