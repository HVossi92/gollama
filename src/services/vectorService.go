package services

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// VectorDBService represents the service responsible for the vector database.
type VectorDBService struct {
	db            *sql.DB
	uploadService *UploadService
}

// SetUpVectorDBService creates and initializes a new VectorDBService.
func SetUpVectorDBService(dbPath string, overwrite bool) (*VectorDBService, error) {
	if overwrite {
		log.Println("Overwriting existing database (if it exists)")
		if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to remove existing database: %w", err)
		}
	}

	sqlite_vec.Auto() // Assuming Auto() is safe to call on every setup

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	var vecVersion string
	err = db.QueryRow("select vec_version()").Scan(&vecVersion)
	if err != nil {
		db.Close() // Close the connection if setup fails
		return nil, fmt.Errorf("failed to get vec_version: %w", err)
	}
	log.Printf("vec_version=%s\n", vecVersion)

	// Consider moving testDb logic out of setup if it's not strictly part of initialization.
	if err := testDb(db); err != nil { // Pass logger for better context
		db.Close()
		return nil, fmt.Errorf("database test failed: %w", err)
	}

	return &VectorDBService{db: db}, nil
}

// Close closes the database connection.  Good practice to add a Close method.
func (s *VectorDBService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetDB returns the underlying sql.DB connection (for use within the service package).
func (s *VectorDBService) GetDB() *sql.DB {
	return s.db
}

func testDb(db *sql.DB) error {
	_, err := db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS vec_items USING vec0(embedding float[4])")
	if err != nil {
		return fmt.Errorf("failed to create virtual table: %w", err)
	}

	items := map[int][]float32{
		1: {0.1, 0.1, 0.1, 0.1},
		2: {0.2, 0.2, 0.2, 0.2},
		3: {0.3, 0.3, 0.3, 0.3},
		4: {0.4, 0.4, 0.4, 0.4},
		5: {0.5, 0.5, 0.5, 0.5},
	}
	q := []float32{0.3, 0.3, 0.3, 0.3}

	for id, values := range items {
		v, err := sqlite_vec.SerializeFloat32(values)
		if err != nil {
			return fmt.Errorf("failed to serialize float32: %w", err)
		}
		_, err = db.Exec("INSERT INTO vec_items(rowid, embedding) VALUES (?, ?)", id, v)
		if err != nil {
			return fmt.Errorf("failed to insert item: %w", err)
		}
	}

	query, err := sqlite_vec.SerializeFloat32(q)
	if err != nil {
		return fmt.Errorf("failed to serialize query: %w", err)
	}

	rows, err := db.Query(`
		SELECT
			rowid,
			distance
		FROM vec_items
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT 3
	`, query)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close() // Important to close rows after use

	for rows.Next() {
		var rowid int64
		var distance float64
		err = rows.Scan(&rowid, &distance)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		log.Printf("rowid=%d, distance=%f\n", rowid, distance) // Use logger
	}
	if err := rows.Err(); err != nil { // Check for errors after iteration
		return fmt.Errorf("rows iteration error: %w", err)
	}
	return nil
}

func (s *VectorDBService) UploadDocumentToVectorDB(w http.ResponseWriter, r *http.Request) {
	file, header, err := s.uploadService.handleFileUpload(r)
	if err != nil {
		w.Write([]byte("Failed to uploaded document to vector DB"))
	}
	ext := filepath.Ext(header.Filename)

	if ext == ".jsonl" {
		fmt.Println("JSONL")
	} else if ext == ".txt" {
		fmt.Println("TXT")
	} else if ext == ".pdf" {
		fmt.Println("PDF")
	} else if ext == ".csv" {
		fmt.Println("CSV")
	} else if ext == ".json" {
		fmt.Println("JSON")
	} else {
		w.Write([]byte("Unsupported file type"))
	}
	// print text from File
	text, err := io.ReadAll(file)
	if err != nil {
		w.Write([]byte("Failed to read document from file"))
	}
	fmt.Println(string(text))

	w.Write([]byte("Document uploaded to vector DB"))
}
