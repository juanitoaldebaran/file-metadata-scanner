package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"server/db"
	"server/models"
	"strconv"
)

// handlePost accepts either a single file metadata object or JSON array
func handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed tp read body", http.StatusBadRequest)
		return
	}

	var files []models.FileMetadata

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(body, &files); err != nil {
			http.Error(w, "invalid JSON array", http.StatusBadRequest)
			return
		}
	} else {
		var single models.FileMetadata
		if err := json.Unmarshal(body, &single); err != nil {
			http.Error(w, "invalid JSON object", http.StatusBadRequest)
			return
		}
		files = []models.FileMetadata{single}
	}

	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("db begin: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO files (file_path, file_size, last_modified_time) 
		VALUES (?, ?, ?)`)
	if err != nil {
		log.Printf("db prepare: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	for _, f := range files {
		if _, err := stmt.Exec(f.FilePath, f.FileSize, f.LastModifiedTime); err != nil {
			log.Printf("db insert: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("db commit: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"uploaded": len(files)})
	log.Printf("stored %d file(s)", len(files))
}

// handlePost return most recently uploaded files, up to ?limit=20
func handleGet(w http.ResponseWriter, r *http.Request) {
	limit := 20

	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}

	rows, err := db.DB.Query(`
		SELECT id, file_path, file_size, last_modified_time, created_at
		FROM files
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)

	if err != nil {
		log.Printf("db query: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	files := make([]models.StoredFile, 0)
	for rows.Next() {
		var f models.StoredFile
		var lm sql.NullTime
		if err := rows.Scan(&f.ID, &f.FilePath, &f.FileSize, &lm, &f.CreatedAt); err != nil {
			log.Printf("db scan: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if lm.Valid {
			f.LastModifiedTime = &lm.Time
		}
		files = append(files, f)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handlePost(w, r)
	case http.MethodGet:
		handleGet(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("db", "files.db", "SQLite database path")
	flag.Parse()

	if err := db.InitDB(*dsn); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	log.Printf("database ready at %s", *dsn)

	mux := http.NewServeMux()
	mux.HandleFunc("/files", filesHandler)

	log.Printf("server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
