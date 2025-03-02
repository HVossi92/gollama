package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/hvossi92/gollama/src/services"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Server struct to hold all services and templates
type Server struct {
	templates     *template.Template
	staticSubFS   fs.FS
	uploadService *services.UploadService
	vectorDB      *services.VectorService
	ollamaService *services.OllamaService
}

// NewServer initializes and returns a new Server instance with all services set up.
func NewServer() (*Server, error) {
	// Parse templates
	templates, err := template.ParseFS(templatesFS,
		"templates/*.html",
		"templates/**/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Create sub filesystem for static assets
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	ollamaService := services.SetUpOllamaService()
	uploadService := services.SetUploadService(templates, ollamaService)
	vectorDB, err := services.SetUpVectorService("gollama.db", false, ollamaService)
	if err != nil {
		return nil, fmt.Errorf("failed to set up VectorDB service: %w", err)
	}

	return &Server{
		templates:     templates,
		staticSubFS:   staticSubFS,
		uploadService: uploadService,
		vectorDB:      vectorDB,
		ollamaService: ollamaService,
	}, nil
}

func main() {
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}
	defer server.vectorDB.Close() // Important to close VectorDB service when done

	err = os.RemoveAll("./uploads")
	if err != nil {
		log.Fatalf("Failed to delete uploads directory: %v", err)
	}

	http.HandleFunc("/", server.handleChat)
	http.HandleFunc("POST /chat", server.handlePostChat)
	http.HandleFunc("POST /upload/image", server.uploadService.UploadAndSaveImage)
	http.HandleFunc("GET /vector", server.GetVectors)
	http.HandleFunc("POST /vector", server.UploadVector)
	http.HandleFunc("GET /annotation-ui", server.uploadService.AnnotationUIHandler)
	http.HandleFunc("POST /submit-annotations", server.uploadService.SubmitAnnotationsHandler)
	http.HandleFunc("GET /cancel-annotation", server.uploadService.CancelAnnotationHandler)
	http.HandleFunc("DELETE /upload", server.uploadService.PruneUploads)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(server.staticSubFS))))

	fmt.Println("Server listening on port 2048")
	err = http.ListenAndServe(":2048", nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	err := s.templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) handlePostChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	doUseRag := r.URL.Query().Get("use-rag") == "true"

	var err error
	fmt.Println("Asking LLM")
	aiResponse, err := s.ollamaService.AskLLM(message, doUseRag, s.vectorDB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		panic(err)
	}
	fmt.Printf("AI Response: %s", aiResponse)

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: message,
		AIResponse:  aiResponse,
	}
	err = s.templates.ExecuteTemplate(w, "message.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) GetVectors(w http.ResponseWriter, r *http.Request) {
	text, err := s.vectorDB.ReadAllVectors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: "Get all vectors",
		AIResponse:  text,
	}
	err = s.templates.ExecuteTemplate(w, "message.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) UploadVector(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("vectors")
	if text == "" {
		http.Error(w, "No data provided", http.StatusBadRequest)
	}
	err := s.vectorDB.SaveVectorToDb(text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: "Save vector to DB",
		AIResponse:  text,
	}
	err = s.templates.ExecuteTemplate(w, "message.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
