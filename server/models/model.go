package models

import "time"

// This is for the metadata
type FileMetadata struct {
	FilePath         string     `json:"file_path"`
	FileSize         int64      `json:"file_size"`
	LastModifiedTime *time.Time `json:"last_modified_time,omitempty"`
}

// This is for storing the data in the database
type StoredFile struct {
	ID               int        `json:"id"`
	FilePath         string     `json:"file_path"`
	FileSize         int64      `json:"file_size"`
	LastModifiedTime *time.Time `json:"last_modified_time,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}
