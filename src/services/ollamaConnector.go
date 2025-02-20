package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// ImageMetadata struct to hold canvas coordinates
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

// OllamaRequest struct to structure the request body
type OllamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Images  []string               `json:"images,omitempty"`  // Omitempty if no image
	Options map[string]interface{} `json:"options,omitempty"` // Changed from string to map
}

var systemPrompt = `SYSTEM PROMPT: You are an expert at analyzing images and pictures. The user may send additional regions of interest in the form of coordinates, denoting user drawn boxes.
These boxes are denoted as x and y coordinates as well as w (width) and h (height). If these coordinates are present, ONLY analyze the image in the specified region.
If no boxes are present, analyze the entire image.

When answering, also mention the annotation data, if present.

Annotation data will have the format of the following example:
Annotation Data: [{"x":14,"y":59.21875,"w":261,"h":85}]

USER QUESTION: `

func SendImageToOllama(question string, imagePath string, annotationData string) string {
	ollamaEndpoint := "http://192.168.178.105:11434/api/generate" // Replace with your Ollama endpoint
	modelName := "llama3.2-vision"                                // Replace with your Ollama model name (or a model that handles images)

	// 1. Load and Base64 Encode Image
	base64Image, err := loadImageBase64(imagePath)
	if err != nil {
		log.Fatalf("Error loading and encoding image: %v", err)
		return err.Error()
	}

	fmt.Println("Annotation Data: " + annotationData)

	// Then use this in your request
	requestPayload := OllamaRequest{
		Model:  modelName,
		Prompt: question + " Annotation Data: " + string(json.RawMessage(annotationData)),
		Images: []string{base64Image},
	}

	// 5. Marshal Request Payload to JSON
	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		log.Fatalf("Error marshaling JSON payload: %v", err)
		return err.Error()
	}

	// 6. Send HTTP POST Request
	resp, err := http.Post(ollamaEndpoint, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatalf("Error sending HTTP request: %v", err)
		return err.Error()
	}
	defer resp.Body.Close()

	// 7. Handle Response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body) // Read body for error details
		log.Fatalf("HTTP request failed with status code: %d, body: %s", resp.StatusCode, string(body))
		return resp.Status
	}

	// Stream the response (Ollama often streams responses)
	decoder := json.NewDecoder(resp.Body)
	type OllamaResponse struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	var fullResponse strings.Builder
	for {
		var chunk OllamaResponse
		if err := decoder.Decode(&chunk); err != nil {
			// handle error
		}
		fullResponse.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	return fullResponse.String()
}

// loadImageBase64 loads an image from a file and encodes it to base64
func loadImageBase64(imagePath string) (string, error) {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("error opening image file: %w", err)
	}
	defer imgFile.Close()

	imgBytes, err := ioutil.ReadAll(imgFile)
	if err != nil {
		return "", fmt.Errorf("error reading image file: %w", err)
	}

	base64String := base64.StdEncoding.EncodeToString(imgBytes)
	return base64String, nil
}
