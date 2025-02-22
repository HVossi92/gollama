package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/hvossi92/gollama/src/services"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Store parsed templates globally
var templates *template.Template

// Define a struct to hold image data

func main() {
	// Parse all templates at startup
	var err error
	templates, err = template.ParseFS(templatesFS,
		"templates/*.html",    // Match HTML files in main directory
		"templates/**/*.html", // Match HTML files in all subdirectories
	)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Create sub filesystem for static assets
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal("Failed to create sub filesystem:", err)
	}

	uploadService, err := services.SetUploadService(templates)
	if err != nil {
		log.Fatal(err)
	}
	vectorService, err := services.SetUpVectorDBService("gollama.db", true)
	if err != nil {
		log.Fatalf("Failed to set up VectorDB service: %v", err)
	}
	defer vectorService.Close() // Important to close the service when done

	http.HandleFunc("/", handleChat)
	http.HandleFunc("POST /chat", handlePostChat)
	http.HandleFunc("POST /upload/image", uploadService.UploadAndSaveImage)
	http.HandleFunc("POST /vector", vectorService.UploadDocumentToVectorDB)
	http.HandleFunc("GET /annotation-ui", uploadService.AnnotationUIHandler)
	http.HandleFunc("POST /submit-annotations", uploadService.SubmitAnnotationsHandler)
	http.HandleFunc("GET /cancel-annotation", uploadService.CancelAnnotationHandler)
	http.HandleFunc("DELETE /upload", uploadService.PruneUploads)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads")))) // Serve uploaded images

	// Serve embedded static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS))))

	fmt.Println("Server listening on port 2048")
	err = http.ListenAndServe(":2048", nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Execute the pre-parsed template
	err := templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
	}
}

func handlePostChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")

	fmt.Println("Asking LLM")
	aiResponse := services.AskLlm(message)
	// Simulate AI response
	fmt.Printf("AI Response: You said: %s", message)

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: message,
		AIResponse:  aiResponse,
	}
	err := templates.ExecuteTemplate(w, "message.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
