package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
)

func main() {
	http.HandleFunc("/", handleChat)         // Handle GET requests for the index page at root "/"
	http.HandleFunc("/chat", handlePostChat) // Handle POST requests to "/chat" for chat messages

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Println("Server listening on port 2048")
	err := http.ListenAndServe(":2048", nil)
	if err != nil {
		fmt.Println("Server failed to start:", err)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" { // Only handle requests exactly to "/" for index.html
		http.NotFound(w, r)
		return
	}
	fp := filepath.Join("src", "templates", "index.html")
	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing template: %v", err), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, nil)
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

	// Simulate AI response (replace with your actual logic)
	aiResponse := fmt.Sprintf("AI Response: You said: %s", message)

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
