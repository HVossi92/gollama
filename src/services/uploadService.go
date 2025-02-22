package services

import (
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type FileUrl struct {
	FileURL string
}

// VectorDBService represents the service responsible for the vector database.
type UploadService struct {
	templates *template.Template
	fileURL   string
	filename  string
}

func SetUploadService(templates *template.Template) (*UploadService, error) {
	return &UploadService{templates: templates}, nil
}

func (s *UploadService) UploadAndSaveImage(w http.ResponseWriter, r *http.Request) {
	file, header, err := s.handleFileUpload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.saveImage(w, r, file, header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *UploadService) handleFileUpload(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	// Parse multipart form, limit memory usage for file uploads
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		return nil, nil, err
	}

	file, header, err := r.FormFile("file") // "image" is the name attribute in your HTML input
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	// Ensure "uploads" directory exists
	err = os.MkdirAll("./uploads", os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	return file, header, nil
}

func (s *UploadService) saveImage(w http.ResponseWriter, r *http.Request, file multipart.File, header *multipart.FileHeader) error {
	// Create a unique filename (you might want to use UUIDs or timestamps for better uniqueness)
	s.filename = filepath.Join("./uploads", header.Filename) // Or generate a unique name
	outFile, err := os.Create(s.filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		return err
	}

	// Respond with HTMX to update the image area
	s.fileURL = "/uploads/" + header.Filename // URL to access the uploaded image

	data := FileUrl{FileURL: s.fileURL}
	err = s.templates.ExecuteTemplate(w, "image-display.html", data) // Use pre-parsed template
	if err != nil {
		return err
	}
	return nil
}

// Handler to serve the annotation UI fragment (buttons, canvas, etc.)
func (s *UploadService) AnnotationUIHandler(w http.ResponseWriter, r *http.Request) {
	data := FileUrl{FileURL: s.fileURL}
	err := s.templates.ExecuteTemplate(w, "annotation-ui.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *UploadService) SubmitAnnotationsHandler(w http.ResponseWriter, r *http.Request) {
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
	aiResponse := SendImageToOllama(message, s.filename, annotationData)

	data := struct {
		UserMessage string
		AIResponse  string
	}{
		UserMessage: message,
		AIResponse:  aiResponse,
	}
	err = s.templates.ExecuteTemplate(w, "message.html", data) // Use pre-parsed template
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *UploadService) CancelAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	// Simply clear the annotation area
	w.Write([]byte("<p>Annotation cancelled.</p>"))
}

func (s *UploadService) PruneUploads(w http.ResponseWriter, r *http.Request) {
	// Delete all files in the uploads directory
	err := os.RemoveAll("./uploads")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// write 'All uploads pruned' to the response
	w.Write([]byte("All uploads pruned"))
}
