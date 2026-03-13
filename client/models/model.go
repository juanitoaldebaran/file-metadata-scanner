package models

import "time"

type FileMetadata struct {
	FilePath         string     `json:"file_path"`
	FileSize         int64      `json:"file_size"`
	LastModifiedTime *time.Time `json:"last_modified_time,omitempty"`
}
