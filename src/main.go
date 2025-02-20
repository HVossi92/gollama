package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hvossi92/gollama/src/services"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Store parsed templates globally
var templates *template.Template

// Define a struct to hold image data
type ImageData struct {
	ImageURL string
}

func main() {
	// Parse all templates at startup
	var err error
	templates, err = template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Create sub filesystem for static assets
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal("Failed to create sub filesystem:", err)
	}

	http.HandleFunc("/", handleChat)
	http.HandleFunc("POST /chat", handlePostChat)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/annotation-ui", annotationUIHandler)
	http.HandleFunc("/submit-annotations", submitAnnotationsHandler)
	http.HandleFunc("/cancel-annotation", cancelAnnotationHandler)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads")))) // Serve uploaded images

	// Serve embedded static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS))))

	// services.SetUpVectorDb(true)

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

var imageURL string
var filename string

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form, limit memory usage for file uploads
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image") // "image" is the name attribute in your HTML input
	if err != nil {
		http.Error(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Ensure "uploads" directory exists
	err = os.MkdirAll("./uploads", os.ModePerm)
	if err != nil {
		http.Error(w, "Error creating uploads directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a unique filename (you might want to use UUIDs or timestamps for better uniqueness)
	filename = filepath.Join("./uploads", header.Filename) // Or generate a unique name
	outFile, err := os.Create(filename)
	if err != nil {
		http.Error(w, "Error creating file on server: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, "Error saving file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with HTMX to update the image area
	imageURL = "/uploads/" + header.Filename // URL to access the uploaded image

	data := ImageData{ImageURL: imageURL}
	err = templates.ExecuteTemplate(w, "image-display.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Handler to serve the annotation UI fragment (buttons, canvas, etc.)
func annotationUIHandler(w http.ResponseWriter, r *http.Request) {
	data := ImageData{ImageURL: imageURL}
	err := templates.ExecuteTemplate(w, "annotation-ui.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func submitAnnotationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	annotationData := r.Form.Get("annotations") // Get the JSON string from hx-vals

	message := "I am giving you annotation data for the provided image, denoting a rectangular area of the image. x, y, w, h and are pixel, so the box starts at x pixels from the left and y pixels from the top. It is w pixels wide and h pixels high. Explain what you see in the box, considering the marked areas."
	aiResponse := services.SendImageToOllama(message, filename, annotationData)

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: message,
		AIResponse:  aiResponse,
	}
	err = templates.ExecuteTemplate(w, "message.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func cancelAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	// Simply clear the annotation area
	w.Write([]byte("<p>Annotation cancelled.</p>"))
}
