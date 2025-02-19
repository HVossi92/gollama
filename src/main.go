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

	// Serve embedded static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS))))

	db := services.SetupDb()
	defer db.Close()
	services.CreateTestVectors()

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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	message := r.FormValue("message")
	// Simulate AI response
	aiResponse := fmt.Sprintf("AI Response: You said: %s", message)

	// You can also pre-parse this template in main() if it's static
	tmpl, err := template.New("message").Parse(`
    <div class="message user-message">
        {{.UserMessage}}
    </div>
    <div class="message ai-message">
        {{.AIResponse}}
    </div>
    `)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing message template: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: message,
		AIResponse:  aiResponse,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing message template: %v", err), http.StatusInternalServerError)
		return
	}
}
