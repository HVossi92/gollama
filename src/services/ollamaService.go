package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type OllamaService struct {
	url string
}

// OllamaRequest struct to structure the request body
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type OllamaChatResponse struct { // New struct for /api/chat response
	Model              string              `json:"model"`
	CreatedAt          string              `json:"created_at"`
	Message            ChatMessageResponse `json:"message"` // <--- Changed to ChatMessageResponse struct
	DoneReason         string              `json:"done_reason"`
	Done               bool                `json:"done"`
	TotalDuration      int64               `json:"total_duration"`
	LoadDuration       int64               `json:"load_duration"`
	PromptEvalCount    int                 `json:"prompt_eval_count"`
	PromptEvalDuration int64               `json:"prompt_eval_duration"`
	EvalCount          int                 `json:"eval_count"`
	EvalDuration       int64               `json:"eval_duration"`
}

type ChatMessageResponse struct { // Struct for the nested "message" object
	Role    string `json:"role"`
	Content string `json:"content"` // <--- The text response is here
}

type ImageMetadata struct {
	CanvasCoordinates []Point `json:"canvas_coordinates"`
}

// Point struct for x, y coordinates
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// GenerateOptions struct to hold metadata within options
type GenerateOptions struct {
	Metadata ImageMetadata `json:"metadata,omitempty"` // Use omitempty if metadata is optional
}

// SetUpVectorDBService creates and initializes a new VectorDBService.
func SetUpOllamaService() *OllamaService {
	return &OllamaService{url: "http://192.168.178.105:11434/api/chat"}
}

var questionSystemPrompt = `
You are a helpful assistant with access to a knowlege base, tasked with answering questions about general knowledge, but also specific to the provided knowledge base.

Answer the question in a very concise manner. Use an unbiased and journalistic tone. Do not repeat text. Don't make anything up. If you are not sure about something, just say that you don't know.
{{- /* Stop here if no context is provided. The rest below is for handling contexts. */ -}}
{{- if . -}}
If possible, answer the question solely based on the provided search results from the knowledge base. If the search results from the knowledge base are not relevant to the question at hand, try to answer the question based on general knowledge. But do not make anything up.

Anything between the following 'context' XML blocks is retrieved from the knowledge base, not part of the conversation with the user. The bullet points are ordered by relevance, so the first one is the most relevant.

<context>
    {{- if . -}}
    {{- range $context := .}}
    - {{.}}{{end}}
    {{- end}}
</context>
{{- end -}}

Don't mention the knowledge base, context or search results in your answer.
`

func (s *OllamaService) AskLLM(question string) (string, error) {
	messages := []OllamaMessage{
		{
			Role:    "system",
			Content: questionSystemPrompt,
		}, {
			Role:    "user",
			Content: "Question: " + question,
		},
	}
	request := OllamaRequest{
		Model:    "qwen_20b_solid:latest",
		Messages: messages,
		Stream:   false,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	response, err := http.Post(s.url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	defer response.Body.Close()

	// --- Capture and Print Raw Response Body ---
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}
	response.Body.Close() // Important: Close response.Body after reading

	ollamaResponse := &OllamaChatResponse{} // <--- Use OllamaChatResponse struct
	err = json.Unmarshal(body, &ollamaResponse)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return "", err
	}

	return ollamaResponse.Message.Content, nil // Return ollamaResponse.Message.Content
}

var imageSystemPrompt = `SYSTEM PROMPT: You are an expert at analyzing images and pictures. The user may send additional regions of interest in the form of coordinates, denoting user drawn boxes.
These boxes are denoted as x and y coordinates as well as w (width) and h (height). If these coordinates are present, ONLY analyze the image in the specified region.
If no boxes are present, analyze the entire image.

When answering, also mention the annotation data, if present.

Annotation data will have the format of the following example:
Annotation Data: [{"x":14,"y":59.21875,"w":261,"h":85}]

USER QUESTION: `

func (s *OllamaService) SendImageToOllama(question string, imagePath string, annotationData string) (string, error) {
	modelName := "llama3.2-vision:latest" // Replace with your Ollama model name (or a model that handles images)

	// 1. Load and Base64 Encode Image
	fmt.Println(imagePath)
	base64Image, err := loadImageBase64(imagePath)
	if err != nil {
		log.Fatalf("Error loading and encoding image: %v", err)
		return "", err
	}

	messages := []OllamaMessage{
		{
			Role:    "system",
			Content: imageSystemPrompt,
		}, {
			Role:    "user",
			Content: question + " Annotation Data: " + string(json.RawMessage(annotationData)),
			Images:  []string{base64Image},
		},
	}
	request := OllamaRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	fmt.Println("Sending image to: ", s.url)
	response, err := http.Post(s.url, "application/json", bytes.NewBuffer(payload))
	fmt.Println(response)
	fmt.Println(err)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	defer response.Body.Close()

	// --- Capture and Print Raw Response Body ---
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}
	response.Body.Close() // Important: Close response.Body after reading

	ollamaResponse := &OllamaChatResponse{} // <--- Use OllamaChatResponse struct
	err = json.Unmarshal(body, &ollamaResponse)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return "", err
	}

	return ollamaResponse.Message.Content, nil
}

// loadImageBase64 loads an image from a file and encodes it to base64
func loadImageBase64(imagePath string) (string, error) {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("error opening image file: %w", err)
	}
	defer imgFile.Close()

	imgBytes, err := io.ReadAll(imgFile)
	if err != nil {
		return "", fmt.Errorf("error reading image file: %w", err)
	}

	base64String := base64.StdEncoding.EncodeToString(imgBytes)
	return base64String, nil
}
