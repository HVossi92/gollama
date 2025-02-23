package services

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// VectorService represents the service responsible for the vector database.
type VectorService struct {
	db            *sql.DB
	ollamaService *OllamaService
}

type VectorItem struct {
	Content   string
	Embedding []byte
}

// SetUpVectorService creates and initializes a new VectorDBService.
func SetUpVectorService(dbPath string, overwrite bool, ollamaService *OllamaService) (*VectorService, error) {
	if overwrite {
		log.Println("Overwriting existing database (if it exists)")
		if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to remove existing database: %w", err)
		}
	}

	// Create VectorService instance first
	vectorService := &VectorService{db: nil, ollamaService: ollamaService}
	db, err := vectorService.createDb(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	vectorService.db = db

	// Then call ensureVectorTableExists on the instance
	if err := vectorService.createVectorTable(); err != nil {
		db.Close() // Close the connection if table creation fails
		return nil, fmt.Errorf("failed to ensure vector table exists: %w", err)
	}
	return vectorService, nil
}

// Close closes the database connection.  Good practice to add a Close method.
func (s *VectorService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetDB returns the underlying sql.DB connection (for use within the service package).
func (s *VectorService) GetDB() *sql.DB {
	return s.db
}

func (s *VectorService) createDb(dbPath string) (*sql.DB, error) {
	sqlite_vec.Auto() // Assuming Auto() is safe to call on every setup

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	var vecVersion string
	err = db.QueryRow("select vec_version()").Scan(&vecVersion)
	if err != nil {
		db.Close() // Close the connection if setup fails
		return nil, err
	}
	log.Printf("vec_version=%s\n", vecVersion)
	return db, nil
}

// EnsureVectorTableExists checks if the vector table exists and creates it if not.
func (s *VectorService) createVectorTable() error {
	createTableSQL := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_items USING vec0(
			embedding float[%d],
			content TEXT
		)
	`, 768)
	_, err := s.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create virtual table: %w", err)
	}
	return nil
}

// StoreChunkAndEmbedding saves a text chunk and its embedding to the SQLite vector database.
func (s *VectorService) StoreChunkAndEmbedding(chunk string, embedding []float32) error {
	if s.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	serializedEmbedding, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize float32 embedding: %w", err)
	}

	_, err = s.db.Exec("INSERT INTO vec_items(embedding, content) VALUES (?, ?)", serializedEmbedding, chunk)
	if err != nil {
		return fmt.Errorf("failed to insert item into vector DB: %w", err)
	}
	return nil
}

func (s *VectorService) GetVectors(w http.ResponseWriter, r *http.Request) {
	s.ReadAllVectors()
	w.Write([]byte("Read vectors, see server logs"))
}

func (s *VectorService) UploadVectors(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("vectors")
	if text == "" {
		http.Error(w, "No data provided", http.StatusBadRequest)
		return
	}

	chunkedText, err := chunkText(strings.TrimSpace(text), 16, 4) // Chunk text first
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, chunk := range chunkedText { // Iterate through each text chunk
		fmt.Printf("\nProcessing chunk %d: \"%s\"\n", i+1, chunk) // Indicate chunk being processed

		embeddings, err := s.ollamaService.GetVectorEmbedding(chunk) // Get embedding for each chunk
		fmt.Printf("Embedding Dimension for chunk %d: %d\n", i+1, len(embeddings))

		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting embedding for chunk %d: %v", i+1, err), http.StatusInternalServerError)
			return // Stop processing if embedding fails for any chunk
		}

		err = s.StoreChunkAndEmbedding(chunk, embeddings) // Store chunk and embedding in DB
		if err != nil {
			http.Error(w, fmt.Sprintf("Error storing vector for chunk %d: %v", i+1, err), http.StatusInternalServerError)
			return
		}
		fmt.Printf("Chunk %d embeddings processed and stored in DB.\n", i+1)
	}

	fmt.Println("\nAll Embeddings Generated and Stored in Vector DB")
	fmt.Println("Total Chunks:", len(chunkedText))

	s.ReadAllVectors()

	w.Write([]byte("Data uploaded to vector DB (embeddings generated and stored)"))
}

// chunkText chunks a string of text into smaller overlapping text chunks based on sentences.
//
// Parameters:
//
//	text:       The input string of text to chunk.
//	chunkSize:  The desired number of sentences per chunk.
//	chunkOverlap: The number of sentences to overlap between consecutive chunks.
//
// Returns:
//
//	[]string:  A slice of strings, where each string is a chunk of text (composed of sentences).
//	error:     An error if the input parameters are invalid.
func chunkText(text string, chunkSize int, chunkOverlap int) ([]string, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be greater than 0")
	}
	if chunkOverlap < 0 {
		return nil, fmt.Errorf("chunkOverlap must be non-negative")
	}
	if chunkOverlap >= chunkSize {
		return nil, fmt.Errorf("chunkOverlap must be less than chunkSize")
	}

	sentences := splitIntoSentences(text) // Split text into sentences
	if len(sentences) == 0 {
		return []string{}, nil // Return empty slice for empty input text
	}

	var chunks []string
	step := chunkSize - chunkOverlap

	for i := 0; i < len(sentences); i += step {
		end := i + chunkSize
		if end > len(sentences) {
			end = len(sentences) // Adjust end for the last chunk
		}
		chunkSentences := sentences[i:end]
		chunk := strings.Join(chunkSentences, " ") // Join sentences in the chunk back into a string
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// splitIntoSentences splits a text into sentences using a simple regex.
// Note: This is a basic sentence splitter and might not be perfect for all cases.
// For more robust sentence splitting, consider using NLP libraries.
func splitIntoSentences(text string) []string {
	// Use a regex to split by sentence-ending punctuation (. ! ?) followed by whitespace
	sentenceRegex := regexp.MustCompile(`(?P<sentence>[^.!?]+[.!?])\s+`) // Improved regex

	var sentences []string
	matches := sentenceRegex.FindAllStringSubmatch(text, -1)
	if matches == nil {
		// Handle case where no sentences are found using the regex (e.g., very short text)
		sentences = strings.Split(text, ". ") // Fallback to simple split if regex fails
		if len(sentences) <= 1 {              // If still only one or zero sentences after simple split
			sentences = strings.Split(text, "\n") // Try splitting by newline as a last resort
			if len(sentences) <= 1 {
				sentences = []string{text} // If all else fails, treat the whole text as one sentence
			}
		}
		return sentences

	}
	for _, match := range matches {
		sentences = append(sentences, strings.TrimSpace(match[1])) // match[1] is the captured sentence group
	}

	// Handle the last part of the text that might not end with sentence-ending punctuation
	lastSentence := sentenceRegex.ReplaceAllString(text, "")
	lastSentence = strings.TrimSpace(lastSentence)
	if lastSentence != "" {
		sentences = append(sentences, lastSentence)
	}

	return sentences
}

// ReadAllVectors reads all data from the vector DB table.
func (s *VectorService) ReadAllVectors() {
	if s.db == nil {
		log.Fatal("database connection is nil in VectorService")
	}
	rows, err := s.db.Query("SELECT * FROM vec_items LIMIT 2")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
	}

	// Make a slice of interface{} to hold each column value
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err = rows.Scan(valuePtrs...)
		if err != nil {
			log.Fatal(err)
		}

		// Print each column name and value
		for i, col := range columns {
			fmt.Printf("%s: %v\n", col, values[i])
		}
		fmt.Println("---") // Separator between rows
	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
}

// FindSimilarVectors queries the vector DB for vectors similar to the given embedding.
func (s *VectorService) FindSimilarVectors(queryEmbedding []float32) ([]VectorItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection is nil in VectorService")
	}

	serializedQueryEmbedding, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT
			content,
			embedding,
			distance
		FROM vec_items
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT 3 -- Limit to top 3 most similar vectors for context
	`, serializedQueryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("vector DB query failed: %w", err)
	}
	defer rows.Close()

	var similarItems []VectorItem
	for rows.Next() {
		var content string
		var serializedEmbedding []byte // Embedding is already []byte from DB
		var distance float64           // Retrieve distance as well

		if err := rows.Scan(&content, &serializedEmbedding, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan vector DB row: %w", err)
		}

		// Deserialization is REMOVED here
		// embedding, err := sqlite_vec.DeserializeFloat32(serializedEmbedding) // Removed DeserializeFloat32
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to deserialize embedding from DB: %w", err)
		// }

		similarItems = append(similarItems, VectorItem{
			Content:   content,
			Embedding: serializedEmbedding, // Store serialized embedding directly in VectorItem
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error during vector DB query: %w", err)
	}

	return similarItems, nil
}
