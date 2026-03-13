# File Metadata Scanner

## Juanito Aldebaran Siahaan - Programming Language : Go 

## Project Description
 Two components system that scans a directory files, collects metadata and upload it to a persistent server.

## Project Structure
```
file-metadata-scanner/client/models/model.go
file-metadata-scanner/client/main.go
file-metadata-scanner/client/go.mod

file-metadata-scanner/server/db/database.go
file-metadata-scanner/server/models/model.go
file-metadata-scanner/server/main.go
file-metadata-scanner/server/go.mod
file-metadata-scanner/server/go.sum
```

## Running the server

```bash
cd server
go mod download
go run main.go
```

By default the server listens on :8080 port and creates files.db

## Optional flag
| Flag | Default | Description |
| --- | --- | --- |
|`--addr` | `:8080` | Addresses on port listen on |
| `--db` | `files.db` | Path to sqlite database |

## Example with custom options

```bash
go run main.go --addr :9090 --db /tmp/myfiles.db
```

The database is created automatically on first run and persist across restart

## Running the Client
 
Open a second terminal:
 
```bash
cd client
go mod download
go run main.go [--server URL] <directory>
```

## Argument
| Flag | Required | Desription |
| --- | --- | --- |
|`<directory>` | `yes` | Path to directory scan |
| `--server` | `no` | server base url http://localhost:8080 |

## Examples 
```bash
# Scan the current directory
go run main.go .
 
# Scan a specific path
go run main.go /usr/bin
 
# Point at a custom server
go run main.go --server http://localhost:9090 /usr/bin
```

## API Endpoints
 
### `POST /files`
 
Upload file metadata. Accepts either a single object or a JSON array.
 
## Request body
 
```json
[
  {
    "file_path": "bin/myapp",
    "file_size": 204800,
    "last_modified_time": "2024-01-15T10:30:00Z"
  },
  {
    "file_path": "README.md",
    "file_size": 1024
  }
]
```

> `last_modified_time` is only included for binary executable files (ELF, PE, Mach-O formats). It is omitted for plain text, images, and other non-executable files.
 
## Response `201 Created`
 
```json
{ "uploaded": 2 }
```
 
---
 
### `GET /files?limit=20`
 
Returns the most recently uploaded files, ordered by upload time descending.
 
## Query parameters
 
| Parameter | Default | Description |
|-----------|---------|-------------|
| `limit` | `20` | Maximum number of records to return |
 
## Example
 
```bash
curl "http://localhost:8080/files?limit=5"
```
 
**Response `200 OK`:**
 
```json
[
  {
    "id": 42,
    "file_path": "bin/myapp",
    "file_size": 204800,
    "last_modified_time": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-15T12:00:00Z"
  }
]
```
 
